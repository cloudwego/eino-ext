package utils

import "testing"

func TestNewPanicErr(t *testing.T) {
	err := NewPanicErr("boom", []byte("stack"))
	if err == nil {
		t.Fatal("expected error")
	}
	want := "panic error: boom, \nstack: stack"
	if err.Error() != want {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}
