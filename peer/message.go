package peer

import (
	"encoding/binary"
	"io"
)

// MessageID is the one-byte type tag of a peer wire-protocol message.
type MessageID uint8

const (
	MsgChoke         MessageID = 0 // peer won't send us data right now
	MsgUnchoke       MessageID = 1 // peer will send us data
	MsgInterested    MessageID = 2 // we want data from the peer
	MsgNotInterested MessageID = 3
	MsgHave          MessageID = 4 // peer just completed a piece (payload: index)
	MsgBitfield      MessageID = 5 // which pieces the peer has (payload: bitfield)
	MsgRequest       MessageID = 6 // ask for a block (payload: index, begin, length)
	MsgPiece         MessageID = 7 // a block of data (payload: index, begin, data)
	MsgCancel        MessageID = 8
)

// Message is a single peer wire-protocol message. On the wire it is framed as a
// 4-byte big-endian length prefix, then one ID byte, then the payload. A
// length prefix of zero is a "keep-alive" with no ID or payload, represented
// here by a nil *Message.
type Message struct {
	ID      MessageID
	Payload []byte
}

// Serialize encodes a message for the wire. A nil receiver is a keep-alive.
func (m *Message) Serialize() []byte {
	if m == nil {
		return make([]byte, 4) // four zero bytes = keep-alive
	}
	length := uint32(len(m.Payload) + 1) // +1 for the ID byte
	buf := make([]byte, 4+length)
	binary.BigEndian.PutUint32(buf[0:4], length)
	buf[4] = byte(m.ID)
	copy(buf[5:], m.Payload)
	return buf
}

// ReadMessage reads one framed message from r. It returns (nil, nil) for a
// keep-alive.
func ReadMessage(r io.Reader) (*Message, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf[:])
	if length == 0 {
		return nil, nil // keep-alive
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return &Message{ID: MessageID(buf[0]), Payload: buf[1:]}, nil
}

// FormatRequest builds a REQUEST for a block: a (index, begin, length) triple.
func FormatRequest(index, begin, length int) *Message {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], uint32(index))
	binary.BigEndian.PutUint32(payload[4:8], uint32(begin))
	binary.BigEndian.PutUint32(payload[8:12], uint32(length))
	return &Message{ID: MsgRequest, Payload: payload}
}

// FormatHave builds a HAVE announcing that we now hold the given piece.
func FormatHave(index int) *Message {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, uint32(index))
	return &Message{ID: MsgHave, Payload: payload}
}
