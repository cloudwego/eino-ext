package tracer

import (
	"context"
	"testing"
)

func TestNoopTracer(t *testing.T) {
	tr := NewNoopTracer()
	ctx := context.Background()
	if tr.Start(ctx) != ctx {
		t.Fatalf("unexpected context")
	}
	tr.Finish(ctx)
}
