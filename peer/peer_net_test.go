package peer

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func testInfoHash() (h [20]byte) {
	for i := range h {
		h[i] = byte(i)
	}
	return
}

// startPeerServer listens on a loopback port and hands the first accepted
// connection to handle. It returns the address to dial.
func startPeerServer(t *testing.T, handle func(conn net.Conn)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		handle(conn)
	}()
	return ln.Addr().String()
}

func nextMsg(t *testing.T, ch <-chan *Message) *Message {
	t.Helper()
	select {
	case m := <-ch:
		return m
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
		return nil
	}
}

func TestConnectReadsBitfieldAndSends(t *testing.T) {
	infoHash := testInfoHash()
	bf := Bitfield{0b10100000} // pieces 0 and 2
	recv := make(chan *Message, 16)
	addr := startPeerServer(t, func(conn net.Conn) {
		if _, err := ReadHandshake(conn); err != nil {
			return
		}
		var pid [20]byte
		conn.Write((&Handshake{InfoHash: infoHash, PeerID: pid}).Serialize())
		conn.Write((&Message{ID: MsgBitfield, Payload: []byte(bf)}).Serialize())
		for {
			m, err := ReadMessage(conn)
			if err != nil {
				return
			}
			recv <- m
		}
	})

	var peerID [20]byte
	c, err := Connect(addr, infoHash, peerID)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	if !c.Choked {
		t.Error("expected Choked=true immediately after connect")
	}
	if !bytes.Equal([]byte(c.Bitfield), []byte(bf)) {
		t.Errorf("Bitfield = %v, want %v", c.Bitfield, bf)
	}
	if !c.Bitfield.HasPiece(0) || !c.Bitfield.HasPiece(2) || c.Bitfield.HasPiece(1) {
		t.Error("bitfield piece membership wrong")
	}

	checks := []struct {
		name string
		send func() error
		want MessageID
	}{
		{"unchoke", c.SendUnchoke, MsgUnchoke},
		{"interested", c.SendInterested, MsgInterested},
		{"not-interested", c.SendNotInterested, MsgNotInterested},
		{"choke", c.SendChoke, MsgChoke},
	}
	for _, ch := range checks {
		if err := ch.send(); err != nil {
			t.Fatalf("Send %s: %v", ch.name, err)
		}
		if m := nextMsg(t, recv); m.ID != ch.want {
			t.Errorf("%s: got id %d, want %d", ch.name, m.ID, ch.want)
		}
	}

	if err := c.SendRequest(2, 16384, 16384); err != nil {
		t.Fatal(err)
	}
	m := nextMsg(t, recv)
	wantReq := []byte{0, 0, 0, 2, 0, 0, 0x40, 0, 0, 0, 0x40, 0}
	if m.ID != MsgRequest || !bytes.Equal(m.Payload, wantReq) {
		t.Errorf("request = id %d payload %v, want id %d payload %v", m.ID, m.Payload, MsgRequest, wantReq)
	}

	if err := c.SendHave(7); err != nil {
		t.Fatal(err)
	}
	m = nextMsg(t, recv)
	if m.ID != MsgHave || !bytes.Equal(m.Payload, []byte{0, 0, 0, 7}) {
		t.Errorf("have = id %d payload %v, want id %d payload [0 0 0 7]", m.ID, m.Payload, MsgHave)
	}
}

func TestConnReadAndSetDeadline(t *testing.T) {
	infoHash := testInfoHash()
	addr := startPeerServer(t, func(conn net.Conn) {
		if _, err := ReadHandshake(conn); err != nil {
			return
		}
		var pid [20]byte
		conn.Write((&Handshake{InfoHash: infoHash, PeerID: pid}).Serialize())
		conn.Write((&Message{ID: MsgBitfield, Payload: []byte{0x00}}).Serialize())
		conn.Write((&Message{ID: MsgUnchoke}).Serialize())
		time.Sleep(200 * time.Millisecond)
	})

	var peerID [20]byte
	c, err := Connect(addr, infoHash, peerID)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	if err := c.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}
	m, err := c.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if m.ID != MsgUnchoke {
		t.Errorf("Read = id %d, want Unchoke", m.ID)
	}
}

func TestConnectInfohashMismatch(t *testing.T) {
	infoHash := testInfoHash()
	var wrong [20]byte
	for i := range wrong {
		wrong[i] = byte(255 - i)
	}
	addr := startPeerServer(t, func(conn net.Conn) {
		if _, err := ReadHandshake(conn); err != nil {
			return
		}
		var pid [20]byte
		conn.Write((&Handshake{InfoHash: wrong, PeerID: pid}).Serialize())
	})
	var peerID [20]byte
	if _, err := Connect(addr, infoHash, peerID); err == nil {
		t.Fatal("Connect expected infohash mismatch error")
	}
}

func TestConnectExpectsBitfield(t *testing.T) {
	infoHash := testInfoHash()
	addr := startPeerServer(t, func(conn net.Conn) {
		if _, err := ReadHandshake(conn); err != nil {
			return
		}
		var pid [20]byte
		conn.Write((&Handshake{InfoHash: infoHash, PeerID: pid}).Serialize())
		conn.Write((&Message{ID: MsgUnchoke}).Serialize()) // not a bitfield
	})
	var peerID [20]byte
	if _, err := Connect(addr, infoHash, peerID); err == nil {
		t.Fatal("Connect expected 'expected bitfield' error")
	}
}

func TestConnectKeepAliveInsteadOfBitfield(t *testing.T) {
	infoHash := testInfoHash()
	addr := startPeerServer(t, func(conn net.Conn) {
		if _, err := ReadHandshake(conn); err != nil {
			return
		}
		var pid [20]byte
		conn.Write((&Handshake{InfoHash: infoHash, PeerID: pid}).Serialize())
		conn.Write([]byte{0, 0, 0, 0}) // keep-alive, not a bitfield
	})
	var peerID [20]byte
	if _, err := Connect(addr, infoHash, peerID); err == nil {
		t.Fatal("Connect expected 'got keep-alive' error")
	}
}

func TestConnectDialFailure(t *testing.T) {
	var infoHash, peerID [20]byte
	if _, err := Connect("127.0.0.1:1", infoHash, peerID); err == nil {
		t.Fatal("Connect expected dial failure on a closed port")
	}
}

func TestReadHandshakeZeroLength(t *testing.T) {
	if _, err := ReadHandshake(bytes.NewReader([]byte{0})); err == nil {
		t.Fatal("ReadHandshake expected zero-length protocol error")
	}
}

func TestReadMessageTruncatedPayload(t *testing.T) {
	// length prefix says 5 bytes follow, but only 1 is present.
	if _, err := ReadMessage(bytes.NewReader([]byte{0, 0, 0, 5, 1})); err == nil {
		t.Fatal("ReadMessage expected error on truncated payload")
	}
}

func TestFormatHave(t *testing.T) {
	m := FormatHave(7)
	if m.ID != MsgHave || !bytes.Equal(m.Payload, []byte{0, 0, 0, 7}) {
		t.Errorf("FormatHave(7) = %+v, want id %d payload [0 0 0 7]", m, MsgHave)
	}
}

func TestSetPieceOutOfRange(t *testing.T) {
	bf := Bitfield{0, 0}
	bf.SetPiece(99) // out of range: must not panic and must not change anything
	for i := 0; i < 16; i++ {
		if bf.HasPiece(i) {
			t.Errorf("piece %d unexpectedly set", i)
		}
	}
}
