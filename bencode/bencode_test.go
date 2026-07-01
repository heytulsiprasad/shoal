package bencode

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDecodePrimitives(t *testing.T) {
	cases := []struct {
		in   string
		want any
	}{
		{"i42e", int64(42)},
		{"i-7e", int64(-7)},
		{"i0e", int64(0)},
		{"4:spam", "spam"},
		{"0:", ""},
		{"l4:spami42ee", []any{"spam", int64(42)}},
		{"d3:cow3:moo4:spam4:eggse", map[string]any{"cow": "moo", "spam": "eggs"}},
	}
	for _, c := range cases {
		got, err := Decode([]byte(c.in))
		if err != nil {
			t.Fatalf("Decode(%q) error: %v", c.in, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Decode(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestDecodeRejectsTrailingBytes(t *testing.T) {
	if _, err := Decode([]byte("i42eX")); err == nil {
		t.Fatal("expected error on trailing bytes, got nil")
	}
}

func TestDecodeRejectsMalformed(t *testing.T) {
	bad := []string{"i42", "4:ab", "li1e", "d3:cow", "x", ""}
	for _, in := range bad {
		if _, err := Decode([]byte(in)); err == nil {
			t.Errorf("Decode(%q) expected error, got nil", in)
		}
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	original := map[string]any{
		"announce": "http://tracker.example/announce",
		"info": map[string]any{
			"name":         "file.bin",
			"piece length": int64(16384),
			"length":       int64(100000),
			"pieces":       string(bytes.Repeat([]byte{0xAB}, 40)),
		},
	}
	encoded, err := Encode(original)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Errorf("round trip mismatch:\n got %#v\nwant %#v", decoded, original)
	}
}

func TestEncodeSortsKeys(t *testing.T) {
	// Keys must come out in bytewise sorted order regardless of insertion order.
	got, err := Encode(map[string]any{"b": int64(2), "a": int64(1), "c": int64(3)})
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	want := "d1:ai1e1:bi2e1:ci3ee"
	if string(got) != want {
		t.Errorf("Encode = %q, want %q", got, want)
	}
}

func TestDecodeTopDictCapturesRawInfo(t *testing.T) {
	info := map[string]any{
		"name":         "x",
		"piece length": int64(16384),
		"length":       int64(10),
		"pieces":       string(bytes.Repeat([]byte{0x01}, 20)),
	}
	infoBytes, err := Encode(info)
	if err != nil {
		t.Fatalf("Encode info: %v", err)
	}
	top, err := Encode(map[string]any{"announce": "http://t/a", "info": info})
	if err != nil {
		t.Fatalf("Encode top: %v", err)
	}

	_, raw, err := DecodeTopDict(top)
	if err != nil {
		t.Fatalf("DecodeTopDict: %v", err)
	}
	if !bytes.Equal(raw["info"], infoBytes) {
		t.Errorf("raw info span = %q, want %q", raw["info"], infoBytes)
	}
}
