package utils

import "testing"

func TestUnboundBuffer(t *testing.T) {
	buf := NewUnboundBuffer[int]()
	for i := 0; i < 257; i++ {
		buf.Push(i)
	}
	if first := <-buf.PopChan(); first != 0 {
		t.Fatalf("unexpected first: %d", first)
	}
	buf.Load()
	last := 0
	for i := 0; i < 256; i++ {
		last = <-buf.PopChan()
	}
	if last != 256 {
		t.Fatalf("unexpected last: %d", last)
	}
}
