package peer

import (
	"bytes"
	"testing"
)

func TestHandshakeRoundTrip(t *testing.T) {
	var infoHash, peerID [20]byte
	for i := range infoHash {
		infoHash[i] = byte(i)
		peerID[i] = byte(i + 100)
	}
	h := &Handshake{InfoHash: infoHash, PeerID: peerID}
	serialized := h.Serialize()
	if len(serialized) != 68 {
		t.Fatalf("handshake length = %d, want 68", len(serialized))
	}

	got, err := ReadHandshake(bytes.NewReader(serialized))
	if err != nil {
		t.Fatalf("ReadHandshake: %v", err)
	}
	if got.InfoHash != infoHash {
		t.Errorf("InfoHash = %x, want %x", got.InfoHash, infoHash)
	}
	if got.PeerID != peerID {
		t.Errorf("PeerID = %x, want %x", got.PeerID, peerID)
	}
}

func TestMessageSerializeRoundTrip(t *testing.T) {
	m := &Message{ID: MsgPiece, Payload: []byte{1, 2, 3, 4}}
	got, err := ReadMessage(bytes.NewReader(m.Serialize()))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.ID != MsgPiece || !bytes.Equal(got.Payload, m.Payload) {
		t.Errorf("round trip = %+v, want %+v", got, m)
	}
}

func TestKeepAlive(t *testing.T) {
	var nilMsg *Message
	serialized := nilMsg.Serialize()
	if !bytes.Equal(serialized, []byte{0, 0, 0, 0}) {
		t.Fatalf("keep-alive = %v, want four zero bytes", serialized)
	}
	got, err := ReadMessage(bytes.NewReader(serialized))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got != nil {
		t.Errorf("keep-alive decoded to %+v, want nil", got)
	}
}

func TestFormatRequest(t *testing.T) {
	m := FormatRequest(1, 16384, 16384)
	if m.ID != MsgRequest {
		t.Errorf("ID = %d, want %d", m.ID, MsgRequest)
	}
	// index=1, begin=16384 (0x00004000), length=16384
	want := []byte{0, 0, 0, 1, 0, 0, 0x40, 0, 0, 0, 0x40, 0}
	if !bytes.Equal(m.Payload, want) {
		t.Errorf("payload = %v, want %v", m.Payload, want)
	}
}

func TestBitfield(t *testing.T) {
	// 0b10110000, 0b00000001 -> pieces 0,2,3 set in byte 0; piece 15 in byte 1.
	bf := Bitfield{0b10110000, 0b00000001}
	set := map[int]bool{0: true, 2: true, 3: true, 15: true}
	for i := 0; i < 16; i++ {
		if bf.HasPiece(i) != set[i] {
			t.Errorf("HasPiece(%d) = %v, want %v", i, bf.HasPiece(i), set[i])
		}
	}
	// Out of range is false, not a panic.
	if bf.HasPiece(99) {
		t.Error("HasPiece(99) = true, want false")
	}

	bf2 := Bitfield{0x00, 0x00}
	bf2.SetPiece(4)
	if !bf2.HasPiece(4) {
		t.Error("SetPiece(4) did not set the bit")
	}
	if bf2.HasPiece(5) {
		t.Error("SetPiece(4) incorrectly set piece 5")
	}
}
