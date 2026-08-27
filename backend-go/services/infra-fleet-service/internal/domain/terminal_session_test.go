package domain

import (
	"strings"
	"testing"
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
