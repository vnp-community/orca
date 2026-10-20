package domain

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseOmittedMarker(t *testing.T) {
	hash := strings.Repeat("ab", 32)
	cases := []struct {
		name string
		in   string
		size int64
		ok   bool
	}{
		{"valid", fmt.Sprintf("<omitted size=2097152 sha256=%s>", hash), 2097152, true},
		{"zero size", fmt.Sprintf("<omitted size=0 sha256=%s>", hash), 0, true},
		{"overflowing size", fmt.Sprintf("<omitted size=99999999999999999999 sha256=%s>", hash), 0, false},
		{"short hash", "<omitted size=10 sha256=abc>", 0, false},
		{"upper-case hash", fmt.Sprintf("<omitted size=10 sha256=%s>", strings.ToUpper(hash)), 0, false},
		{"negative size", fmt.Sprintf("<omitted size=-5 sha256=%s>", hash), 0, false},
		{"trailing text", fmt.Sprintf("<omitted size=10 sha256=%s> more", hash), 0, false},
		{"plain text", "# tasks", 0, false},
		{"empty", "", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			size, sum, ok := ParseOmittedMarker(c.in)
			if ok != c.ok || size != c.size {
				t.Fatalf("got size=%d ok=%v, want size=%d ok=%v", size, ok, c.size, c.ok)
			}
			if ok && sum != hash {
				t.Fatalf("sha256 = %q", sum)
			}
		})
	}
}
