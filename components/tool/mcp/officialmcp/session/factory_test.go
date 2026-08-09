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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	officialmcp "github.com/cloudwego/eino-ext/components/tool/mcp/officialmcp"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAddServer returns a server with an "add" tool, like newStreamableServerHandler
// but transport-agnostic so it can be attached to in-memory pipes.
func newAddServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "add two numbers"}, func(ctx context.Context, req *mcp.CallToolRequest, args addParams) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("%d", args.X+args.Y)}},
		}, nil, nil
	})
	return server
}

// pipeFactory builds transports that bridge the client to server over a fresh
// in-memory pipe on every invocation, recording each server-side session so
// tests can kill the current connection. The server is connected before the
// client, as mcp.NewInMemoryTransports requires.
type pipeFactory struct {
	server *mcp.Server
	calls  int32

	mu               sync.Mutex
	serverSession    []*mcp.ServerSession
	clientTransports []*countingTransport
}

type countingTransport struct {
	inner mcp.Transport
	calls int32
}

func (t *countingTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	atomic.AddInt32(&t.calls, 1)
	return t.inner.Connect(ctx)
}

type invalidatingTransport struct {
	inner mcp.Transport
}

func (t invalidatingTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	connection, err := t.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &invalidatingConnection{inner: connection}, nil
}

type invalidatingConnection struct {
	inner mcp.Connection
}

func (c *invalidatingConnection) Read(ctx context.Context) (jsonrpc.Message, error) {
	return c.inner.Read(ctx)
}

func (c *invalidatingConnection) Write(ctx context.Context, message jsonrpc.Message) error {
	if request, ok := message.(*jsonrpc.Request); ok && request.Method == "tools/call" {
		return officialmcp.MarkConnectionInvalid(&officialmcp.Error{
			Kind: officialmcp.ErrorKindUncertainOutcome,
			Err:  errors.New("response lost"),
		})
	}
	return c.inner.Write(ctx, message)
}

func (c *invalidatingConnection) Close() error { return c.inner.Close() }

func (c *invalidatingConnection) SessionID() string { return c.inner.SessionID() }

type terminalTransport struct {
	inner mcp.Transport
}

func (t terminalTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	connection, err := t.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &terminalConnection{inner: connection}, nil
}

type terminalConnection struct {
	inner mcp.Connection
}

func (c *terminalConnection) Read(ctx context.Context) (jsonrpc.Message, error) {
	return c.inner.Read(ctx)
}

func (c *terminalConnection) Write(ctx context.Context, message jsonrpc.Message) error {
	if request, ok := message.(*jsonrpc.Request); ok && request.Method == "tools/call" {
		return officialmcp.MarkSessionTerminal(errors.New("invalid tunnel response"))
	}
	return c.inner.Write(ctx, message)
}

func (c *terminalConnection) Close() error { return c.inner.Close() }

func (c *terminalConnection) SessionID() string { return c.inner.SessionID() }

func (f *pipeFactory) factory(ctx context.Context) (mcp.Transport, error) {
	atomic.AddInt32(&f.calls, 1)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ss, err := f.server.Connect(ctx, serverTransport, nil)
	if err != nil {
		return nil, err
	}
	countingClientTransport := &countingTransport{inner: clientTransport}
	f.mu.Lock()
	f.serverSession = append(f.serverSession, ss)
	f.clientTransports = append(f.clientTransports, countingClientTransport)
	f.mu.Unlock()
	return countingClientTransport, nil
}

func (f *pipeFactory) callCount() int32 {
	return atomic.LoadInt32(&f.calls)
}

func (f *pipeFactory) transportConnectCounts() []int32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	counts := make([]int32, len(f.clientTransports))
	for index, transport := range f.clientTransports {
		counts[index] = atomic.LoadInt32(&transport.calls)
	}
	return counts
}

// closeLastServerSession kills the server side of the most recent pipe,
// breaking the current client connection.
func (f *pipeFactory) closeLastServerSession() error {
	f.mu.Lock()
	ss := f.serverSession[len(f.serverSession)-1]
	f.mu.Unlock()
	return ss.Close()
}

func TestConnectViaFactory(t *testing.T) {
	ctx := context.Background()
	pf := &pipeFactory{server: newAddServer()}

	managed, err := Connect(ctx, ServerConfig{
		Name:      "test",
		Transport: TransportConfig{Factory: pf.factory},
	})
	require.NoError(t, err)
	defer managed.Close()

	result, err := managed.CallTool(ctx, &mcp.CallToolParams{Name: "add", Arguments: map[string]any{"x": 1, "y": 2}})
	require.NoError(t, err)
	assert.Equal(t, "3", result.Content[0].(*mcp.TextContent).Text)
	assert.Equal(t, int32(1), pf.callCount())
	assert.Equal(t, []int32{1}, pf.transportConnectCounts())
}

