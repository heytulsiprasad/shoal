// Package bencode implements the "bencoding" format that BitTorrent uses for
// .torrent files and tracker responses. It is deliberately small and written
// by hand so you can read the whole grammar in one sitting.
//
// Bencode has exactly four types:
//
//	i42e            integer (42)
//	4:spam          byte string (length-prefixed; can hold arbitrary bytes)
//	l...e           list
//	d...e           dictionary (keys are byte strings, sorted, each followed by a value)
//
// Decoded values map to Go types as: int64, string, []any, map[string]any.
// Strings are used for byte strings even when they hold binary data (e.g. the
// concatenated 20-byte piece hashes), which is fine because a Go string can
// hold any bytes.
package bencode

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
)

// Decode parses a single bencoded value and requires the entire input to be
// consumed (no trailing garbage).
func Decode(data []byte) (any, error) {
	p := &parser{buf: data}
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.buf) {
		return nil, fmt.Errorf("bencode: %d trailing byte(s) after value", len(p.buf)-p.pos)
	}
	return v, nil
}

// DecodeTopDict parses a top-level dictionary and, alongside the decoded value,
// returns the *exact original bytes* of each key's value.
//
// This matters for one specific reason: a torrent's infohash is the SHA-1 of
// the raw bytes of its "info" dictionary as they appear in the file. If you
// decoded and then re-encoded the info dict, any difference (key ordering,
// integer formatting, unknown extra keys) would change the hash and you'd talk
// to the wrong swarm. Capturing the raw span avoids that entirely.
func DecodeTopDict(data []byte) (decoded map[string]any, raw map[string][]byte, err error) {
	p := &parser{buf: data}
	if p.pos >= len(p.buf) || p.buf[p.pos] != 'd' {
		return nil, nil, errors.New("bencode: top level is not a dictionary")
	}
	p.pos++ // consume 'd'
	decoded = map[string]any{}
	raw = map[string][]byte{}
	for {
		if p.pos >= len(p.buf) {
			return nil, nil, errors.New("bencode: unterminated dictionary")
		}
		if p.buf[p.pos] == 'e' {
			p.pos++
			return decoded, raw, nil
		}
		key, err := p.str()
		if err != nil {
			return nil, nil, err
		}
		start := p.pos
		v, err := p.value()
		if err != nil {
			return nil, nil, err
		}
		decoded[key] = v
		raw[key] = data[start:p.pos]
	}
}

type parser struct {
	buf []byte
	pos int
}

func (p *parser) value() (any, error) {
	if p.pos >= len(p.buf) {
		return nil, errors.New("bencode: unexpected end of input")
	}
	c := p.buf[p.pos]
	switch {
	case c == 'i':
		return p.integer()
	case c == 'l':
		return p.list()
	case c == 'd':
		return p.dict()
	case c >= '0' && c <= '9':
		return p.str()
	default:
		return nil, fmt.Errorf("bencode: unexpected byte %q at offset %d", c, p.pos)
	}
}

func (p *parser) integer() (int64, error) {
	p.pos++ // consume 'i'
	start := p.pos
	for p.pos < len(p.buf) && p.buf[p.pos] != 'e' {
		p.pos++
	}
	if p.pos >= len(p.buf) {
		return 0, errors.New("bencode: unterminated integer")
	}
	tok := string(p.buf[start:p.pos])
	p.pos++ // consume 'e'
	n, err := strconv.ParseInt(tok, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bencode: invalid integer %q", tok)
	}
	return n, nil
}

func (p *parser) str() (string, error) {
	start := p.pos
	for p.pos < len(p.buf) && p.buf[p.pos] != ':' {
		if p.buf[p.pos] < '0' || p.buf[p.pos] > '9' {
			return "", fmt.Errorf("bencode: invalid string length at offset %d", start)
		}
		p.pos++
	}
	if p.pos >= len(p.buf) {
		return "", errors.New("bencode: unterminated string length")
	}
	n, err := strconv.Atoi(string(p.buf[start:p.pos]))
	if err != nil || n < 0 {
		return "", fmt.Errorf("bencode: invalid string length %q", string(p.buf[start:p.pos]))
	}
	p.pos++ // consume ':'
	if p.pos+n > len(p.buf) {
		return "", errors.New("bencode: string runs past end of input")
	}
	s := string(p.buf[p.pos : p.pos+n])
	p.pos += n
	return s, nil
}

func (p *parser) list() ([]any, error) {
	p.pos++ // consume 'l'
	out := []any{}
	for {
		if p.pos >= len(p.buf) {
			return nil, errors.New("bencode: unterminated list")
		}
		if p.buf[p.pos] == 'e' {
			p.pos++
			return out, nil
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
}

func (p *parser) dict() (map[string]any, error) {
	p.pos++ // consume 'd'
	out := map[string]any{}
	for {
		if p.pos >= len(p.buf) {
			return nil, errors.New("bencode: unterminated dictionary")
		}
		if p.buf[p.pos] == 'e' {
			p.pos++
			return out, nil
		}
		key, err := p.str()
		if err != nil {
			return nil, err
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		out[key] = v
	}
}

// Encode serializes a Go value back into bencode. Dictionary keys are emitted
// in sorted (bytewise) order, as the spec requires. Supported input types are
// string, []byte, int, int64, []any, and map[string]any.
func Encode(v any) ([]byte, error) {
	var b []byte
	var enc func(any) error
	enc = func(v any) error {
		switch x := v.(type) {
		case string:
			b = append(b, strconv.Itoa(len(x))...)
			b = append(b, ':')
			b = append(b, x...)
		case []byte:
			b = append(b, strconv.Itoa(len(x))...)
			b = append(b, ':')
			b = append(b, x...)
		case int:
			b = append(b, 'i')
			b = append(b, strconv.Itoa(x)...)
			b = append(b, 'e')
		case int64:
			b = append(b, 'i')
			b = append(b, strconv.FormatInt(x, 10)...)
			b = append(b, 'e')
		case []any:
			b = append(b, 'l')
			for _, e := range x {
				if err := enc(e); err != nil {
					return err
				}
			}
			b = append(b, 'e')
		case map[string]any:
			b = append(b, 'd')
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys) // bytewise order, matching the spec
			for _, k := range keys {
				b = append(b, strconv.Itoa(len(k))...)
				b = append(b, ':')
				b = append(b, k...)
				if err := enc(x[k]); err != nil {
					return err
				}
			}
			b = append(b, 'e')
		default:
			return fmt.Errorf("bencode: cannot encode value of type %T", v)
		}
		return nil
	}
	if err := enc(v); err != nil {
		return nil, err
	}
	return b, nil
}
