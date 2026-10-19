package localgit

import (
	"context"
	"path/filepath"
	"testing"
)

// Real git binary and real repos, no fakes.

func wantBaseRef(t *testing.T, repo, preferred, want string) {
	t.Helper()
	got, err := New().BaseRefDefault(context.Background(), repo, preferred)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("BaseRefDefault(preferred=%q) = %q, want %q", preferred, got, want)
	}
}

// bareUpstream returns a bare repo whose HEAD is defaultBranch, holding one
// commit on defaultBranch and on every extra branch.
func bareUpstream(t *testing.T, defaultBranch string, extra ...string) string {
	t.Helper()
	upstream := t.TempDir()
	gitIn(t, upstream, "init", "--bare", "-b", defaultBranch)
	seed := initRepo(t)
	gitIn(t, seed, "branch", "-m", defaultBranch)
	gitIn(t, seed, "remote", "add", "origin", upstream)
	gitIn(t, seed, "push", "origin", defaultBranch)
	for _, b := range extra {
		gitIn(t, seed, "branch", b)
		gitIn(t, seed, "push", "origin", b)
	}
	return upstream
}

func TestBaseRefDefault_RealCloneUsesOriginHead(t *testing.T) {
	upstream := bareUpstream(t, "main")
	clone := filepath.Join(t.TempDir(), "clone")
	gitIn(t, t.TempDir(), "clone", upstream, clone)
	wantBaseRef(t, clone, "", "main")
}

func TestBaseRefDefault_SlashedOriginHeadTargetKeepsEverySegment(t *testing.T) {
	upstream := bareUpstream(t, "release/1.x", "feature/other")
	clone := filepath.Join(t.TempDir(), "clone")
	gitIn(t, t.TempDir(), "clone", upstream, clone)
	// Checked-out branch differs from the default: the default must still win.
	gitIn(t, clone, "checkout", "-b", "feature/work")
	wantBaseRef(t, clone, "", "release/1.x")
}

func TestBaseRefDefault_OriginHeadIsNotTheCheckedOutFeatureBranch(t *testing.T) {
	upstream := bareUpstream(t, "main", "feature/other")
	clone := filepath.Join(t.TempDir(), "clone")
	gitIn(t, t.TempDir(), "clone", upstream, clone)
	gitIn(t, clone, "checkout", "feature/other")
	wantBaseRef(t, clone, "", "main")
}

func TestBaseRefDefault_DanglingOriginHeadFallsThroughToARealBranch(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/gone")
	wantBaseRef(t, dir, "", "main")
}

func TestBaseRefDefault_InitPlusRemoteAdd_UsesLsRemoteWithSlashedBranch(t *testing.T) {
	upstream := bareUpstream(t, "release/1.x")
	dir := t.TempDir()
	gitIn(t, dir, "init", "-b", "feature/work")
	gitIn(t, dir, "remote", "add", "origin", upstream)
	wantBaseRef(t, dir, "", "release/1.x")
}

func TestBaseRefDefault_LocalOnly_ConfiguredDefaultWinsOverCheckedOutFeatureBranch(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "checkout", "-b", "feature/other")
	wantBaseRef(t, dir, "main", "main")
}

func TestBaseRefDefault_LocalOnly_SlashedConfiguredDefault(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "branch", "release/1.x")
	gitIn(t, dir, "checkout", "-b", "feature/other")
	wantBaseRef(t, dir, "release/1.x", "release/1.x")
	wantBaseRef(t, dir, "refs/heads/release/1.x", "release/1.x")
}

func TestBaseRefDefault_LocalOnly_ConfiguredDefaultMissingLocally_FallsBackToMain(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "checkout", "-b", "feature/other")
	wantBaseRef(t, dir, "develop", "main")
}

func TestBaseRefDefault_LocalOnly_NoHint_PrefersMainOverCheckedOutBranch(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "checkout", "-b", "feature/other")
	wantBaseRef(t, dir, "", "main")
}

func TestBaseRefDefault_LocalOnly_NoMainButMaster(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "branch", "-m", "master")
	gitIn(t, dir, "checkout", "-b", "feature/other")
	wantBaseRef(t, dir, "", "master")
}

func TestBaseRefDefault_LocalOnly_MainBeatsMaster(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "branch", "master")
	gitIn(t, dir, "checkout", "-b", "feature/other")
	wantBaseRef(t, dir, "", "main")
}

func TestBaseRefDefault_LocalOnly_OptionLikeHintIsIgnored(t *testing.T) {
	dir := initRepo(t)
	wantBaseRef(t, dir, "--upload-pack=x", "main")
}

func TestBaseRefDefault_FreshInitWithoutCommits_UsesCurrentBranch(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-b", "trunk")
	wantBaseRef(t, dir, "", "trunk")
	// A hint that cannot exist yet must not be returned for a repo with no commits.
	wantBaseRef(t, dir, "main", "trunk")
}

func TestBaseRefDefault_FreshInitWithEmptyRemote_UsesCurrentBranch(t *testing.T) {
	upstream := t.TempDir()
	gitIn(t, upstream, "init", "--bare", "-b", "main")
	dir := t.TempDir()
	gitIn(t, dir, "init", "-b", "main")
	gitIn(t, dir, "remote", "add", "origin", upstream)
	wantBaseRef(t, dir, "", "main")
}

func TestBaseRefDefault_DetachedHead_StillFindsMain(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "checkout", "--detach")
	wantBaseRef(t, dir, "", "main")
}

func TestBaseRefDefault_DetachedHeadWithoutAnyCandidate_ReturnsErrorNotPanic(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "checkout", "-b", "feature/only")
	gitIn(t, dir, "branch", "-D", "main")
	gitIn(t, dir, "checkout", "--detach")
	if got, err := New().BaseRefDefault(context.Background(), dir, ""); err == nil {
		t.Fatalf("expected an error, got %q", got)
	}
}

func TestBaseRefDefault_NotAGitRepo_ReturnsError(t *testing.T) {
	if got, err := New().BaseRefDefault(context.Background(), t.TempDir(), "main"); err == nil {
		t.Fatalf("expected an error, got %q", got)
	}
}

func TestBranchFromRef(t *testing.T) {
	cases := map[string]string{
		"refs/heads/main":                 "main",
		"refs/heads/feature/other\n":      "feature/other",
		"refs/remotes/origin/main":        "main",
		"refs/remotes/origin/release/1.x": "release/1.x",
		"refs/remotes/upstream/team/a/b":  "team/a/b",
		"refs/remotes/origin":             "",
		"release/1.x":                     "release/1.x",
		"":                                "",
	}
	for in, want := range cases {
		if got := branchFromRef(in); got != want {
			t.Errorf("branchFromRef(%q) = %q, want %q", in, got, want)
		}
	}
}
