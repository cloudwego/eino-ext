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
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloudwego/eino-ext/components/document/parser/html"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
)

type MockParser struct {
	mock func(io.Reader) ([]*schema.Document, error)
}

func (p *MockParser) Parse(ctx context.Context, reader io.Reader, opts ...parser.Option) (doc []*schema.Document, err error) {
	return p.mock(reader)
}

// swallowedErrorParser returns a document without ever reading the body, which
// is how a sloppy custom parser behaves when it ignores read errors.
type swallowedErrorParser struct{}

func (swallowedErrorParser) Parse(context.Context, io.Reader, ...parser.Option) ([]*schema.Document, error) {
	return []*schema.Document{{Content: "partial"}}, nil
}

func newFileServer(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.FileServer(http.Dir("./testdata")))
	t.Cleanup(srv.Close)

	return srv
}

func newLoader(t *testing.T, ctx context.Context, conf *LoaderConfig) *Loader {
	t.Helper()

	loader, err := NewLoader(ctx, conf)
	require.NoError(t, err)

	return loader
}

func TestLoad(t *testing.T) {
	srv := newFileServer(t)

	ctx := context.Background()
	ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{})

	t.Run("html loader", func(t *testing.T) {
		loader := newLoader(t, ctx, &LoaderConfig{})

		uri := fmt.Sprintf("%s/test.html", srv.URL)
		docs, err := loader.Load(ctx, document.Source{URI: uri})
		require.NoError(t, err)

		require.Len(t, docs, 1)
		assert.Equal(t, uri, docs[0].MetaData[html.MetaKeySource])
		assert.Equal(t, "Test html in url loader", docs[0].MetaData[html.MetaKeyTitle])
	})

	t.Run("md loader", func(t *testing.T) {
		p := &MockParser{
			mock: func(reader io.Reader) ([]*schema.Document, error) {
				data, err := io.ReadAll(reader)
				assert.NoError(t, err)

				return []*schema.Document{
					{
						Content: string(data),
					},
				}, nil
			},
		}

		loader := newLoader(t, ctx, &LoaderConfig{Parser: p})

		docs, err := loader.Load(ctx, document.Source{URI: fmt.Sprintf("%s/test.md", srv.URL)})
		require.NoError(t, err)

		require.Len(t, docs, 1)
		// Normalise line endings so the expectation holds regardless of the
		// checkout's core.autocrlf setting.
		assert.Equal(t, "# Title\nhello world", strings.ReplaceAll(docs[0].Content, "\r\n", "\n"))
	})

	t.Run("custom request builder and custom client", func(t *testing.T) {
		loader := newLoader(t, ctx, &LoaderConfig{
			RequestBuilder: func(ctx context.Context, src document.Source, opts ...document.LoaderOption) (*http.Request, error) {
				return http.NewRequestWithContext(ctx, http.MethodGet, "file:///test.html", nil)
			},
			Client: &http.Client{
				Timeout:   5 * time.Second,
				Transport: http.NewFileTransport(http.Dir("./testdata")),
			},
		})

		docs, err := loader.Load(ctx, document.Source{URI: fmt.Sprintf("%s/test.xx", srv.URL)})
		require.NoError(t, err)

		require.Len(t, docs, 1)
		assert.Equal(t, "Test html in url loader", docs[0].MetaData[html.MetaKeyTitle])
	})
}

func TestNewLoaderDefaults(t *testing.T) {
	ctx := context.Background()

	loader := newLoader(t, ctx, nil)

	assert.NotNil(t, loader.conf.Parser)
	require.NotNil(t, loader.conf.Client)
	// A client without a timeout lets a silent peer pin the goroutine forever.
	assert.Equal(t, defaultTimeout, loader.conf.Client.Timeout)
	// A nil Transport keeps http.DefaultTransport behavior, including proxy
	// environment variables and dial timeouts.
	assert.Nil(t, loader.conf.Client.Transport)
	assert.Equal(t, defaultMaxResponseBytes, loader.conf.MaxResponseBytes)
}

func TestLoadPassesOptionsToRequestBuilder(t *testing.T) {
	var seenParserOptions int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "<html><body>ok</body></html>")
	}))
	t.Cleanup(srv.Close)

	loader := newLoader(t, context.Background(), &LoaderConfig{
		RequestBuilder: func(ctx context.Context, src document.Source, opts ...document.LoaderOption) (*http.Request, error) {
			// Regression: Load used to drop opts before calling the builder, so
			// per-call options never reached it.
			seenParserOptions = len(document.GetLoaderCommonOptions(nil, opts...).ParserOptions)

			return http.NewRequestWithContext(ctx, http.MethodGet, src.URI, nil)
		},
	})

	_, err := loader.Load(context.Background(), document.Source{URI: srv.URL},
		document.WithParserOptions(parser.WithURI("first"), parser.WithURI("second")))
	require.NoError(t, err)
	assert.Equal(t, 2, seenParserOptions)
}

