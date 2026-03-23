package rpcinfo

import (
	"context"
	"testing"
)

func TestRPCInfo(t *testing.T) {
	inv := NewInvocation("1", "m")
	ri := NewRPCInfo(inv)
	if ri.Invocation().ID() != "1" {
		t.Fatalf("unexpected id: %s", ri.Invocation().ID())
	}
	if ri.Invocation().MethodName() != "m" {
		t.Fatalf("unexpected method: %s", ri.Invocation().MethodName())
	}
}

func TestRPCInfoContext(t *testing.T) {
	inv := NewInvocation("2", "method")
	ri := NewRPCInfo(inv)
	ctx := NewCtxWithRPCInfo(context.Background(), ri)
	got := RPCInfoFromCtx(ctx)
	if got.Invocation().ID() != "2" {
		t.Fatalf("unexpected id: %s", got.Invocation().ID())
	}
}
