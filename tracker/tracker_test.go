package tracker

import (
	"net"
	"testing"
)

func TestParseCompactPeers(t *testing.T) {
	// Two peers: 1.2.3.4:6881 and 10.0.0.1:6969.
	// 6881 = 0x1AE1, 6969 = 0x1B39.
	b := []byte{
		1, 2, 3, 4, 0x1A, 0xE1,
		10, 0, 0, 1, 0x1B, 0x39,
	}
	peers, err := parseCompactPeers(b)
	if err != nil {
		t.Fatalf("parseCompactPeers: %v", err)
	}
	if len(peers) != 2 {
		t.Fatalf("got %d peers, want 2", len(peers))
	}
	if !peers[0].IP.Equal(net.IPv4(1, 2, 3, 4)) || peers[0].Port != 6881 {
		t.Errorf("peer[0] = %s, want 1.2.3.4:6881", peers[0])
	}
	if !peers[1].IP.Equal(net.IPv4(10, 0, 0, 1)) || peers[1].Port != 6969 {
		t.Errorf("peer[1] = %s, want 10.0.0.1:6969", peers[1])
	}
}

func TestParseCompactPeersRejectsRagged(t *testing.T) {
	if _, err := parseCompactPeers([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected error for length not divisible by 6")
	}
}
