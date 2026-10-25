/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package url

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/document/parser/html"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
)

var _ document.Loader = (*Loader)(nil)

const (
	// defaultTimeout bounds a single HTTP round trip when LoaderConfig.Client is
	// omitted. A bounded default matters: http.DefaultClient has no timeout, so
	// a stalled peer would pin the caller's goroutine and its connection for as
	// long as the peer stays silent.
	defaultTimeout = 30 * time.Second

	// defaultMaxResponseBytes caps how many response body bytes are handed to
	// the parser. Without a cap a single large or hostile URL can exhaust
	// process memory before the parser ever runs.
	defaultMaxResponseBytes int64 = 10 << 20 // 10 MiB

	// maxErrorSnippetBytes bounds how much of a failing response body is quoted
	// back in the returned error.
	maxErrorSnippetBytes = 512
)

// ErrResponseTooLarge is returned when a response body exceeds
// LoaderConfig.MaxResponseBytes. It is reported instead of silently truncating,
// because a truncated document is indistinguishable from a complete one and
// would quietly poison downstream retrieval.
var ErrResponseTooLarge = errors.New("response body exceeds the configured limit")

// LoaderConfig is the config for url Loader.
type LoaderConfig struct {
	// Parser parses the response body into documents.
	// Optional. Default: parser/html with the <body> selector.
	Parser parser.Parser

	// Client performs the HTTP request.
	// Optional. Default: an http.Client with a 30s timeout that uses
	// http.DefaultTransport, so proxy environment variables, dial timeouts and
	// HTTP/2 keep working.
	Client *http.Client

	// RequestBuilder builds the request for a source URI. It receives the same
	// LoaderOptions that were passed to Load, so implementation specific
	// options can influence the request.
	// Optional. Default: a GET request against source.URI.
	RequestBuilder func(ctx context.Context, source document.Source, opts ...document.LoaderOption) (*http.Request, error)

	// MaxResponseBytes caps how many response body bytes are passed to Parser.
	// Exceeding it fails the load with ErrResponseTooLarge.
	// Optional. Default: 10 MiB. Non-positive values restore the default.
	MaxResponseBytes int64
}

func defaultRequestBuilder(ctx context.Context, source document.Source, _ ...document.LoaderOption) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodGet, source.URI, nil)
}

