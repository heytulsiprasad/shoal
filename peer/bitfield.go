package peer

// Bitfield records which pieces a peer has. It is a big-endian bitset: the
// high bit (0x80) of byte 0 is piece 0, the next bit is piece 1, and so on.
type Bitfield []byte

// HasPiece reports whether the bit for piece index is set.
func (bf Bitfield) HasPiece(index int) bool {
	byteIndex := index / 8
	offset := index % 8
	if byteIndex < 0 || byteIndex >= len(bf) {
		return false
	}
	return bf[byteIndex]>>uint(7-offset)&1 != 0
}

// SetPiece sets the bit for piece index (ignored if out of range).
func (bf Bitfield) SetPiece(index int) {
	byteIndex := index / 8
	offset := index % 8
	if byteIndex < 0 || byteIndex >= len(bf) {
		return
	}
	bf[byteIndex] |= 1 << uint(7-offset)
}
