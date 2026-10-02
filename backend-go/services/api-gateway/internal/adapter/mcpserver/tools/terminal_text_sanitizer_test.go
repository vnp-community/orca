package tools

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeTerminal_Table(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"plain", "hello\nworld", "hello\nworld"},
		{"csi color", "\x1b[31mred\x1b[0m ok", "red ok"},
		{"csi cursor", "a\x1b[2K\x1b[1;1Hb", "ab"},
		{"osc title bel", "\x1b]0;evil title\x07after", "after"},
		{"osc st", "\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\", "link"},
		{"osc52 clipboard", "x\x1b]52;c;SGVsbG8=\x07y", "xy"},
		{"osc133 prompt mark", "\x1b]133;A\x07$ ", "$ "},
		{"dcs", "a\x1bPq#0;2;0;0;0\x1b\\b", "ab"},
		{"apc", "a\x1b_secret\x1b\\b", "ab"},
		{"charset", "a\x1b(Bb", "ab"},
		{"carriage return progress", "10%\r50%\r100%\ndone", "100%\ndone"},
		{"crlf", "a\r\nb", "a\nb"},
		{"overwrite shorter keeps tail", "abcdef\rXY", "XYcdef"},
		{"backspace", "abc\b\bX", "aXc"},
		{"bell and nul dropped", "a\x07b\x00c", "abc"},
		{"tab kept", "a\tb", "a\tb"},
		{"c1 csi 8-bit", "a\u009b31mb", "ab"},
		{"invalid utf8", "a\xffb", "a�b"},
	}
	for _, c := range cases {
		got, n := sanitizeTerminal([]byte(c.in), true)
		if got != c.want || n != len(c.in) {
			t.Errorf("%s: got %q consumed %d/%d, want %q", c.name, got, n, len(c.in), c.want)
		}
	}
}

func TestSanitizeTerminal_HoldsBackIncompleteSequenceUntilNextRead(t *testing.T) {
	in := "ok\x1b]52;c;SGVs"
	got, n := sanitizeTerminal([]byte(in), false)
	if got != "ok" || n != 2 {
		t.Fatalf("got %q consumed %d", got, n)
	}
	got, n = sanitizeTerminal([]byte(in[n:]+"bG8=\x07z"), false)
	if got != "z" {
		t.Fatalf("completed sequence leaked: %q", got)
	}
	_ = n
	got, n = sanitizeTerminal([]byte("a\x1b["), true)
	if got != "a" || n != 3 {
		t.Fatalf("final read drops the dangling sequence: %q %d", got, n)
	}
}

func TestSanitizeTerminal_LongLineIsCapped(t *testing.T) {
	in := strings.Repeat("x", 100000) + "\nnext"
	got, _ := sanitizeTerminal([]byte(in), true)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 || lines[1] != "next" {
		t.Fatalf("structure lost: %d lines", len(lines))
	}
	if n := utf8.RuneCountInString(lines[0]); n > maxSanitizedLine+len([]rune(lineTruncMarker)) || !strings.HasSuffix(lines[0], lineTruncMarker) {
		t.Fatalf("line not capped: %d runes", n)
	}
}

func TestSanitizeTerminal_NoEscapeSurvivesAnywhere(t *testing.T) {
	in := "\x1b[?2004h\x1b]0;t\x07$ \x1b[1m\x1b[7m%\x1b[27m\x1b[0m\r\x1b[K"
	got, _ := sanitizeTerminal([]byte(in), true)
	if strings.ContainsRune(got, 0x1b) || strings.ContainsRune(got, 0x07) {
		t.Fatalf("control characters leaked: %q", got)
	}
}

func FuzzSanitizeTerminal(f *testing.F) {
	for _, s := range []string{"\x1b[", "\x1b]52;c;", "a\rb", "\x90", "\xe2\x98", "\x1b(", "\x1bP"} {
		f.Add([]byte(s), true)
	}
	f.Fuzz(func(t *testing.T, in []byte, final bool) {
		got, n := sanitizeTerminal(in, final)
		if n < 0 || n > len(in) {
			t.Fatalf("consumed %d of %d", n, len(in))
		}
		if !utf8.ValidString(got) {
			t.Fatalf("invalid utf8 output %q", got)
		}
		for _, r := range got {
			if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
				t.Fatalf("control rune %U in %q (input %q)", r, got, in)
			}
		}
	})
}
