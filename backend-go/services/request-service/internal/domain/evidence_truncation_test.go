package domain

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func excerptOf(t *testing.T, in string) string {
	t.Helper()
	return NewEvidence("id", "tenant", "req", 1, "src", "ref", "title", in, "digest", time.Time{}).Excerpt
}

func TestNewEvidence_ExcerptLimitIsInBytes(t *testing.T) {
	for _, c := range []struct {
		name    string
		in      string
		wantLen int
	}{
		{"empty", "", 0},
		{"short", "ngắn", len("ngắn")},
		{"ascii exactly 4096", strings.Repeat("a", 4096), 4096},
		{"ascii 4097", strings.Repeat("a", 4097), 4096},
		{"two byte letters exactly 4096 bytes", strings.Repeat("ă", 2048), 4096},
	} {
		if got := excerptOf(t, c.in); len(got) != c.wantLen || !strings.HasPrefix(c.in, got) {
			t.Errorf("%s: kept %d bytes, want %d", c.name, len(got), c.wantLen)
		}
	}
}

// The column is CHECK (octet_length(excerpt) <= 4096): the cut must respect bytes yet never leave half a character.
func TestNewEvidence_CutLandsOnARuneBoundary(t *testing.T) {
	for _, letter := range []string{"ă", "ệ", "ợ", "😀"} {
		for pad := 0; pad < 4; pad++ {
			in := strings.Repeat("a", pad) + strings.Repeat(letter, 4096/len(letter)+2)
			got := excerptOf(t, in)
			if !utf8.ValidString(got) {
				t.Errorf("%q pad=%d: the excerpt is not valid UTF-8", letter, pad)
			}
			if len(got) > 4096 {
				t.Errorf("%q pad=%d: %d bytes exceed the column limit", letter, pad, len(got))
			}
			if !strings.HasPrefix(in, got) {
				t.Errorf("%q pad=%d: the excerpt is not a prefix of the input", letter, pad)
			}
			// Nothing is dropped beyond the one character that did not fit.
			if len(got) <= 4096-len(letter) {
				t.Errorf("%q pad=%d: cut too much, kept %d bytes", letter, pad, len(got))
			}
		}
	}
}

func TestNewEvidence_KeepsEveryOtherField(t *testing.T) {
	at := time.Date(2026, 10, 20, 1, 2, 3, 0, time.UTC)
	e := NewEvidence("id", "tenant", "req", 7, "src", "ref", "title", "x", "digest", at)
	if e.ID != "id" || e.TenantID != "tenant" || e.RequestID != "req" || e.Seq != 7 || e.SourceID != "src" || e.SourceRef != "ref" ||
		e.Title != "title" || e.Excerpt != "x" || e.Digest != "digest" || !e.RetrievedAt.Equal(at) {
		t.Fatalf("got %+v", e)
	}
}
