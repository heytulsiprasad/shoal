// Package peer implements the BitTorrent peer wire protocol: the TCP handshake
// that opens a connection to another peer, and the small set of messages the
// two sides exchange to trade pieces.
package peer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const protocolName = "BitTorrent protocol"

// Handshake is the fixed-format greeting both peers send first.
//
// Layout: 1 byte length of the protocol string (19), the string itself,
// 8 reserved bytes, the 20-byte infohash, and the 20-byte peer id.
type Handshake struct {
	InfoHash [20]byte
	PeerID   [20]byte
}

// Serialize encodes the handshake (always 68 bytes for this protocol string).
func (h *Handshake) Serialize() []byte {
	buf := make([]byte, 49+len(protocolName))
	buf[0] = byte(len(protocolName))
	n := 1
	n += copy(buf[n:], protocolName)
	n += copy(buf[n:], make([]byte, 8)) // 8 reserved bytes, all zero
	n += copy(buf[n:], h.InfoHash[:])
	copy(buf[n:], h.PeerID[:])
	return buf
}

// ReadHandshake reads and parses a handshake from r.
func ReadHandshake(r io.Reader) (*Handshake, error) {
	var lenBuf [1]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	plen := int(lenBuf[0])
	if plen == 0 {
		return nil, errors.New("peer: zero-length protocol identifier")
	}
	buf := make([]byte, plen+48) // protocol string + 8 reserved + 20 + 20
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	var h Handshake
	copy(h.InfoHash[:], buf[plen+8:plen+28])
	copy(h.PeerID[:], buf[plen+28:plen+48])
	return &h, nil
}

// Conn is a live connection to a single peer after a successful handshake.
type Conn struct {
	conn     net.Conn
	Bitfield Bitfield
	Choked   bool // true while the peer is choking us (won't serve data)
	infoHash [20]byte
	peerID   [20]byte
}

// Connect dials the peer, performs the handshake (verifying the infohash
// matches), and reads the peer's opening bitfield.
//
// Simplification worth knowing: this expects the peer to send its BITFIELD as
// the first message after the handshake. That is the common case, but the spec
// allows a peer to skip it (and announce pieces via HAVE instead). Peers that
// do that are simply dropped here; a fuller client would tolerate it.
func Connect(addr string, infoHash, peerID [20]byte) (*Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		conn.Close()
		return nil, err
	}

	if _, err := conn.Write((&Handshake{InfoHash: infoHash, PeerID: peerID}).Serialize()); err != nil {
		conn.Close()
		return nil, err
	}
	resp, err := ReadHandshake(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !bytes.Equal(resp.InfoHash[:], infoHash[:]) {
		conn.Close()
		return nil, errors.New("peer: infohash mismatch in handshake")
	}

	bf, err := readBitfield(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Clear the connect-time deadline; per-piece deadlines are set later.
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, err
	}
	return &Conn{conn: conn, Bitfield: bf, Choked: true, infoHash: infoHash, peerID: peerID}, nil
}

func readBitfield(conn net.Conn) (Bitfield, error) {
	msg, err := ReadMessage(conn)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, errors.New("peer: expected bitfield, got keep-alive")
	}
	if msg.ID != MsgBitfield {
		return nil, fmt.Errorf("peer: expected bitfield, got message id %d", msg.ID)
	}
	return Bitfield(msg.Payload), nil
}

// Read reads the next message from the peer.
func (c *Conn) Read() (*Message, error) { return ReadMessage(c.conn) }

func (c *Conn) send(m *Message) error {
	_, err := c.conn.Write(m.Serialize())
	return err
}

func (c *Conn) SendInterested() error    { return c.send(&Message{ID: MsgInterested}) }
func (c *Conn) SendNotInterested() error { return c.send(&Message{ID: MsgNotInterested}) }
func (c *Conn) SendUnchoke() error       { return c.send(&Message{ID: MsgUnchoke}) }
func (c *Conn) SendChoke() error         { return c.send(&Message{ID: MsgChoke}) }

// SendRequest asks the peer for a block of piece data.
func (c *Conn) SendRequest(index, begin, length int) error {
	return c.send(FormatRequest(index, begin, length))
}

// SendHave tells the peer we now have a piece.
func (c *Conn) SendHave(index int) error { return c.send(FormatHave(index)) }

// SetDeadline sets the read/write deadline on the underlying connection.
func (c *Conn) SetDeadline(t time.Time) error { return c.conn.SetDeadline(t) }

// Close closes the connection.
func (c *Conn) Close() error { return c.conn.Close() }
