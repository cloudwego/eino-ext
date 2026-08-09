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
	"errors"
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
	TransportStreamableHTTP = "streamable-http"
)

// ReplayPolicy controls whether an operation is issued again after Session
// replaces a failed connection. Reconnection still happens for ReplayNever;
// only replay of the operation that observed the failure is suppressed.
type ReplayPolicy uint8

const (
	// ReplayDefault preserves the historical behavior and replays once.
	ReplayDefault ReplayPolicy = iota
	ReplayAlways
	ReplayNever
	// ReplaySafe uses method-specific conservative semantics: Ping is safe,
	// ListTools is safe only for an empty cursor, and CallTool is never safe.
	ReplaySafe
)

// ReplayPolicies configures replay independently for each operation exposed by
// Session. Zero values preserve the historical replay-once behavior.
type ReplayPolicies struct {
	ListTools ReplayPolicy
	CallTool  ReplayPolicy
	Ping      ReplayPolicy
}

// ErrorKindUnsupportedTransport tags an unsupported TransportConfig.Type. It
// lives here rather than in the officialmcp package because only the session
// layer constructs transports.
const ErrorKindUnsupportedTransport officialmcp.ErrorKind = "unsupported_transport"

type ServerConfig struct {
	Name              string
	Transport         TransportConfig
	Replay            ReplayPolicies
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

	// HTTPClient overrides the client used for URL-based transports (SSE,
	// streamable-http). When set, it is used as the base client, so a caller can install a
	// RoundTripper that injects per-request auth (e.g. a credential vault whose
	// token is resolved fresh on every request and survives reconnects). Headers,
	// when set, are pre-set on each request before the supplied client's
	// RoundTripper runs, so that RoundTripper sees them and may override them.
	// Ignored for stdio.
	HTTPClient *http.Client

	// Factory builds a custom transport. When set, it takes precedence over the
	// built-in transports selected by Type, and it is invoked again on every
	// reconnect, so callers control redial semantics.
	Factory func(ctx context.Context) (mcp.Transport, error)
}

var ErrSessionClosed = errors.New("official mcp session is closed")

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
// churn. A failed reconnect is itself reported as a connection-level error
// (officialmcp.ErrorKindConnection) so callers can keep telling an unreachable
// server apart from a protocol rejection.
type Session struct {
	Name string

	cfg ServerConfig

	mu      sync.Mutex
	session *mcp.ClientSession
	closed  bool
	closedC chan struct{}
	attempt *reconnectAttempt
}

type reconnectAttempt struct {
	done   chan struct{}
	cancel context.CancelFunc
	result *mcp.ClientSession
	err    error
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
	return &Session{Name: cfg.Name, cfg: cfg, session: session, closedC: make(chan struct{})}, nil
}

// connect builds the transport and dials a single go-sdk session. Reconnect
// re-enters here with the same cfg, so a TransportConfig.Factory is invoked
// again on every reconnect.
func connect(ctx context.Context, cfg ServerConfig) (*mcp.ClientSession, error) {
	if err := validateReplayPolicies(cfg.Replay); err != nil {
		return nil, startupError(cfg, err)
	}
	transport, err := newTransport(ctx, cfg.Transport)
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
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	cur := s.session
	s.session = nil
	attempt := s.attempt
	if s.closedC != nil {
		close(s.closedC)
	}
	s.mu.Unlock()
	if attempt != nil {
		attempt.cancel()
	}
	if cur == nil {
		return nil
	}
	return cur.Close()
}

func (s *Session) current() (*mcp.ClientSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.session == nil {
		return nil, ErrSessionClosed
	}
	return s.session, nil
}

