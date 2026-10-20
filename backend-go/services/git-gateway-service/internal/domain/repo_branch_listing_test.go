package domain

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestParseRepoBranchRefs(t *testing.T) {
	l := func(n string) RepoBranch { return RepoBranch{Name: n} }
	r := func(n string) RepoBranch { return RepoBranch{Name: n, Remote: true} }

	tests := []struct {
		name string
		out  string
		want []RepoBranch
	}{
		{"empty output", "", nil},
		{"blank and whitespace lines", "\n  \n\trefs/heads/main  \n\n", []RepoBranch{l("main")}},
		{
			"slashed local names sorted by byte order",
			"refs/heads/release/1.x\nrefs/heads/Zeta\nrefs/heads/feature/other\nrefs/heads/main\n",
			[]RepoBranch{l("Zeta"), l("feature/other"), l("main"), l("release/1.x")},
		},
		{
			"remote-only branch keeps remote prefix and slashes",
			"refs/heads/main\nrefs/remotes/origin/main\nrefs/remotes/origin/hotfix/x\n",
			[]RepoBranch{l("main"), r("origin/hotfix/x")},
		},
		{
			"two remotes, locals first then remotes sorted",
			"refs/remotes/upstream/dev\nrefs/remotes/origin/dev\nrefs/heads/main\nrefs/remotes/origin/b\n",
			[]RepoBranch{l("main"), r("origin/b"), r("origin/dev"), r("upstream/dev")},
		},
		{
			"remote with a local twin is hidden on every remote",
			"refs/heads/dev\nrefs/remotes/origin/dev\nrefs/remotes/upstream/dev\nrefs/remotes/origin/other\n",
			[]RepoBranch{l("dev"), r("origin/other")},
		},
		{
			"remote HEAD is dropped",
			"refs/remotes/origin/HEAD\nrefs/remotes/upstream/HEAD\nrefs/remotes/origin/main\n",
			[]RepoBranch{r("origin/main")},
		},
		{
			"branch merely named HEAD below a directory is kept",
			"refs/remotes/origin/feature/HEAD\n",
			[]RepoBranch{r("origin/feature/HEAD")},
		},
		{
			"local branch named like a remote prefix does not collide with the remote branch",
			"refs/heads/origin/x\nrefs/remotes/origin/x\nrefs/remotes/origin/y\n",
			[]RepoBranch{l("origin/x"), r("origin/y")},
		},
		{
			"local branch named origin hides the remote origin/origin twin",
			"refs/heads/origin\nrefs/remotes/origin/origin\n",
			[]RepoBranch{l("origin")},
		},
		{
			"non-branch refs and bare remote dir are ignored",
			"refs/tags/v1\nrefs/remotes/origin\nrefs/stash\n(HEAD detached at abc)\nrefs/heads/main\n",
			[]RepoBranch{l("main")},
		},
		{
			"duplicate lines collapse",
			"refs/heads/main\nrefs/heads/main\nrefs/remotes/o/a\nrefs/remotes/o/a\n",
			[]RepoBranch{l("main"), r("o/a")},
		},
		{"CRLF line endings", "refs/heads/main\r\nrefs/remotes/origin/a\r\n", []RepoBranch{l("main"), r("origin/a")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, truncated := ParseRepoBranchRefs(tc.out)
			if truncated {
				t.Fatalf("truncated = true, want false")
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseRepoBranchRefsCapsAtMax(t *testing.T) {
	build := func(locals, remotes int) string {
		var b strings.Builder
		for i := 0; i < locals; i++ {
			fmt.Fprintf(&b, "refs/heads/l%04d\n", i)
		}
		for i := 0; i < remotes; i++ {
			fmt.Fprintf(&b, "refs/remotes/origin/r%04d\n", i)
		}
		return b.String()
	}

	t.Run("exactly the cap is not truncated", func(t *testing.T) {
		got, truncated := ParseRepoBranchRefs(build(250, 250))
		if truncated || len(got) != MaxRepoBranches {
			t.Fatalf("len=%d truncated=%v, want %d false", len(got), truncated, MaxRepoBranches)
		}
	})
	t.Run("one over the cap is cut and flagged", func(t *testing.T) {
		got, truncated := ParseRepoBranchRefs(build(250, 251))
		if !truncated || len(got) != MaxRepoBranches {
			t.Fatalf("len=%d truncated=%v, want %d true", len(got), truncated, MaxRepoBranches)
		}
		if last := got[len(got)-1]; last.Name != "origin/r0249" || !last.Remote {
			t.Fatalf("last kept = %+v, want origin/r0249 remote", last)
		}
	})
	t.Run("locals survive the cut before remotes", func(t *testing.T) {
		got, truncated := ParseRepoBranchRefs(build(600, 10))
		if !truncated || len(got) != MaxRepoBranches {
			t.Fatalf("len=%d truncated=%v", len(got), truncated)
		}
		for _, b := range got {
			if b.Remote {
				t.Fatalf("remote branch %q kept while locals were cut", b.Name)
			}
		}
	})
}
