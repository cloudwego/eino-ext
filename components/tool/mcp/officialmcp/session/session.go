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

type ManagedSession struct {
	Name    string
	Session *mcp.ClientSession

	closeOnce sync.Once
	closeErr  error
}

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

func Connect(ctx context.Context, cfg ServerConfig) (*ManagedSession, error) {
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
	return &ManagedSession{Name: cfg.Name, Session: session}, nil
}

func (m *ManagedSession) Close(ctx context.Context) error {
	if m == nil || m.Session == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() {
		m.closeOnce.Do(func() {
			m.closeErr = m.Session.Close()
		})
		done <- m.closeErr
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
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
			Kind: officialmcp.ErrorKindUnsupportedTransport,
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
