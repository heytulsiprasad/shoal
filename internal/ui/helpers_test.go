package ui

import "testing"

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"hello", 1, "h"},
		{"hello", 0, ""},
		{"héllo", 3, "hé…"}, // rune-aware
	}
	for _, c := range cases {
		if got := truncate(c.in, c.n); got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		0:          "0 B",
		512:        "512 B",
		1024:       "1.0 KiB",
		1536:       "1.5 KiB",
		1048576:    "1.0 MiB",
		1073741824: "1.0 GiB",
	}
	for in, want := range cases {
		if got := formatBytes(in); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestAsMagnet(t *testing.T) {
	if got := asMagnet("  magnet:?xt=urn:btih:abc  "); got != "magnet:?xt=urn:btih:abc" {
		t.Errorf("asMagnet(magnet) = %q", got)
	}
	if got := asMagnet("MAGNET:?xt=abc"); got != "MAGNET:?xt=abc" {
		t.Errorf("asMagnet(upper) = %q, want preserved original", got)
	}
	if got := asMagnet("http://example/x"); got != "" {
		t.Errorf("asMagnet(http) = %q, want empty", got)
	}
}

func TestPadOrTrim(t *testing.T) {
	if got := padOrTrim("hi", 5); got != "hi   " {
		t.Errorf("padOrTrim pad = %q, want %q", got, "hi   ")
	}
	if got := padOrTrim("hello world", 5); got != "hell…" {
		t.Errorf("padOrTrim trim = %q, want %q", got, "hell…")
	}
	if got := padOrTrim("x", 0); got != "" {
		t.Errorf("padOrTrim(_, 0) = %q, want empty", got)
	}
}

func TestThousands(t *testing.T) {
	cases := map[int64]string{
		0:       "0",
		42:      "42",
		1234:    "1,234",
		1234567: "1,234,567",
		-5:      "0",
	}
	for in, want := range cases {
		if got := thousands(in); got != want {
			t.Errorf("thousands(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestSizeOrDash(t *testing.T) {
	if got := sizeOrDash(0); got != "—" {
		t.Errorf("sizeOrDash(0) = %q, want —", got)
	}
	if got := sizeOrDash(-1); got != "—" {
		t.Errorf("sizeOrDash(-1) = %q, want —", got)
	}
	if got := sizeOrDash(1024); got != "1.0 KiB" {
		t.Errorf("sizeOrDash(1024) = %q, want 1.0 KiB", got)
	}
}