// reconnect rebuilds the session, but only if stale is still the current one.
// Concurrent callers share one in-flight attempt. The potentially blocking
// connect runs without holding s.mu, which lets Close cancel the attempt and
// return without waiting for a transport that ignores cancellation. A late
// successful connection is closed instead of being adopted after Close.
func (s *Session) reconnect(ctx context.Context, stale *mcp.ClientSession) (*mcp.ClientSession, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrSessionClosed
	}
	if s.session != stale {
		current := s.session
		s.mu.Unlock()
		return current, nil
	}
	if attempt := s.attempt; attempt != nil {
		closedC := s.closedC
		s.mu.Unlock()
		select {
		case <-attempt.done:
			return attempt.result, attempt.err
		case <-closedC:
			return nil, ErrSessionClosed
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	attemptCtx, cancel := context.WithCancel(ctx)
	attempt := &reconnectAttempt{
		done:   make(chan struct{}),
		cancel: cancel,
	}
	s.attempt = attempt
	cfg := s.cfg
	s.mu.Unlock()

	fresh, connectErr := connect(attemptCtx, cfg)

	var closeFresh, closeStale *mcp.ClientSession
	s.mu.Lock()
	switch {
	case s.closed:
		attempt.err = ErrSessionClosed
		closeFresh = fresh
	case s.session != stale:
		attempt.result = s.session
		closeFresh = fresh
	case connectErr != nil:
		// Leave stale installed so a later operation can start a new attempt.
		attempt.err = &officialmcp.Error{
			Kind:       officialmcp.ErrorKindConnection,
			ServerName: cfg.Name,
			Err:        connectErr,
		}
	default:
		s.session = fresh
		attempt.result = fresh
		closeStale = stale
	}
	if s.attempt == attempt {
		s.attempt = nil
	}
	close(attempt.done)
	s.mu.Unlock()
	cancel()

	if closeFresh != nil {
		_ = closeFresh.Close()
	}
	if closeStale != nil {
		_ = closeStale.Close()
	}
	return attempt.result, attempt.err
}

// ListTools calls the underlying session and follows the configured replay
// policy after a connection-level failure.
func (s *Session) ListTools(ctx context.Context, params *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	cur, err := s.current()
	if err != nil {
		return nil, err
	}
	res, err := cur.ListTools(ctx, params)
	if err == nil || !officialmcp.IsConnectionError(err) {
		return res, err
	}
	fresh, reconnectErr := s.reconnect(ctx, cur)
	replay := shouldReplay(s.cfg.Replay.ListTools, params == nil || params.Cursor == "")
	if !replay {
		return nil, connectionError(s.cfg.Name, "list tools", err, reconnectErr)
	}
	if reconnectErr != nil {
		return nil, reconnectErr
	}
	if fresh == nil {
		return nil, ErrSessionClosed
	}
	return fresh.ListTools(ctx, params)
}

// CallTool calls the underlying session and follows the configured replay
// policy after a connection-level failure. ReplayNever conservatively returns
// ErrorKindUncertainOutcome because the transport cannot prove whether the
// server applied the request before the connection failed.
func (s *Session) CallTool(ctx context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	cur, err := s.current()
	if err != nil {
		return nil, err
	}
	res, err := cur.CallTool(ctx, params)
	if err == nil || !officialmcp.IsConnectionError(err) {
		return res, err
	}
	fresh, reconnectErr := s.reconnect(ctx, cur)
	if !shouldReplay(s.cfg.Replay.CallTool, false) {
		return nil, uncertainOutcomeError(s.cfg.Name, "call tool", err, reconnectErr)
	}
	if reconnectErr != nil {
		return nil, reconnectErr
	}
	if fresh == nil {
		return nil, ErrSessionClosed
	}
	return fresh.CallTool(ctx, params)
}

// Ping follows the configured replay policy after a connection-level failure.
func (s *Session) Ping(ctx context.Context, params *mcp.PingParams) error {
	cur, err := s.current()
	if err != nil {
		return err
	}
	err = cur.Ping(ctx, params)
	if err == nil || !officialmcp.IsConnectionError(err) {
		return err
	}
	fresh, reconnectErr := s.reconnect(ctx, cur)
	if !shouldReplay(s.cfg.Replay.Ping, true) {
		return uncertainOutcomeError(s.cfg.Name, "ping", err, reconnectErr)
	}
	if reconnectErr != nil {
		return reconnectErr
	}
	if fresh == nil {
		return ErrSessionClosed
	}
	return fresh.Ping(ctx, params)
}

func newTransport(ctx context.Context, cfg TransportConfig) (mcp.Transport, error) {
	if cfg.Factory != nil {
		transport, err := cfg.Factory(ctx)
		if err != nil {
			return nil, err
		}
		if transport == nil {
			return nil, errors.New("official mcp transport factory returned nil")
		}
		return transport, nil
	}
	switch cfg.Type {
	case TransportSSE:
		if err := validateAbsoluteURL(cfg.URL); err != nil {
			return nil, err
		}
		return &mcp.SSEClientTransport{Endpoint: cfg.URL, HTTPClient: httpClientFor(cfg)}, nil
	case TransportStreamableHTTP:
		if err := validateAbsoluteURL(cfg.URL); err != nil {
			return nil, err
		}
		return &mcp.StreamableClientTransport{Endpoint: cfg.URL, HTTPClient: httpClientFor(cfg)}, nil
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

func validateReplayPolicies(policies ReplayPolicies) error {
	for _, candidate := range []struct {
		name   string
		policy ReplayPolicy
	}{
		{name: "list tools", policy: policies.ListTools},
		{name: "call tool", policy: policies.CallTool},
		{name: "ping", policy: policies.Ping},
	} {
		name, policy := candidate.name, candidate.policy
		switch policy {
		case ReplayDefault, ReplayAlways, ReplayNever, ReplaySafe:
		default:
			return fmt.Errorf("invalid official mcp %s replay policy: %d", name, policy)
		}
	}
	return nil
}

func shouldReplay(policy ReplayPolicy, operationSafe bool) bool {
	switch policy {
	case ReplayDefault, ReplayAlways:
		return true
	case ReplayNever:
		return false
	case ReplaySafe:
		return operationSafe
	default:
		return false
	}
}

func uncertainOutcomeError(serverName, operation string, operationErr, reconnectErr error) error {
	cause := operationErr
	if reconnectErr != nil {
		cause = errors.Join(operationErr, reconnectErr)
	}
	return &officialmcp.Error{
		Kind:       officialmcp.ErrorKindUncertainOutcome,
		ServerName: serverName,
		Err:        fmt.Errorf("official mcp %s outcome is uncertain: %w", operation, cause),
	}
}

func connectionError(serverName, operation string, operationErr, reconnectErr error) error {
	cause := operationErr
	if reconnectErr != nil {
		cause = errors.Join(operationErr, reconnectErr)
	}
	return &officialmcp.Error{
		Kind:       officialmcp.ErrorKindConnection,
		ServerName: serverName,
		Err: fmt.Errorf(
			"official mcp %s was not replayed after reconnect: %w",
			operation,
			cause,
		),
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

// httpClientFor returns the HTTP client for a URL-based transport. A caller-
// supplied HTTPClient becomes the base client; static Headers are layered on top.
func httpClientFor(cfg TransportConfig) *http.Client {
	if cfg.HTTPClient == nil {
		return httpClientWithHeaders(cfg.Headers)
	}
	if len(cfg.Headers) == 0 {
		return cfg.HTTPClient
	}
	copied := *cfg.HTTPClient
	base := copied.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	copied.Transport = headerRoundTripper{
		base:    base,
		headers: cfg.Headers,
	}
	return &copied
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
