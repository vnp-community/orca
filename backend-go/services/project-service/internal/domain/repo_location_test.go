package domain

import (
	"strings"
	"testing"
)

func TestNormalizeRepoLocation(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"posix plain", "/Users/me/code/app", "/Users/me/code/app"},
		{"posix trailing slash", "/Users/me/code/app/", "/Users/me/code/app"},
		{"posix many trailing slashes", "/Users/me/code/app///", "/Users/me/code/app"},
		{"posix repeated slashes", "/Users//me/code///app", "/Users/me/code/app"},
		{"posix root stays", "/", "/"},
		{"posix outer spaces", "  /srv/repo/ \n", "/srv/repo"},
		{"posix case kept", "/Users/Me/Code", "/Users/Me/Code"},
		{"posix dotdot not resolved", "/a/b/../c", "/a/b/../c"},
		{"posix backslash is a filename char", `/srv/we\ird`, `/srv/we\ird`},
		{"windows backslash", `C:\Users\me\app`, `C:\Users\me\app`},
		{"windows trailing backslash", `C:\Users\me\app\`, `C:\Users\me\app`},
		{"windows forward slashes", `C:/Users/me/app/`, `C:\Users\me\app`},
		{"windows mixed and repeated", `C:\Users//me\\app`, `C:\Users\me\app`},
		{"windows root", `C:\`, `C:\`},
		{"windows root forward", `C:/`, `C:\`},
		{"windows case kept", `c:\Users\ME`, `c:\Users\ME`},
		{"windows UNC", `\\srv\share\x\`, `\\srv\share\x`},
		{"remote url", "https://github.com/org/repo/", "https://github.com/org/repo"},
		{"remote url keeps double slash scheme", "ssh://git@host//org/repo//", "ssh://git@host//org/repo"},
		{"empty", "   ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeRepoLocation(tc.in); got != tc.want {
				t.Errorf("NormalizeRepoLocation(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRepoRegistrationKey_SameLocationSameKey(t *testing.T) {
	a := RepoRegistrationKey("t1", "DEV-1", "/a/b")
	for _, url := range []string{"/a/b/", "/a//b", " /a/b "} {
		if got := RepoRegistrationKey("t1", "dev-1", url); got != a {
			t.Errorf("key for %q differs from /a/b", url)
		}
	}
	if len(a) != 64 {
		t.Errorf("key length = %d, want 64", len(a))
	}
}

func TestRepoRegistrationKey_DifferentWhenAnyPartDiffers(t *testing.T) {
	base := RepoRegistrationKey("t1", "dev-1", "/a/b")
	for name, other := range map[string]string{
		"tenant":     RepoRegistrationKey("t2", "dev-1", "/a/b"),
		"dev server": RepoRegistrationKey("t1", "dev-2", "/a/b"),
		"path":       RepoRegistrationKey("t1", "dev-1", "/a/c"),
		"case":       RepoRegistrationKey("t1", "dev-1", "/A/b"),
		"local":      RepoRegistrationKey("t1", "", "/a/b"),
	} {
		if other == base {
			t.Errorf("key must differ when %s differs", name)
		}
	}
}

func TestRepoRegistrationKey_NoKeyWithoutTenantOrPath(t *testing.T) {
	if k := RepoRegistrationKey("", "dev", "/a"); k != "" {
		t.Errorf("no tenant must give no key, got %q", k)
	}
	if k := RepoRegistrationKey("t", "dev", " / "); k == "" {
		t.Error("root path is a real location and must be keyed")
	}
	if k := RepoRegistrationKey("t", "dev", "  "); k != "" {
		t.Errorf("blank path must give no key, got %q", k)
	}
	if strings.ContainsAny(RepoRegistrationKey("t", "d", "/x"), "GHIJ") {
		t.Error("key must be lower hex")
	}
}