func TestConnectFactoryError(t *testing.T) {
	_, err := Connect(context.Background(), ServerConfig{
		Name: "bad",
		Transport: TransportConfig{
			Factory: func(context.Context) (mcp.Transport, error) {
				return nil, errors.New("factory dial failed")
			},
		},
	})
	require.Error(t, err)
	var startupErr *StartupError
	require.ErrorAs(t, err, &startupErr)
	assert.Contains(t, err.Error(), "factory dial failed")
}

func TestConnectFactoryRejectsNilTransport(t *testing.T) {
	_, err := Connect(context.Background(), ServerConfig{
		Name: "bad",
		Transport: TransportConfig{
			Factory: func(context.Context) (mcp.Transport, error) { return nil, nil },
		},
	})
	require.Error(t, err)
	var startupErr *StartupError
	require.ErrorAs(t, err, &startupErr)
	assert.Contains(t, err.Error(), "transport factory returned nil")
}

func TestConnectRejectsInvalidReplayPolicyBeforeFactory(t *testing.T) {
	var factoryCalls int32
	_, err := Connect(context.Background(), ServerConfig{
		Name:   "bad",
		Replay: ReplayPolicies{CallTool: ReplayPolicy(255)},
		Transport: TransportConfig{Factory: func(context.Context) (mcp.Transport, error) {
			atomic.AddInt32(&factoryCalls, 1)
			return nil, errors.New("must not run")
		}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid official mcp call tool replay policy")
	assert.Equal(t, int32(0), atomic.LoadInt32(&factoryCalls))
}

func TestReconnectReinvokesFactory(t *testing.T) {
	ctx := context.Background()
	pf := &pipeFactory{server: newAddServer()}

	rs, err := Connect(ctx, ServerConfig{
		Name:      "test",
		Transport: TransportConfig{Factory: pf.factory},
	})
	require.NoError(t, err)
	defer rs.Close()

	res, err := rs.CallTool(ctx, &mcp.CallToolParams{Name: "add", Arguments: map[string]any{"x": 1, "y": 2}})
	require.NoError(t, err)
	assert.Equal(t, "3", res.Content[0].(*mcp.TextContent).Text)

	stale, err := rs.current()
	require.NoError(t, err)

	// Kill the server side of the current pipe. The client notices the broken
	// connection asynchronously, and only then do its calls fail
	// connection-level (a write that wins the race against the teardown
	// surfaces as a raw io error), so wait for that before triggering the
	// reconnect. Ping the raw session directly to avoid Session's own retry.
	require.NoError(t, pf.closeLastServerSession())
	require.Eventually(t, func() bool {
		return officialmcp.IsConnectionError(stale.Ping(ctx, nil))
	}, 5*time.Second, time.Millisecond)

	res, err = rs.CallTool(ctx, &mcp.CallToolParams{Name: "add", Arguments: map[string]any{"x": 4, "y": 5}})
	require.NoError(t, err)
	assert.Equal(t, "9", res.Content[0].(*mcp.TextContent).Text)

	assert.Equal(t, int32(2), pf.callCount())
	assert.Equal(t, []int32{1, 1}, pf.transportConnectCounts())
	sessionAfter, err := rs.current()
	require.NoError(t, err)
	assert.NotSame(t, stale, sessionAfter, "expected a new underlying session after reconnect")
}

func TestReplaySafeCallToolReconnectsWithoutReplaying(t *testing.T) {
	ctx := context.Background()
	var toolCalls int32
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "add two numbers"}, func(ctx context.Context, req *mcp.CallToolRequest, args addParams) (*mcp.CallToolResult, any, error) {
		atomic.AddInt32(&toolCalls, 1)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("%d", args.X+args.Y)}},
		}, nil, nil
	})
	pf := &pipeFactory{server: server}

	managed, err := Connect(ctx, ServerConfig{
		Name:      "test",
		Transport: TransportConfig{Factory: pf.factory},
		Replay:    ReplayPolicies{CallTool: ReplaySafe},
	})
	require.NoError(t, err)
	defer managed.Close()

	result, err := managed.CallTool(ctx, &mcp.CallToolParams{Name: "add", Arguments: map[string]any{"x": 1, "y": 2}})
	require.NoError(t, err)
	assert.Equal(t, "3", result.Content[0].(*mcp.TextContent).Text)
	assert.Equal(t, int32(1), atomic.LoadInt32(&toolCalls))

	stale, err := managed.current()
	require.NoError(t, err)
	require.NoError(t, pf.closeLastServerSession())
	require.Eventually(t, func() bool {
		return officialmcp.IsConnectionError(stale.Ping(ctx, nil))
	}, 5*time.Second, time.Millisecond)

	_, err = managed.CallTool(ctx, &mcp.CallToolParams{Name: "add", Arguments: map[string]any{"x": 4, "y": 5}})
	require.Error(t, err)
	assert.True(t, officialmcp.IsErrorKind(err, officialmcp.ErrorKindUncertainOutcome))
	assert.False(t, officialmcp.IsConnectionError(err))
	assert.Equal(t, int32(1), atomic.LoadInt32(&toolCalls), "failed call must not be replayed on the fresh connection")
	assert.Equal(t, int32(2), pf.callCount())

	result, err = managed.CallTool(ctx, &mcp.CallToolParams{Name: "add", Arguments: map[string]any{"x": 4, "y": 5}})
	require.NoError(t, err)
	assert.Equal(t, "9", result.Content[0].(*mcp.TextContent).Text)
	assert.Equal(t, int32(2), atomic.LoadInt32(&toolCalls))
	assert.Equal(t, []int32{1, 1}, pf.transportConnectCounts())
}

