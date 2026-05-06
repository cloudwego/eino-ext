package streaming

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/tracer"
)

type fakeClientAsync struct {
	recvCalled  bool
	closeCalled bool
	recvErr     error
}

func (f *fakeClientAsync) Recv(ctx context.Context, obj interface{}) error {
	f.recvCalled = true
	return f.recvErr
}

func (f *fakeClientAsync) Close() error {
	f.closeCalled = true
	return nil
}

type fakeConn struct {
	method string
	req    interface{}
	async  core.ClientAsync
	tracer tracer.Tracer
}

func (f *fakeConn) Call(ctx context.Context, method string, req, res interface{}, opts ...core.CallOption) error {
	return nil
}

func (f *fakeConn) AsyncCall(ctx context.Context, method string, req interface{}, opts ...core.CallOption) (core.ClientAsync, error) {
	f.method = method
	f.req = req
	return f.async, nil
}

func (f *fakeConn) Notify(ctx context.Context, method string, params interface{}, opts ...core.CallOption) error {
	return nil
}

func (f *fakeConn) GetTracer() tracer.Tracer {
	return f.tracer
}

type fakeServerAsync struct {
	finishedErr error
}

func (f *fakeServerAsync) SendStreaming(ctx context.Context, obj interface{}) error {
	return nil
}

func (f *fakeServerAsync) FinishStreaming(ctx context.Context, err error) error {
	f.finishedErr = err
	return nil
}

func (f *fakeServerAsync) Send(ctx context.Context, obj interface{}) error {
	return nil
}

func (f *fakeServerAsync) Finish(ctx context.Context, err error) error {
	return nil
}

func TestServerStreamingClient(t *testing.T) {
	async := &fakeClientAsync{recvErr: errors.New("recv")}
	conn := &fakeConn{async: async, tracer: tracer.NewNoopTracer()}
	ext, err := NewExtension(conn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cli, err := ext.ServerStreaming(context.Background(), "m", "req")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.method != "m" {
		t.Fatalf("unexpected method: %s", conn.method)
	}
	if err := cli.Recv(context.Background(), &struct{}{}); err == nil {
		t.Fatalf("expected error")
	}
	if !async.recvCalled {
		t.Fatalf("expected Recv called")
	}
	if err := cli.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if !async.closeCalled {
		t.Fatalf("expected Close called")
	}
}

func TestServerStreamingCallChain(t *testing.T) {
	order := ""
	mw1 := func(next ServerStreamingCallEndpoint) ServerStreamingCallEndpoint {
		return func(ctx context.Context, method string, req interface{}) (ServerStreamingClient, error) {
			order += "1"
			return next(ctx, method, req)
		}
	}
	mw2 := func(next ServerStreamingCallEndpoint) ServerStreamingCallEndpoint {
		return func(ctx context.Context, method string, req interface{}) (ServerStreamingClient, error) {
			order += "2"
			return next(ctx, method, req)
		}
	}
	end := serverStreamingCallChain(mw1, mw2)(func(ctx context.Context, method string, req interface{}) (ServerStreamingClient, error) {
		order += "3"
		return &serverStreamingClient{}, nil
	})
	_, err := end(context.Background(), "m", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order != "123" {
		t.Fatalf("unexpected order: %s", order)
	}
}

func TestConvertRequestHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		async := &fakeServerAsync{}
		handler := convertRequestHandler(func(ctx context.Context, conn core.Connection, req json.RawMessage, srv ServerStreamingServer) error {
			return nil
		})
		err := handler(context.Background(), nil, &core.Request{}, async)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if async.finishedErr != nil {
			t.Fatalf("unexpected finish error: %v", async.finishedErr)
		}
	})
	t.Run("error", func(t *testing.T) {
		async := &fakeServerAsync{}
		handler := convertRequestHandler(func(ctx context.Context, conn core.Connection, req json.RawMessage, srv ServerStreamingServer) error {
			return errors.New("boom")
		})
		err := handler(context.Background(), nil, &core.Request{}, async)
		if err == nil {
			t.Fatalf("expected error")
		}
		if async.finishedErr == nil {
			t.Fatalf("expected finish error")
		}
	})
	t.Run("panic", func(t *testing.T) {
		async := &fakeServerAsync{}
		handler := convertRequestHandler(func(ctx context.Context, conn core.Connection, req json.RawMessage, srv ServerStreamingServer) error {
			panic(errors.New("boom"))
		})
		err := handler(context.Background(), nil, &core.Request{}, async)
		if err == nil {
			t.Fatalf("expected error")
		}
		if async.finishedErr == nil {
			t.Fatalf("expected finish error")
		}
	})
}
