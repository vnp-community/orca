package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

func TestParseWorktreeBaseRef_Valid(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"empty is the default", "", ""},
		{"simple branch", "main", "main"},
		{"release branch", "release/1.4", "release/1.4"},
		{"remote branch", "origin/release/1.4", "origin/release/1.4"},
		{"tag", "v1.4.0", "v1.4.0"},
		{"full ref name", "refs/heads/main", "refs/heads/main"},
		{"edge whitespace is trimmed", "  main\t\n", "main"},
		{"dots inside a component", "feature/a.b.c", "feature/a.b.c"},
		{"underscore and dash inside", "fix/some_bug-12", "fix/some_bug-12"},
		{"at sign without brace", "user@host", "user@host"},
		{"non-ascii letters", "tính-năng/mới", "tính-năng/mới"},
		{"exactly 255 bytes", strings.Repeat("a", 255), strings.Repeat("a", 255)},
		{"lock inside a component name", "a.lockfile/b", "a.lockfile/b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseWorktreeBaseRef(c.in)
			if err != nil || string(got) != c.want {
				t.Fatalf("ParseWorktreeBaseRef(%q) = %q, %v; want %q", c.in, got, err, c.want)
			}
		})
	}
}

func TestParseWorktreeBaseRef_Invalid(t *testing.T) {
	cases := []struct{ name, in string }{
		{"blank", "   "},
		{"blank tab and newline", "\t\n "},
		{"leading dash", "-x"},
		{"leading double dash", "--upload-pack=evil"},
		{"dot dot", "a..b"},
		{"dot dot at start", "..main"},
		{"reflog syntax", "main@{1}"},
		{"bare at sign", "@"},
		{"backslash", `a\b`},
		{"NUL byte", "a\x00b"},
		{"newline", "a\nb"},
		{"carriage return", "a\rb"},
		{"tab inside", "a\tb"},
		{"DEL", "a\x7fb"},
		{"C1 control", "a\u0085b"},
		{"space inside", "my branch"},
		{"non-breaking space inside", "a b"},
		{"tilde", "main~1"},
		{"caret", "main^"},
		{"colon", "origin:main"},
		{"question mark", "release?"},
		{"asterisk", "release/*"},
		{"open bracket", "a[0]"},
		{"trailing slash", "release/"},
		{"leading slash", "/release"},
		{"double slash", "release//1.4"},
		{"trailing dot", "release."},
		{"trailing lock", "main.lock"},
		{"component ending in lock", "a.lock/b"},
		{"component starting with dot", "a/.hidden"},
		{"leading dot", ".hidden"},
		{"invalid utf-8", "a\xffb"},
		{"256 bytes", strings.Repeat("a", 256)},
		{"256 bytes of multibyte", strings.Repeat("é", 128)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseWorktreeBaseRef(c.in)
			var ae *apperrors.AppError
			if !errors.As(err, &ae) || ae.Code != "REQUEST_BASE_REF_INVALID" || ae.Kind != apperrors.KindInvalidArgument {
				t.Fatalf("ParseWorktreeBaseRef(%q) = %q, %v; want REQUEST_BASE_REF_INVALID", c.in, got, err)
			}
			if got != "" {
				t.Fatalf("invalid input must not yield a value, got %q", got)
			}
		})
	}
}

func TestParseWorktreeBaseRef_ErrorDoesNotEchoTheValue(t *testing.T) {
	_, err := ParseWorktreeBaseRef("secret-token:abc")
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error must name the rule, not echo the value: %v", err)
	}
}

func TestWorktreeBaseRefOrDefault(t *testing.T) {
	if got := (Request{BaseRef: "release/1.4"}).WorktreeBaseRefOrDefault("main"); got != "release/1.4" {
		t.Fatalf("pinned ref must win, got %q", got)
	}
	if got := (Request{}).WorktreeBaseRefOrDefault("main"); got != "main" {
		t.Fatalf("empty ref must use the default, got %q", got)
	}
	if got := (Request{}).WorktreeBaseRefOrDefault(""); got != "" {
		t.Fatalf("no ref and no default stays empty (the caller must treat it as an error, never as HEAD), got %q", got)
	}
}

func TestBaseRefErrors_KindsAndDetails(t *testing.T) {
	nf := ErrRequestBaseRefNotFound("rel/9", []string{"a", "b", "c", "d", "e", "f", "g"})
	var ae *apperrors.AppError
	if !errors.As(nf, &ae) || ae.Code != "REQUEST_BASE_REF_NOT_FOUND" || ae.Kind != apperrors.KindNotFound {
		t.Fatalf("not found: %v", nf)
	}
	d, ok := BaseRefDetails(nf)
	if !ok || d["suggestions"] != "a,b,c,d,e" || d["ref"] != "rel/9" {
		t.Fatalf("suggestions must be capped at %d: %v", MaxBaseRefSuggestions, d)
	}
	lk := ErrRequestBaseRefLocked(BaseRefLockedTask)
	if !errors.As(lk, &ae) || ae.Code != "REQUEST_BASE_REF_LOCKED" || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("locked: %v", lk)
	}
	if d, ok := BaseRefDetails(lk); !ok || d["reason"] != "task" {
		t.Fatalf("lock reason detail: %v", d)
	}
	if _, ok := BaseRefDetails(ErrRequestBaseRefInvalid("x")); ok {
		t.Fatal("invalid carries no details")
	}
}
