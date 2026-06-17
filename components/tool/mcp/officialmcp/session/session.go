/*
 * Copyright 2026 CloudWeGo Authors
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

package session

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sync"

	officialmcp "github.com/cloudwego/eino-ext/components/tool/mcp/officialmcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	TransportSSE            = "sse"
	TransportStdio          = "stdio"
	TransportStreamableHTTP = "streamable_http"
)

// ErrorKindUnsupportedTransport tags an unsupported TransportConfig.Type. It
// lives here rather than in the officialmcp package because only the session
// layer constructs transports.
const ErrorKindUnsupportedTransport officialmcp.ErrorKind = "unsupported_transport"

type ServerConfig struct {
	Name              string
	Transport         TransportConfig
	Client            *mcp.Implementation
	ClientOptions     *mcp.ClientOptions
	InitializeOptions *mcp.ClientSessionOptions
}

type TransportConfig struct {
	Type    string
	URL     string
	Command string
	Args    []string
	Env     map[string]string
	Headers map[string]string
	CWD     string
}

// Session is an officialmcp.ClientSession backed by a go-sdk session that
// it rebuilds when a call fails with a connection-level error.
//
// A go-sdk session cannot be revived: once its connection fails the failure is
// terminal and every subsequent call on it errors. The only recovery is to
// discard it and connect again. Session owns the ServerConfig so it can
// do exactly that — transparently to the officialmcp tools, which only see the
// ClientSession interface.
//
// Reconnection is triggered only by officialmcp.IsConnectionError (the go-sdk
// terminal sentinels). Protocol-level rejections (unknown tool, invalid params)
// and application-level tool errors (result.IsError) leave the session healthy
// and are returned to the caller unchanged — they never trigger a reconnect, so
// a model repeatedly calling a tool with bad arguments cannot cause reconnect
// churn.
type Session struct {
	Name string

	cfg ServerConfig

	mu      sync.Mutex
	session *mcp.ClientSession
}

var _ officialmcp.ClientSession = (*Session)(nil)

type StartupError struct {
	ServerName    string
	TransportType string
	Err           error
}

func (e *StartupError) Error() string {
	return fmt.Sprintf("failed to start official mcp session, server: %s, transport: %s: %v", e.ServerName, e.TransportType, e.Err)
}

func (e *StartupError) Unwrap() error {
	return e.Err
}

// Connect establishes a session for cfg and returns a Session that will
// transparently reconnect (with the same cfg) on connection-level failures.
func Connect(ctx context.Context, cfg ServerConfig) (*Session, error) {
	session, err := connect(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Session{Name: cfg.Name, cfg: cfg, session: session}, nil
}

// connect builds the transport and dials a single go-sdk session.
func connect(ctx context.Context, cfg ServerConfig) (*mcp.ClientSession, error) {
	transport, err := newTransport(cfg.Transport)
	if err != nil {
		return nil, startupError(cfg, err)
	}

	impl := cfg.Client
	if impl == nil {
		impl = &mcp.Implementation{Name: "eino-officialmcp", Version: "v0.0.0"}
	}
	client := mcp.NewClient(impl, cfg.ClientOptions)
	session, err := client.Connect(ctx, transport, cfg.InitializeOptions)
	if err != nil {
		return nil, startupError(cfg, err)
	}
	return session, nil
}

// Close closes the current underlying session. It is safe to call concurrently
// with in-flight calls, but the session must not be used afterwards.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	cur := s.session
	s.mu.Unlock()
	if cur == nil {
		return nil
	}
	return cur.Close()
}

func (s *Session) current() *mcp.ClientSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session
}

// reconnect rebuilds the session, but only if stale is still the current one.
// If another goroutine already reconnected (current != stale), it returns the
// fresh session without connecting again, so a burst of concurrent connection
// errors yields a single reconnect.
func (s *Session) reconnect(ctx context.Context, stale *mcp.ClientSession) (*mcp.ClientSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != stale {
		return s.session, nil
	}
	if stale != nil {
		_ = stale.Close()
	}
	cur, err := connect(ctx, s.cfg)
	if err != nil {
		return nil, err
	}
	s.session = cur
	return s.session, nil
}

// ListTools calls the underlying session, reconnecting and retrying once on a
// connection-level failure.
func (s *Session) ListTools(ctx context.Context, params *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	cur := s.current()
	res, err := cur.ListTools(ctx, params)
	if err == nil || !officialmcp.IsConnectionError(err) {
		return res, err
	}
	cur, rerr := s.reconnect(ctx, cur)
	if rerr != nil {
		return nil, rerr
	}
	return cur.ListTools(ctx, params)
}

// CallTool calls the underlying session, reconnecting and retrying once on a
// connection-level failure.
func (s *Session) CallTool(ctx context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	cur := s.current()
	res, err := cur.CallTool(ctx, params)
	if err == nil || !officialmcp.IsConnectionError(err) {
		return res, err
	}
	cur, rerr := s.reconnect(ctx, cur)
	if rerr != nil {
		return nil, rerr
	}
	return cur.CallTool(ctx, params)
}

// Ping pings the underlying session, reconnecting and retrying once on a
// connection-level failure.
func (s *Session) Ping(ctx context.Context, params *mcp.PingParams) error {
	cur := s.current()
	err := cur.Ping(ctx, params)
	if err == nil || !officialmcp.IsConnectionError(err) {
		return err
	}
	cur, rerr := s.reconnect(ctx, cur)
	if rerr != nil {
		return rerr
	}
	return cur.Ping(ctx, params)
}

func newTransport(cfg TransportConfig) (mcp.Transport, error) {
	switch cfg.Type {
	case TransportSSE:
		if err := validateAbsoluteURL(cfg.URL); err != nil {
			return nil, err
		}
		return &mcp.SSEClientTransport{Endpoint: cfg.URL, HTTPClient: httpClientWithHeaders(cfg.Headers)}, nil
	case TransportStreamableHTTP:
		if err := validateAbsoluteURL(cfg.URL); err != nil {
			return nil, err
		}
		return &mcp.StreamableClientTransport{Endpoint: cfg.URL, HTTPClient: httpClientWithHeaders(cfg.Headers)}, nil
	case TransportStdio:
		if cfg.Command == "" {
			return nil, fmt.Errorf("stdio command is empty")
		}
		cmd := exec.Command(cfg.Command, cfg.Args...)
		if cfg.CWD != "" {
			cmd.Dir = cfg.CWD
		}
		cmd.Env = append(os.Environ(), flattenEnv(cfg.Env)...)
		return &mcp.CommandTransport{Command: cmd}, nil
	default:
		return nil, &officialmcp.Error{
			Kind: ErrorKindUnsupportedTransport,
			Err:  fmt.Errorf("unsupported official mcp transport: %s", cfg.Type),
		}
	}
}

func validateAbsoluteURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("transport URL is empty")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if !u.IsAbs() || u.Host == "" {
		return fmt.Errorf("transport URL must be absolute: %s", rawURL)
	}
	return nil
}

func startupError(cfg ServerConfig, err error) error {
	return &StartupError{
		ServerName:    cfg.Name,
		TransportType: cfg.Transport.Type,
		Err:           err,
	}
}

func flattenEnv(env map[string]string) []string {
	ret := make([]string, 0, len(env))
	for k, v := range env {
		ret = append(ret, k+"="+v)
	}
	return ret
}

func httpClientWithHeaders(headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return nil
	}
	return &http.Client{Transport: headerRoundTripper{
		base:    http.DefaultTransport,
		headers: headers,
	}}
}

type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	for k, v := range h.headers {
		cloned.Header.Set(k, v)
	}
	return h.base.RoundTrip(cloned)
}
