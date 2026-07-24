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
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	officialmcp "github.com/cloudwego/eino-ext/components/tool/mcp/officialmcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionImplementsClientSession(t *testing.T) {
	var _ officialmcp.ClientSession = (*Session)(nil)
}

// newStreamableServer returns a fresh streamable-http handler serving an "add" tool.
func newStreamableServerHandler() http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "add two numbers"}, func(ctx context.Context, req *mcp.CallToolRequest, args addParams) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("%d", args.X+args.Y)}},
		}, nil, nil
	})
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
}

func TestSessionReconnectsAfterServerRestart(t *testing.T) {
	ctx := context.Background()

	// A switchable handler: we swap the live server to simulate the original
	// connection dying and a fresh server coming up at the same URL.
	var mu sync.Mutex
	handler := newStreamableServerHandler()
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		h := handler
		mu.Unlock()
		h.ServeHTTP(w, r)
	})
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	rs, err := Connect(ctx, ServerConfig{
		Name:      "test",
		Transport: TransportConfig{Type: TransportStreamableHTTP, URL: httpServer.URL},
	})
	require.NoError(t, err)
	defer rs.Close()

	// First call works on the original session.
	res, err := rs.CallTool(ctx, &mcp.CallToolParams{Name: "add", Arguments: map[string]any{"x": 1, "y": 2}})
	require.NoError(t, err)
	assert.Equal(t, "3", res.Content[0].(*mcp.TextContent).Text)

	sessionBefore, err := rs.current()
	require.NoError(t, err)

	// Replace the server with a brand-new one. The old session's sessionID is now
	// unknown to the server, so the next call fails connection-level and triggers
	// a reconnect onto the fresh server.
	mu.Lock()
	handler = newStreamableServerHandler()
	mu.Unlock()

	res, err = rs.CallTool(ctx, &mcp.CallToolParams{Name: "add", Arguments: map[string]any{"x": 4, "y": 5}})
	require.NoError(t, err)
	assert.Equal(t, "9", res.Content[0].(*mcp.TextContent).Text)

	sessionAfter, err := rs.current()
	require.NoError(t, err)
	assert.NotSame(t, sessionBefore, sessionAfter, "expected a new underlying session after reconnect")
}

func TestReconnectFailureIsConnectionError(t *testing.T) {
	ctx := context.Background()

	// Seed a sentinel current session and point the config at an unreachable
	// endpoint, so reconnect takes the dial path (stale == current) and the dial
	// fails — no live server needed. The sentinel is never dereferenced because
	// connect fails before the stale session is touched.
	sentinel := &mcp.ClientSession{}
	s := &Session{
		Name:    "test",
		cfg:     ServerConfig{Name: "test", Transport: TransportConfig{Type: TransportStreamableHTTP, URL: "http://127.0.0.1:1"}},
		session: sentinel,
	}

	_, rerr := s.reconnect(ctx, sentinel)
	require.Error(t, rerr)
	// The reconnect (dial) failure must be recognizable as a connection-level
	// error, so upstream classification does not mistake an unreachable server
	// for a call/list protocol error. The StartupError is preserved as the cause.
	assert.True(t, officialmcp.IsConnectionError(rerr), "reconnect failure should be a connection error")
	var startup *StartupError
	assert.True(t, errors.As(rerr, &startup), "StartupError should be preserved as the cause")
}

func TestReconnectOnlyOncePerStaleSession(t *testing.T) {
	ctx := context.Background()
	httpServer := httptest.NewServer(newStreamableServerHandler())
	defer httpServer.Close()

	rs, err := Connect(ctx, ServerConfig{
		Name:      "test",
		Transport: TransportConfig{Type: TransportStreamableHTTP, URL: httpServer.URL},
	})
	require.NoError(t, err)
	defer rs.Close()

	stale, err := rs.current()
	require.NoError(t, err)

	// Concurrent reconnect calls referencing the same stale session must collapse
	// into a single Connect: only the first swaps, the rest observe the fresh one.
	var wg sync.WaitGroup
	var distinct int32
	results := make([]*mcp.ClientSession, 8)
	for i := range results {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s, rerr := rs.reconnect(ctx, stale)
			require.NoError(t, rerr)
			results[idx] = s
		}(i)
	}
	wg.Wait()

	first := results[0]
	for _, s := range results {
		if s != first {
			atomic.AddInt32(&distinct, 1)
		}
	}
	assert.Equal(t, int32(0), distinct, "all concurrent reconnects should yield the same fresh session")
	assert.NotSame(t, stale, first)
}
