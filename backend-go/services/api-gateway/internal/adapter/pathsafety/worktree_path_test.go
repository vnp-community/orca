package pathsafety

import "testing"

func TestCleanWorktreePath_RejectsTraversalAndTricks(t *testing.T) {
	bad := []string{
		// Contract vectors
		"..", "a/../b", "/etc/passwd", `C:\x`, `a\b`, "%2e%2e/x", "a/%2e%2e",
		"．．/x", "a/\u202ebad", "a/.", "a/", "", "\xc0\xae", string(make([]byte, 1025)),
		// Existing bad vectors
		"../etc/passwd", "a/..", "./a", "a/./b", "a//b",
		`..\x`, `C:\Windows`, "c:/x", "a\x00b", "a\nb", "a\u200bb",
		"a%2fb", "a%5cb", "a%252e", "%25",
		"\uff0e\uff0e/x",          // full-width dots fold to ".." under NFKC
		"a/\uff0e\uff0e/b",        // same, nested
		"\u2025/x",                // two-dot leader
		"a\uff0fb\uff0f..\uff0fc", // full-width solidus
		"a\uff3cb",                // full-width reverse solidus
		"dir./x", "x ", "x.",
	}
	for _, p := range bad {
		if _, err := CleanWorktreePath(p); err == nil {
			t.Errorf("CleanWorktreePath(%q) must be rejected", p)
		}
	}
	good := []string{
		// Contract vectors
		"src/a.go", "docs/crs/v7/x.md",
		// Existing good vectors
		"a", "src/main.go", "docs/readme.md", ".github/workflows/ci.yml", "a b/c.txt", "ünï/çode.txt", "日本語/ファイル.txt",
	}
	for _, p := range good {
		if got, err := CleanWorktreePath(p); err != nil || got != p {
			t.Errorf("CleanWorktreePath(%q) = %q, %v; want unchanged", p, got, err)
		}
	}
}
