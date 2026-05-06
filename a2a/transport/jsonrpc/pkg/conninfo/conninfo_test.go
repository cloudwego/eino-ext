package conninfo

import "testing"

func TestConnInfo(t *testing.T) {
	from := NewPeer(PeerTypeAddress, "127.0.0.1")
	to := NewPeer(PeerTypeURL, "http://example.com")
	info := NewConnInfo(from, to)
	if info.From().Address() != "127.0.0.1" {
		t.Fatalf("unexpected from address")
	}
	if info.To().Type() != PeerTypeURL {
		t.Fatalf("unexpected to type")
	}
}