func TestReplaySafeListToolsUsesCursorBoundary(t *testing.T) {
	t.Run("empty cursor replays", func(t *testing.T) {
		ctx := context.Background()
		pf := &pipeFactory{server: newAddServer()}
		managed, err := Connect(ctx, ServerConfig{
			Name:      "test",
			Transport: TransportConfig{Factory: pf.factory},
			Replay:    ReplayPolicies{ListTools: ReplaySafe},
		})
		require.NoError(t, err)
		defer managed.Close()

		stale, err := managed.current()
		require.NoError(t, err)
		require.NoError(t, pf.closeLastServerSession())
		require.Eventually(t, func() bool {
			return officialmcp.IsConnectionError(stale.Ping(ctx, nil))
		}, 5*time.Second, time.Millisecond)

		result, err := managed.ListTools(ctx, &mcp.ListToolsParams{})
		require.NoError(t, err)
		assert.Len(t, result.Tools, 1)
		assert.Equal(t, int32(2), pf.callCount())
	})

	t.Run("nonempty cursor reconnects without replay", func(t *testing.T) {
		ctx := context.Background()
		pf := &pipeFactory{server: newAddServer()}
		managed, err := Connect(ctx, ServerConfig{
			Name:      "test",
			Transport: TransportConfig{Factory: pf.factory},
			Replay:    ReplayPolicies{ListTools: ReplaySafe},
		})
		require.NoError(t, err)
		defer managed.Close()

		stale, err := managed.current()
		require.NoError(t, err)
		require.NoError(t, pf.closeLastServerSession())
		require.Eventually(t, func() bool {
			return officialmcp.IsConnectionError(stale.Ping(ctx, nil))
		}, 5*time.Second, time.Millisecond)

		_, err = managed.ListTools(ctx, &mcp.ListToolsParams{Cursor: "generation-bound-cursor"})
		require.Error(t, err)
		assert.True(t, officialmcp.IsErrorKind(err, officialmcp.ErrorKindConnection))
		assert.True(t, officialmcp.IsConnectionError(err))
		assert.Equal(t, int32(2), pf.callCount())

		result, err := managed.ListTools(ctx, &mcp.ListToolsParams{})
		require.NoError(t, err)
		assert.Len(t, result.Tools, 1)
		assert.Equal(t, int32(2), pf.callCount())
	})
}

func TestReplaySafeMethodSemantics(t *testing.T) {
	assert.True(t, shouldReplay(ReplayDefault, false))
	assert.True(t, shouldReplay(ReplayAlways, false))
	assert.False(t, shouldReplay(ReplayNever, true))
	assert.True(t, shouldReplay(ReplaySafe, true))
	assert.False(t, shouldReplay(ReplaySafe, false))
}

