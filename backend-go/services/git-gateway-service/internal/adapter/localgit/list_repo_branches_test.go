package localgit

import (
	"context"
	"reflect"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// Real git binary and real repos; remotes are faked with `git remote add` + `git update-ref`, no network.

func listRepoBranches(t *testing.T, repo string) ([]domain.RepoBranch, bool) {
	t.Helper()
	branches, truncated, err := New().ListRepoBranches(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return branches, truncated
}

func TestListRepoBranches_LocalSlashedAndRemoteOnly(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "branch", "feature/other")
	gitIn(t, dir, "branch", "release/1.x")
	gitIn(t, dir, "remote", "add", "origin", "https://example.invalid/repo.git")
	gitIn(t, dir, "update-ref", "refs/remotes/origin/hotfix/x", "HEAD")

	got, truncated := listRepoBranches(t, dir)
	want := []domain.RepoBranch{
		{Name: "feature/other"},
		{Name: "main"},
		{Name: "release/1.x"},
		{Name: "origin/hotfix/x", Remote: true},
	}
	if truncated || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v truncated=%v, want %+v", got, truncated, want)
	}
}

func TestListRepoBranches_RemoteWithLocalTwinIsHidden(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "remote", "add", "origin", "https://example.invalid/repo.git")
	gitIn(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitIn(t, dir, "update-ref", "refs/remotes/origin/only-remote", "HEAD")

	got, _ := listRepoBranches(t, dir)
	want := []domain.RepoBranch{{Name: "main"}, {Name: "origin/only-remote", Remote: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestListRepoBranches_RemoteHeadIsDropped(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "remote", "add", "origin", "https://example.invalid/repo.git")
	gitIn(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitIn(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	gitIn(t, dir, "update-ref", "refs/remotes/origin/dev", "HEAD")

	got, _ := listRepoBranches(t, dir)
	for _, b := range got {
		if b.Name == "origin/HEAD" || b.Name == "HEAD" || b.Name == "origin" {
			t.Fatalf("HEAD entry leaked into the list: %+v", got)
		}
	}
	want := []domain.RepoBranch{{Name: "main"}, {Name: "origin/dev", Remote: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestListRepoBranches_DetachedHeadIsNotListed(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "checkout", "--detach")

	got, _ := listRepoBranches(t, dir)
	if want := []domain.RepoBranch{{Name: "main"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestListRepoBranches_NotARepoReturnsError(t *testing.T) {
	if _, _, err := New().ListRepoBranches(context.Background(), t.TempDir()); err == nil {
		t.Fatal("expected an error outside a git repository")
	}
}
