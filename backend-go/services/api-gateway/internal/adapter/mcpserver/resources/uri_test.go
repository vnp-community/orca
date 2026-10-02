package resources

import (
	"strings"
	"testing"
)

const (
	idA = "11111111-1111-4111-8111-111111111111"
	idB = "22222222-2222-4222-8222-222222222222"
)

func TestParse_Valid(t *testing.T) {
	tests := []struct {
		uri  string
		want Ref
	}{
		{"orca://projects", Ref{Kind: KindProjects}},
		{"orca://project/" + idA, Ref{Kind: KindProject, ID: idA}},
		{"orca://project/" + strings.ToUpper(idA), Ref{Kind: KindProject, ID: idA}},
		{"orca://task/" + idA, Ref{Kind: KindTask, ID: idA}},
		{"orca://worktree/" + idA + "/status", Ref{Kind: KindWorktreeStatus, ID: idA}},
		{"orca://worktree/" + idA + "/diff?path=src%2Fmain.go", Ref{Kind: KindWorktreeDiff, ID: idA, Path: "src/main.go"}},
		{"orca://worktree/" + idA + "/diff?path=a.go&base=origin/main&staged=true", Ref{Kind: KindWorktreeDiff, ID: idA, Path: "a.go", Base: "origin/main", Staged: "true"}},
		{"orca://worktree/" + idA + "/file/src/main.go", Ref{Kind: KindWorktreeFile, ID: idA, Path: "src/main.go"}},
		{"orca://worktree/" + idA + "/file/src/m%C3%A9.go", Ref{Kind: KindWorktreeFile, ID: idA, Path: "src/mé.go"}},
		{"orca://worktree/" + idA + "/file/a.txt?offset=10&length=20", Ref{Kind: KindWorktreeFile, ID: idA, Path: "a.txt", Offset: 10, Length: 20}},
		{"orca://review/github/o%2Fr/12", Ref{Kind: KindReview, Provider: "github", Repo: "o/r", Number: "12"}},
		{"orca://review/gitlab/org%2Fsub%2Frepo/7", Ref{Kind: KindReview, Provider: "gitlab", Repo: "org/sub/repo", Number: "7"}},
	}
	for _, tc := range tests {
		got, err := Parse(tc.uri)
		if err != nil {
			t.Errorf("Parse(%q): %v", tc.uri, err)
			continue
		}
		if got.Kind != tc.want.Kind || got.ID != tc.want.ID || got.Path != tc.want.Path || got.Provider != tc.want.Provider ||
			got.Repo != tc.want.Repo || got.Number != tc.want.Number || got.Base != tc.want.Base || got.Staged != tc.want.Staged ||
			got.Offset != tc.want.Offset || got.Length != tc.want.Length {
			t.Errorf("Parse(%q) = %+v, want %+v", tc.uri, got, tc.want)
		}
		if got.RawURI != tc.uri {
			t.Errorf("RawURI altered: %q", got.RawURI)
		}
	}
}

func TestParse_Hostile(t *testing.T) {
	wt := "orca://worktree/" + idA + "/file/"
	bad := []string{
		"", "orca://", "ORCA://projects", "Orca://projects", "http://projects", "file:///etc/passwd", "orca:/projects", "orca:projects",
		"orca://unknown", "orca://projects/extra", "orca://project", "orca://project/", "orca://project/not-a-uuid",
		"orca://project/" + idA + "/", "orca://project/" + idA + "/x", "orca://project/" + idA + "?x=1", "orca://projects?x=1", "orca://projects?",
		"orca://project/" + idA + "#frag", "orca://user@project/" + idA, "orca://project/%31" + idA[1:], "orca://project/" + idA + "%00",
		"orca://project/ " + idA, "orca://project/" + idA + "\n", "orca://project/" + idA + "\u00e9", "orca://pro\u0307ject/" + idA,
		// traversal and encoding tricks in file paths
		wt + "..", wt + "../x", wt + "a/../b", wt + "%2e%2e/x", wt + "%2E%2E%2Fx", wt + "..%2fx", wt + "a%2f..%2fb", wt + "%252e%252e/x",
		wt + "%2e", wt + "./a", wt + "a/./b", wt + "a//b", wt + "/etc/passwd", wt + "%2Fetc%2Fpasswd", wt + `a%5Cb`, wt + `..%5Cx`,
		wt + "a%00b", wt + "a%0ab", wt + "a%0db", wt + "%ef%bc%8e%ef%bc%8e/x", // full-width ".."
		wt + "%e2%80%a5/x", // two-dot leader
		wt + "%ef%bc%8f", wt + "C:/x", wt + "C%3A%5Cx", wt + "%c0%ae%c0%ae/x", wt + "%ff", wt + "a%", wt + "a%zz",
		wt + "dir./x", wt + "x%20", wt + "x.",
		wt + "a?offset=-1", wt + "a?offset=x", wt + "a?length=0", wt + "a?length=999999", wt + "a?bogus=1", wt + "a?offset=1&offset=2",
		"orca://worktree/" + idA + "/file", "orca://worktree/" + idA + "/file/",
		"orca://worktree/" + idA + "/status?x=1", "orca://worktree/" + idA + "/other",
		// diff needs a safe path; query is validated like a file path
		"orca://worktree/" + idA + "/diff", "orca://worktree/" + idA + "/diff?path=..%2fx", "orca://worktree/" + idA + "/diff?path=%2Fetc",
		"orca://worktree/" + idA + "/diff?path=a&base=--upload-pack=x", "orca://worktree/" + idA + "/diff?path=a&base=a..b",
		"orca://worktree/" + idA + "/diff?path=a&staged=yes", "orca://worktree/" + idA + "/diff?path=a&path=b",
		// review
		"orca://review/bitbucket/o%2Fr/1", "orca://review/github/o%2Fr/x", "orca://review/github/o%2Fr/1234567890", "orca://review/github/o/r/1",
		"orca://review/github/or/1", "orca://review/github/o%2F..%2Fr/1", "orca://review/github/o%252Fr/1", "orca://review/github/o%2Fr%2Fx/1",
		"orca://review/gitlab/solo/1", "orca://review/github/o%2F%2Fr/1", "orca://review/github/o%2Fr%00/1",
		"orca://" + strings.Repeat("a", 3000),
	}
	for _, u := range bad {
		if r, err := Parse(u); err == nil {
			t.Errorf("Parse(%q) must fail, got %+v", u, r)
		}
	}
}

func TestParse_PathIsDecodedExactlyOnce(t *testing.T) {
	// %2541 decodes once to the literal "%41": a name, not "A", and the
	// double-encoding guard refuses it so nothing downstream decodes twice.
	if _, err := Parse("orca://worktree/" + idA + "/file/%2541"); err == nil {
		t.Fatal("residual percent-encoding must be rejected")
	}
	r, err := Parse("orca://worktree/" + idA + "/file/a%20b.txt")
	if err != nil || r.Path != "a b.txt" {
		t.Fatalf("%+v %v", r, err)
	}
}
