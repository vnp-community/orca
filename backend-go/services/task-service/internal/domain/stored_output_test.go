package domain

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBoundStoredOutput(t *testing.T) {
	if got := BoundStoredOutput("ok", 10); got != "ok" {
		t.Fatalf("short text is kept: %q", got)
	}
	long := strings.Repeat("ế", 5000) // 3 bytes each: a cut at 8192 lands inside a character
	got := BoundStoredOutput(long, 8192)
	if len(got) > 8192 || !utf8.ValidString(got) || len(got) < 8192-3 {
		t.Fatalf("cut on a character boundary near the limit: %d bytes, valid=%v", len(got), utf8.ValidString(got))
	}
	if got := BoundStoredOutput("a\x00b\xffc", 10); got != "abc" {
		t.Fatalf("NUL and invalid bytes are dropped: %q", got)
	}
}