// NewLoader creates a new loader for url.
func NewLoader(ctx context.Context, conf *LoaderConfig) (*Loader, error) {
	if conf == nil {
		conf = &LoaderConfig{}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if conf.Parser == nil {
		p, err := html.NewParser(ctx, &html.Config{
			Selector: &html.BodySelector,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create default HTML parser: %w", err)
		}

		conf.Parser = p
	}
	if conf.Client == nil {
		// Only Timeout is set on purpose. Leaving Transport nil makes the client
		// fall back to http.DefaultTransport, which honours HTTP(S)_PROXY, dial
		// timeouts and HTTP/2. Assigning http.DefaultClient here would inherit
		// no timeout at all.
		conf.Client = &http.Client{Timeout: defaultTimeout}
	}
	if conf.RequestBuilder == nil {
		conf.RequestBuilder = defaultRequestBuilder
	}
	if conf.MaxResponseBytes <= 0 {
		conf.MaxResponseBytes = defaultMaxResponseBytes
	}

	return &Loader{
		conf: conf,
	}, nil
}

// Loader is a loader for url.
type Loader struct {
	conf *LoaderConfig
}

func (l *Loader) Load(ctx context.Context, src document.Source, opts ...document.LoaderOption) (docs []*schema.Document, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, l.GetType(), components.ComponentOfLoader)
	ctx = callbacks.OnStart(ctx, &document.LoaderCallbackInput{
		Source: src,
	})
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	if l.conf == nil || l.conf.Parser == nil {
		return nil, errors.New("parser is nil")
	}

	body, err := l.load(ctx, src, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load content from uri [%s]: %w", src.URI, err)
	}

	closed := false
	closeBody := func() error {
		if closed {
			return nil
		}
		closed = true

		return body.Close()
	}
	// Release the connection on every early-return path. The explicit close
	// below still reports a close failure as a load error.
	defer func() {
		_ = closeBody()
	}()

	o := document.GetLoaderCommonOptions(&document.LoaderOptions{}, opts...)

	docs, err = l.conf.Parser.Parse(ctx, body, append([]parser.Option{parser.WithURI(src.URI)}, o.ParserOptions...)...)
	if err != nil {
		return nil, fmt.Errorf("parse content of uri [%s] err: %w", src.URI, err)
	}
	// Drain whatever the parser left unread, then ask the reader whether
	// anything went wrong. This makes the size limit apply to the whole response
	// instead of only to the bytes a parser happened to consume, so a parser
	// that never reads (or swallows read errors) cannot hand back a document
	// built from a body that blew the limit. Draining also lets the transport
	// reuse the connection instead of tearing it down.
	_, _ = io.Copy(io.Discard, body)
	if bodyErr := body.Err(); bodyErr != nil {
		return nil, fmt.Errorf("read response body of uri [%s] err: %w", src.URI, bodyErr)
	}

	// Close before reporting success: a close error must surface as an error,
	// not as OnEnd followed by OnError for the same node.
	if closeErr := closeBody(); closeErr != nil {
		return nil, fmt.Errorf("failed to close response body of uri [%s]: %w", src.URI, closeErr)
	}

	_ = callbacks.OnEnd(ctx, &document.LoaderCallbackOutput{
		Source: src,
		Docs:   docs,
	})

	return docs, nil
}

func (l *Loader) load(ctx context.Context, src document.Source, opts ...document.LoaderOption) (*sizeLimitedReadCloser, error) {
	req, err := l.conf.RequestBuilder(ctx, src, opts...)
	if err != nil {
		return nil, err
	}
	if req == nil {
		// A custom builder may return a nil request without an error, which would
		// otherwise panic inside http.Client.Do.
		return nil, errors.New("request builder returned a nil request")
	}

	resp, err := l.conf.Client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// Fail loudly: parsing an error page as document content silently corrupts
		// the downstream corpus. Quote a bounded snippet to keep the error useful.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorSnippetBytes))
		_ = resp.Body.Close()

		status := strings.TrimSpace(resp.Status)
		if status == "" {
			status = fmt.Sprintf("status code %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected HTTP status %s%s", status, snippetSuffix(snippet))
	}

	return newSizeLimitedReadCloser(resp.Body, l.conf.MaxResponseBytes), nil
}

func (l *Loader) GetType() string {
	return "URLLoader"
}

func (l *Loader) IsCallbacksEnabled() bool {
	return true
}

func snippetSuffix(snippet []byte) string {
	text := strings.TrimSpace(string(snippet))
	if text == "" {
		return ""
	}
	return ": " + text
}

// sizeLimitedReadCloser bounds the number of bytes read from an HTTP response
// body and records the first error it hits, so callers can distinguish a body
// that ended cleanly from one that was cut short by the limit.
type sizeLimitedReadCloser struct {
	rc        io.ReadCloser
	remaining int64
	err       error
}

func newSizeLimitedReadCloser(rc io.ReadCloser, limit int64) *sizeLimitedReadCloser {
	if limit <= 0 {
		limit = defaultMaxResponseBytes
	}

	return &sizeLimitedReadCloser{rc: rc, remaining: limit}
}

// Err reports the first read error observed by the reader. It is nil when the
// body was consumed within the configured limit.
func (b *sizeLimitedReadCloser) Err() error {
	return b.err
}

func (b *sizeLimitedReadCloser) Read(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	if len(p) == 0 {
		return 0, nil
	}

	if b.remaining <= 0 {
		// The limit is exhausted. Probe a single byte so a body of exactly the
		// configured size still terminates cleanly with io.EOF.
		var probe [1]byte
		n, err := b.rc.Read(probe[:])
		switch {
		case n > 0:
			b.err = ErrResponseTooLarge
		case errors.Is(err, io.EOF):
			return 0, io.EOF
		case err != nil:
			b.err = err
		default:
			// The body reported neither data nor EOF; treat it as an overflow to
			// avoid spinning on a reader that never makes progress.
			b.err = ErrResponseTooLarge
		}

		return 0, b.err
	}

	if int64(len(p)) > b.remaining {
		p = p[:int(b.remaining)]
	}

	n, err := b.rc.Read(p)
	b.remaining -= int64(n)
	if err != nil && !errors.Is(err, io.EOF) {
		b.err = err
	}

	return n, err
}

func (b *sizeLimitedReadCloser) Close() error {
	return b.rc.Close()
}
