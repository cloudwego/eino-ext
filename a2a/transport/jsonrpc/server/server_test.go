package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
)

type fakeTransport struct {
	cli core.ClientRounder
	srv core.ServerRounder
}

func (f *fakeTransport) ClientCapability() (core.ClientRounder, bool) {
	if f.cli == nil {
		return nil, false
	}
	return f.cli, true
}

func (f *fakeTransport) ServerCapability() (core.ServerRounder, bool) {
	if f.srv == nil {
		return nil, false
	}
	return f.srv, true
}

type fakeClientRounder struct{}

func (f *fakeClientRounder) Round(ctx context.Context, msg core.Message) (core.MessageReader, error) {
	return nil, nil
}

func TestBuildOptions(t *testing.T) {
	opt := buildOptions(
		WithPingPongHandler("m", func(ctx context.Context, conn core.Connection, req json.RawMessage) (interface{}, error) {
			return nil, nil
		}),
	)
	if len(opt.ppHdls) != 1 {
		t.Fatalf("unexpected handlers")
	}
}

func TestOnTransport(t *testing.T) {
	hdl := newJsonRPCHandler(buildOptions())
	err := hdl.OnTransport(context.Background(), &fakeTransport{})
	if err == nil {
		t.Fatalf("expected error")
	}
	hdl = newJsonRPCHandler(buildOptions())
	err = hdl.OnTransport(context.Background(), &fakeTransport{cli: &fakeClientRounder{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
