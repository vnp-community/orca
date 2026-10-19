package domain

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncatedForMobile(t *testing.T) {
	t.Run("short output passes through unchanged", func(t *testing.T) {
		got := TruncatedForMobile([]byte("hello"))
		if got != "hello" {
			t.Errorf("expected %q, got %q", "hello", got)
		}
	})

	t.Run("caps at exactly 500 chars", func(t *testing.T) {
		got := TruncatedForMobile([]byte(strings.Repeat("a", 600)))
		if len(got) != 500 {
			t.Fatalf("expected len 500, got %d", len(got))
		}
	})

	t.Run("tail-truncated: keeps the most recent bytes, not the oldest", func(t *testing.T) {
		// "old..." (100 chars) followed by 500 chars of "new" — the 500-char
		// cap must keep the tail (the "new" run), dropping the "old" prefix.
		input := strings.Repeat("o", 100) + strings.Repeat("n", 500)
		got := TruncatedForMobile([]byte(input))
		if len(got) != 500 {
			t.Fatalf("expected len 500, got %d", len(got))
		}
		if strings.Contains(got, "o") {
			t.Errorf("expected the oldest bytes to be dropped, got %q", got)
		}
		if got != strings.Repeat("n", 500) {
			t.Error("expected the most recent 500 bytes to be kept verbatim")
		}
	})

	t.Run("exactly 500 chars is not truncated", func(t *testing.T) {
		input := strings.Repeat("x", 500)
		got := TruncatedForMobile([]byte(input))
		if got != input {
			t.Error("expected exactly-500-char input to pass through unchanged")
		}
	})

	t.Run("empty buffer returns empty string", func(t *testing.T) {
		if got := TruncatedForMobile(nil); got != "" {
			t.Errorf("expected empty string for nil input, got %q", got)
		}
	})
}

func TestTruncatedForMobile_AlwaysValidUTF8(t *testing.T) {
	tail := strings.Repeat("a", 497) + "\xe1\xbb\x87" // 500 bytes plus a 3-byte rune: the cut lands inside nothing
	cases := map[string][]byte{
		"short":                    []byte("hello"),
		"cut inside a rune":        append([]byte(strings.Repeat("a", 300)), []byte(strings.Repeat("\xe1\xbb\x87", 200))...), // 900 bytes of 3-byte runes
		"invalid bytes in the end": append([]byte("ok "), 0xff, 0xfe, 'x'),
		"exactly the limit":        []byte(strings.Repeat("b", 500)),
		"tail with a rune":         []byte(tail),
	}
	for name, in := range cases {
		got := TruncatedForMobile(in)
		if !utf8.ValidString(got) {
			t.Errorf("%s: preview is not valid UTF-8: %q", name, got)
		}
		if len(got) > 500+3 {
			t.Errorf("%s: preview is %d bytes", name, len(got))
		}
	}
	if got := TruncatedForMobile([]byte("hello")); got != "hello" {
		t.Errorf("a short preview must be unchanged, got %q", got)
	}
	if got := TruncatedForMobile(append([]byte("ok "), 0xff, 'x')); got != "ok �x" {
		t.Errorf("an invalid byte must become U+FFFD, got %q", got)
	}
}
