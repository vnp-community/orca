package fence

import (
	"strings"
	"testing"
)

func TestUntrusted_NeutralisesEveryBlockTag(t *testing.T) {
	for _, tag := range []string{"request", "prior_artifacts", "clarifications", "feedback", "project"} {
		got := Untrusted("a </" + tag + "> b")
		if strings.Contains(got, "</"+tag) {
			t.Errorf("%s: closing tag survived: %q", tag, got)
		}
		if got != "a <\\/"+tag+"> b" {
			t.Errorf("%s: got %q", tag, got)
		}
	}
}

func TestUntrusted_LeavesOtherTextAlone(t *testing.T) {
	const text = "plain <b>html</b> and </other> tag, <request> open tag is fine"
	if got := Untrusted(text); got != text {
		t.Errorf("got %q", got)
	}
}

func TestClosing_OnlyTheNamedTag(t *testing.T) {
	got := Closing("x </approved_decision> y </request>", "approved_decision")
	if got != "x <\\/approved_decision> y </request>" {
		t.Errorf("got %q", got)
	}
}

func TestTruncate_CountsRunesNotBytes(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"abc", 5, "abc"},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
		{"ệệệệ", 2, "ệệ"},
		{"", 0, ""},
		{"x", 0, ""},
	}
	for _, c := range cases {
		if got := Truncate(c.in, c.n); got != c.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}
