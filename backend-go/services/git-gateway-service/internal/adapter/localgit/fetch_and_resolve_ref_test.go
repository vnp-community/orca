package localgit

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func gitInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// cloneWithOrigin makes a bare origin holding main and release/x, and a clone of it.
func cloneWithOrigin(t *testing.T) (clone, origin string) {
	t.Helper()
	seed := initRepo(t)
	gitInDir(t, seed, "branch", "release/x")
	gitInDir(t, seed, "tag", "v1.0")
	origin = filepath.Join(t.TempDir(), "origin.git")
	gitInDir(t, seed, "clone", "--bare", seed, origin)
	clone = filepath.Join(t.TempDir(), "clone")
	gitInDir(t, filepath.Dir(clone), "clone", origin, clone)
	return clone, origin
}

func TestFetchAndResolveRef_RemoteBranchAndTag_ResolveToTheirCommit(t *testing.T) {
	clone, _ := cloneWithOrigin(t)
	e := New()
	for ref, rev := range map[string]string{"main": "origin/main", "release/x": "origin/release/x", "v1.0": "v1.0"} {
		want := gitInDir(t, clone, "rev-parse", rev+"^{commit}")
		got, err := e.FetchAndResolveRef(context.Background(), clone, ref)
		if err != nil || got != want {
			t.Errorf("%s: got %q %v, want %q", ref, got, err, want)
		}
	}
}

func TestFetchAndResolveRef_PicksUpANewCommitOnTheRemote(t *testing.T) {
	clone, origin := cloneWithOrigin(t)
	other := filepath.Join(t.TempDir(), "other")
	gitInDir(t, filepath.Dir(other), "clone", origin, other)
	gitInDir(t, other, "config", "user.email", "t@example.com")
	gitInDir(t, other, "config", "user.name", "T")
	gitInDir(t, other, "commit", "--allow-empty", "-m", "newer")
	gitInDir(t, other, "push", "origin", "main")
	want := gitInDir(t, other, "rev-parse", "HEAD")

	got, err := New().FetchAndResolveRef(context.Background(), clone, "main")
	if err != nil || got != want {
		t.Fatalf("a prefetch must see the remote's newer commit: got %q %v, want %q", got, err, want)
	}
}

func TestFetchAndResolveRef_LocalOnlyBranch_FallsBackToTheLocalCommit(t *testing.T) {
	clone, _ := cloneWithOrigin(t)
	gitInDir(t, clone, "branch", "local-only")
	want := gitInDir(t, clone, "rev-parse", "local-only")
	got, err := New().FetchAndResolveRef(context.Background(), clone, "local-only")
	if err != nil || got != want {
		t.Fatalf("got %q %v, want %q", got, err, want)
	}
}

func TestFetchAndResolveRef_RepoWithoutOrigin_UsesLocalBranches(t *testing.T) {
	dir := initRepo(t)
	gitInDir(t, dir, "branch", "release/x")
	want := gitInDir(t, dir, "rev-parse", "release/x")
	got, err := New().FetchAndResolveRef(context.Background(), dir, "release/x")
	if err != nil || got != want {
		t.Fatalf("got %q %v, want %q", got, err, want)
	}
}

func TestFetchAndResolveRef_MissingRef_ErrorIsClassifiedAsMissing(t *testing.T) {
	clone, _ := cloneWithOrigin(t)
	noOrigin := initRepo(t)
	for name, dir := range map[string]string{"with origin": clone, "without origin": noOrigin} {
		_, err := New().FetchAndResolveRef(context.Background(), dir, "release/none")
		if err == nil || !domain.IsRemoteRefMissing(err) {
			t.Errorf("%s: want an error classified as a missing ref, got %v", name, err)
		}
	}
}

func TestFetchAndResolveRef_OriginThatCannotBeRead_IsUnreachableNotMissing(t *testing.T) {
	clone, origin := cloneWithOrigin(t)
	gitInDir(t, clone, "remote", "set-url", "origin", filepath.Join(filepath.Dir(origin), "gone.git"))
	_, err := New().FetchAndResolveRef(context.Background(), clone, "release/none")
	if err == nil || domain.IsRemoteRefMissing(err) || !domain.IsRemoteUnreachable(err) {
		t.Fatalf("a remote that cannot be read says nothing about the ref: %v", err)
	}
}

func TestFetchAndResolveRef_RefThatLooksLikeAnOption_IsRefused(t *testing.T) {
	clone, _ := cloneWithOrigin(t)
	if _, err := New().FetchAndResolveRef(context.Background(), clone, "--upload-pack=touch /tmp/x"); err == nil {
		t.Fatal("an option-looking ref must never reach git")
	}
}