func TestConnectionInvalidRebuildsWithoutReplaying(t *testing.T) {
	ctx := context.Background()
	var toolCalls int32
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "add two numbers"}, func(ctx context.Context, req *mcp.CallToolRequest, args addParams) (*mcp.CallToolResult, any, error) {
		atomic.AddInt32(&toolCalls, 1)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("%d", args.X+args.Y)}},
		}, nil, nil
	})
	pf := &pipeFactory{server: server}
	var factoryCalls int32
	managed, err := Connect(ctx, ServerConfig{
		Name: "test",
		Transport: TransportConfig{Factory: func(ctx context.Context) (mcp.Transport, error) {
			transport, err := pf.factory(ctx)
			if err != nil {
				return nil, err
			}
			if atomic.AddInt32(&factoryCalls, 1) == 1 {
				return invalidatingTransport{inner: transport}, nil
			}
			return transport, nil
		}},
		Replay: ReplayPolicies{CallTool: ReplaySafe},
	})
	require.NoError(t, err)
	defer managed.Close()

	_, err = managed.CallTool(ctx, &mcp.CallToolParams{
		Name:      "add",
		Arguments: map[string]any{"x": 1, "y": 2},
	})
	require.Error(t, err)
	assert.True(t, officialmcp.IsErrorKind(err, officialmcp.ErrorKindUncertainOutcome))
	assert.True(t, officialmcp.IsConnectionInvalid(err))
	assert.False(t, officialmcp.IsConnectionError(err))
	assert.Equal(t, int32(0), atomic.LoadInt32(&toolCalls), "failed call must not reach the server")
	assert.Equal(t, int32(2), pf.callCount(), "connection should rebuild without replaying CallTool")

	result, err := managed.CallTool(ctx, &mcp.CallToolParams{
		Name:      "add",
		Arguments: map[string]any{"x": 4, "y": 5},
	})
	require.NoError(t, err)
	assert.Equal(t, "9", result.Content[0].(*mcp.TextContent).Text)
	assert.Equal(t, int32(1), atomic.LoadInt32(&toolCalls))
}

func TestSessionTerminalErrorClosesManagedSession(t *testing.T) {
	ctx := context.Background()
	pf := &pipeFactory{server: newAddServer()}
	var factoryCalls int32
	managed, err := Connect(ctx, ServerConfig{
		Name: "test",
		Transport: TransportConfig{Factory: func(ctx context.Context) (mcp.Transport, error) {
			transport, err := pf.factory(ctx)
			if err != nil {
				return nil, err
			}
			atomic.AddInt32(&factoryCalls, 1)
			return terminalTransport{inner: transport}, nil
		}},
	})
	require.NoError(t, err)

	_, err = managed.CallTool(ctx, &mcp.CallToolParams{
		Name:      "add",
		Arguments: map[string]any{"x": 1, "y": 2},
	})
	require.Error(t, err)
	assert.True(t, officialmcp.IsSessionTerminal(err))
	assert.False(t, officialmcp.IsConnectionError(err))

	_, err = managed.CallTool(ctx, &mcp.CallToolParams{Name: "add"})
	require.ErrorIs(t, err, ErrSessionClosed)
	assert.Equal(t, int32(1), atomic.LoadInt32(&factoryCalls), "terminal session must not reconnect")
	require.NoError(t, managed.Close())
}

func TestCloseDoesNotWaitForBlockedReconnectFactory(t *testing.T) {
	ctx := context.Background()
	pf := &pipeFactory{server: newAddServer()}
	started := make(chan struct{})
	release := make(chan struct{})
	var calls int32
	factory := func(ctx context.Context) (mcp.Transport, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return pf.factory(ctx)
		}
		close(started)
		<-release // Deliberately ignore ctx to model a misbehaving dialer.
		return nil, errors.New("late reconnect failure")
	}

	managed, err := Connect(ctx, ServerConfig{
		Name:      "test",
		Transport: TransportConfig{Factory: factory},
	})
	require.NoError(t, err)
	stale, err := managed.current()
	require.NoError(t, err)

	reconnectDone := make(chan error, 1)
	go func() {
		_, reconnectErr := managed.reconnect(ctx, stale)
		reconnectDone <- reconnectErr
	}()
	<-started

	waiterDone := make(chan error, 1)
	go func() {
		_, reconnectErr := managed.reconnect(ctx, stale)
		waiterDone <- reconnectErr
	}()

	closeDone := make(chan error, 1)
	go func() { closeDone <- managed.Close() }()
	select {
	case closeErr := <-closeDone:
		require.NoError(t, closeErr)
	case <-time.After(time.Second):
		t.Fatal("Close blocked behind reconnect factory")
	}
	select {
	case waiterErr := <-waiterDone:
		require.ErrorIs(t, waiterErr, ErrSessionClosed)
	case <-time.After(time.Second):
		t.Fatal("reconnect waiter did not observe Close")
	}

	close(release)
	require.ErrorIs(t, <-reconnectDone, ErrSessionClosed)
	_, err = managed.current()
	require.ErrorIs(t, err, ErrSessionClosed)
}