func TestLoadRejectsNon2xxStatus(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "not found", statusCode: http.StatusNotFound, body: "<html><body>missing</body></html>"},
		{name: "server error", statusCode: http.StatusInternalServerError, body: "boom"},
		{name: "empty body", statusCode: http.StatusBadGateway, body: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(srv.Close)

			loader := newLoader(t, context.Background(), &LoaderConfig{})

			docs, err := loader.Load(context.Background(), document.Source{URI: srv.URL})
			require.Error(t, err)
			assert.Nil(t, docs)
			// Regression: an error page used to be parsed and returned as a
			// successfully loaded document.
			assert.Contains(t, err.Error(), fmt.Sprintf("%d", tc.statusCode))
		})
	}
}

func TestLoadRejectsOversizedResponse(t *testing.T) {
	const limit = 32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("a", limit+1))
	}))
	t.Cleanup(srv.Close)

	loader := newLoader(t, context.Background(), &LoaderConfig{
		MaxResponseBytes: limit,
		Parser: &MockParser{mock: func(reader io.Reader) ([]*schema.Document, error) {
			data, _ := io.ReadAll(reader)

			return []*schema.Document{{Content: string(data)}}, nil
		}},
	})

	docs, err := loader.Load(context.Background(), document.Source{URI: srv.URL})
	require.Error(t, err)
	assert.Nil(t, docs)
	assert.ErrorIs(t, err, ErrResponseTooLarge)
}

func TestLoadDetectsOversizeEvenIfParserSwallowsTheError(t *testing.T) {
	const limit = 16

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("b", limit*4))
	}))
	t.Cleanup(srv.Close)

	loader := newLoader(t, context.Background(), &LoaderConfig{
		MaxResponseBytes: limit,
		Parser:           swallowedErrorParser{},
	})

	docs, err := loader.Load(context.Background(), document.Source{URI: srv.URL})
	require.Error(t, err)
	assert.Nil(t, docs)
	assert.ErrorIs(t, err, ErrResponseTooLarge)
}

func TestLoadAcceptsResponseAtExactlyTheLimit(t *testing.T) {
	const limit = 32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("c", limit))
	}))
	t.Cleanup(srv.Close)

	loader := newLoader(t, context.Background(), &LoaderConfig{
		MaxResponseBytes: limit,
		Parser: &MockParser{mock: func(reader io.Reader) ([]*schema.Document, error) {
			data, err := io.ReadAll(reader)
			if err != nil {
				return nil, err
			}

			return []*schema.Document{{Content: string(data)}}, nil
		}},
	})

	docs, err := loader.Load(context.Background(), document.Source{URI: srv.URL})
	require.NoError(t, err)
	require.Len(t, docs, 1)
	// A body that fits the limit byte for byte must not be reported as overflow.
	assert.Equal(t, strings.Repeat("c", limit), docs[0].Content)
}

func TestLoadRejectsNilRequestFromBuilder(t *testing.T) {
	loader := newLoader(t, context.Background(), &LoaderConfig{
		RequestBuilder: func(context.Context, document.Source, ...document.LoaderOption) (*http.Request, error) {
			return nil, nil
		},
	})

	docs, err := loader.Load(context.Background(), document.Source{URI: "http://example.invalid"})
	require.Error(t, err)
	assert.Nil(t, docs)
	assert.Contains(t, err.Error(), "nil request")
}

func TestLoadPropagatesRequestBuilderError(t *testing.T) {
	builderErr := errors.New("builder failed")

	loader := newLoader(t, context.Background(), &LoaderConfig{
		RequestBuilder: func(context.Context, document.Source, ...document.LoaderOption) (*http.Request, error) {
			return nil, builderErr
		},
	})

	_, err := loader.Load(context.Background(), document.Source{URI: "http://example.invalid"})
	require.Error(t, err)
	assert.ErrorIs(t, err, builderErr)
}

func TestSizeLimitedReadCloser(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		rc := newSizeLimitedReadCloser(io.NopCloser(strings.NewReader("hello")), 32)

		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		assert.Equal(t, "hello", string(data))
		assert.NoError(t, rc.Err())
	})

	t.Run("exact limit", func(t *testing.T) {
		rc := newSizeLimitedReadCloser(io.NopCloser(strings.NewReader("hello")), 5)

		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		assert.Equal(t, "hello", string(data))
		assert.NoError(t, rc.Err())
	})

	t.Run("over limit", func(t *testing.T) {
		rc := newSizeLimitedReadCloser(io.NopCloser(strings.NewReader("hello!")), 5)

		data, err := io.ReadAll(rc)
		require.ErrorIs(t, err, ErrResponseTooLarge)
		assert.Equal(t, "hello", string(data))
		assert.ErrorIs(t, rc.Err(), ErrResponseTooLarge)
	})

	t.Run("non-positive limit falls back to default", func(t *testing.T) {
		rc := newSizeLimitedReadCloser(io.NopCloser(strings.NewReader("hello")), 0)
		assert.Equal(t, defaultMaxResponseBytes, rc.remaining)
	})
}
