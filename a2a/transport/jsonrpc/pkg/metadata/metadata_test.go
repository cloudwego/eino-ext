package metadata

import (
	"context"
	"testing"
)

func TestMetadataValues(t *testing.T) {
	ctx := context.Background()
	_, ok := GetValue(ctx, "k")
	if ok {
		t.Fatalf("expected empty metadata")
	}
	ctx = WithValue(ctx, "k", "v")
	if val, ok := GetValue(ctx, "k"); !ok || val != "v" {
		t.Fatalf("unexpected value: %v %v", val, ok)
	}
	all, ok := GetAllValues(ctx)
	if !ok || all["k"] != "v" {
		t.Fatalf("unexpected all values")
	}
	ctx = WithValue(ctx, "k", "v2")
	if val, ok := GetValue(ctx, "k"); !ok || val != "v2" {
		t.Fatalf("unexpected value: %v %v", val, ok)
	}
	ctx = ClearValue(ctx)
	if _, ok := GetValue(ctx, "k"); ok {
		t.Fatalf("expected cleared metadata")
	}
}
