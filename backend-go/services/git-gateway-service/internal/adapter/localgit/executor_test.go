package localgit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// initRepo creates a real git repository in a temp dir, with an initial
// commit, and returns its path — these tests exercise the real `git`
// binary (no fakes), per this service's build task: "implement a REAL
// local git executor using os/exec... genuinely simple and testable".
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("writing seed file: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")

	return dir
}

func TestGetStatus_CleanRepo(t *testing.T) {
	dir := initRepo(t)
	e := New()

	status, err := e.GetStatus(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Branch != "main" {
		t.Errorf("expected branch=main, got %q", status.Branch)
	}
	if len(status.Files) != 0 {
		t.Errorf("expected no file changes in a clean repo, got %+v", status.Files)
	}
}

func TestGetStatus_ReportsModifiedAddedUntracked(t *testing.T) {
	dir := initRepo(t)
	e := New()

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("modifying seed file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("writing new file: %v", err)
	}

	status, err := e.GetStatus(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	states := map[string]domain.FileState{}
	for _, f := range status.Files {
		states[f.Path] = f.State
	}
	if states["README.md"] != domain.FileStateModified {
		t.Errorf("expected README.md=modified, got %+v", states)
	}
	if states["new.txt"] != domain.FileStateUntracked {
		t.Errorf("expected new.txt=untracked, got %+v", states)
	}
}

func TestGetDiff_UnstagedAndStaged(t *testing.T) {
	dir := initRepo(t)
	e := New()

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed content\n"), 0o644); err != nil {
		t.Fatalf("modifying seed file: %v", err)
	}

	unstaged, err := e.GetDiff(context.Background(), dir, "README.md", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(unstaged.UnifiedDiff, "changed content") {
		t.Errorf("expected unstaged diff to contain the change, got: %s", unstaged.UnifiedDiff)
	}

	staged, err := e.GetDiff(context.Background(), dir, "README.md", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if staged.UnifiedDiff != "" {
		t.Errorf("expected empty staged diff before `git add`, got: %s", staged.UnifiedDiff)
	}
}

func TestCommit_StagesAndCommits(t *testing.T) {
	dir := initRepo(t)
	e := New()

	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("writing new file: %v", err)
	}

	result, err := e.Commit(context.Background(), dir, "add new.txt", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CommitSHA == "" {
		t.Error("expected a non-empty commit SHA")
	}

	status, err := e.GetStatus(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(status.Files) != 0 {
		t.Errorf("expected clean status after commit, got %+v", status.Files)
	}
}

func TestCommit_EmptyMessage_ReturnsError(t *testing.T) {
	dir := initRepo(t)
	e := New()

	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("writing new file: %v", err)
	}

	if _, err := e.Commit(context.Background(), dir, "", nil); err == nil {
		t.Fatal("expected error for empty commit message (git rejects an empty commit message)")
	}
}

func TestPull_FastForward(t *testing.T) {
	remoteDir := initRepo(t)
	e := New()

	cloneDir := t.TempDir()
	cmd := exec.Command("git", "clone", remoteDir, cloneDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v\n%s", err, out)
	}

	// Advance the remote past the clone so Pull has something to fetch.
	if err := os.WriteFile(filepath.Join(remoteDir, "second.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatalf("writing file in remote: %v", err)
	}
	if _, err := e.Commit(context.Background(), remoteDir, "second commit", nil); err != nil {
		t.Fatalf("committing in remote: %v", err)
	}

	result, err := e.Pull(context.Background(), cloneDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success || result.HadConflicts {
		t.Errorf("expected a clean fast-forward pull, got %+v", result)
	}
	if _, err := os.Stat(filepath.Join(cloneDir, "second.txt")); err != nil {
		t.Errorf("expected second.txt to exist after pull: %v", err)
	}
}

func TestStageUnstage_RoundTrips(t *testing.T) {
	dir := initRepo(t)
	e := New()

	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("writing new file: %v", err)
	}

	if _, err := e.Stage(context.Background(), dir, []string{"new.txt"}); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	status, err := e.GetStatus(context.Background(), dir)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if len(status.Files) != 1 || status.Files[0].State != domain.FileStateAdded {
		t.Errorf("expected new.txt staged as added, got %+v", status.Files)
	}

	if _, err := e.Unstage(context.Background(), dir, []string{"new.txt"}); err != nil {
		t.Fatalf("Unstage: %v", err)
	}
	status, err = e.GetStatus(context.Background(), dir)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if len(status.Files) != 1 || status.Files[0].State != domain.FileStateUntracked {
		t.Errorf("expected new.txt back to untracked after unstage, got %+v", status.Files)
	}
}

func TestHistory_ReturnsCommitsNewestFirst(t *testing.T) {
	dir := initRepo(t)
	e := New()

	if err := os.WriteFile(filepath.Join(dir, "second.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatalf("writing second file: %v", err)
	}
	if _, err := e.Commit(context.Background(), dir, "second commit", nil); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	commits, err := e.History(context.Background(), dir, "", 0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d: %+v", len(commits), commits)
	}
	if commits[0].Message != "second commit" {
		t.Errorf("expected newest commit first, got %+v", commits[0])
	}
	if len(commits[1].ParentSHAs) != 0 {
		t.Errorf("expected the initial commit to have no parents, got %+v", commits[1])
	}
}

func TestHistory_Limit(t *testing.T) {
	dir := initRepo(t)
	e := New()
	if err := os.WriteFile(filepath.Join(dir, "second.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := e.Commit(context.Background(), dir, "second commit", nil); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	commits, err := e.History(context.Background(), dir, "", 1)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(commits) != 1 {
		t.Errorf("expected 1 commit with limit=1, got %d", len(commits))
	}
}

func TestCheckIgnored_ReturnsOnlyIgnoredSubset(t *testing.T) {
	dir := initRepo(t)
	e := New()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatalf("writing .gitignore: %v", err)
	}
	if _, err := e.Commit(context.Background(), dir, "add gitignore", nil); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	got, err := e.CheckIgnored(context.Background(), dir, []string{"debug.log", "README.md"})
	if err != nil {
		t.Fatalf("CheckIgnored: %v", err)
	}
	if len(got) != 1 || got[0] != "debug.log" {
		t.Errorf("expected only debug.log in the ignored subset, got %+v", got)
	}
}

func TestUpstreamStatus_NoUpstream_ReturnsHasUpstreamFalse(t *testing.T) {
	dir := initRepo(t)
	e := New()

	got, err := e.UpstreamStatus(context.Background(), dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.HasUpstream {
		t.Errorf("expected HasUpstream=false for a repo with no configured upstream, got %+v", got)
	}
}

func TestForkSync_ComparesAgainstExpectedUpstream(t *testing.T) {
	remoteDir := initRepo(t)
	cloneDir := t.TempDir()
	cmd := exec.Command("git", "clone", remoteDir, cloneDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v\n%s", err, out)
	}
	e := New()

	// Advance the clone by one local commit (ahead), leave the remote alone.
	if err := os.WriteFile(filepath.Join(cloneDir, "local.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := e.Commit(context.Background(), cloneDir, "local-only commit", nil); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	got, err := e.ForkSync(context.Background(), cloneDir, "origin/main")
	if err != nil {
		t.Fatalf("ForkSync: %v", err)
	}
	if got.Ahead != 1 || got.Behind != 0 || got.Diverged {
		t.Errorf("expected ahead=1,behind=0,diverged=false, got %+v", got)
	}
}

func TestRemoteCommitURL_GitHubHTTPS(t *testing.T) {
	dir := initRepo(t)
	e := New()
	cmd := exec.Command("git", "remote", "add", "origin", "https://github.com/example/repo.git")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("adding remote: %v\n%s", err, out)
	}

	got, err := e.RemoteCommitURL(context.Background(), dir, "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://github.com/example/repo/commit/abc123"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRemoteFileURL_GitHubSSH(t *testing.T) {
	dir := initRepo(t)
	e := New()
	cmd := exec.Command("git", "remote", "add", "origin", "git@github.com:example/repo.git")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("adding remote: %v\n%s", err, out)
	}

	got, err := e.RemoteFileURL(context.Background(), dir, "src/a.go", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://github.com/example/repo/blob/main/src/a.go"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRemoteFileURL_Bitbucket(t *testing.T) {
	dir := initRepo(t)
	e := New()
	cmd := exec.Command("git", "remote", "add", "origin", "https://bitbucket.org/example/repo.git")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("adding remote: %v\n%s", err, out)
	}

	got, err := e.RemoteFileURL(context.Background(), dir, "src/a.go", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://bitbucket.org/example/repo/src/main/src/a.go"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// ── Group A — branch/ref operations (TASK-207) ─────────────────────────────

// runGit runs `git <args...>` in dir, failing the test on error — shared
// setup helper for the tests below (initRepo's own inline `run` closure
// stays local to that function; this is the equivalent for the tests that
// follow).
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func TestCheckout_SwitchesToExistingBranch(t *testing.T) {
	dir := initRepo(t)
	runGit(t, dir, "branch", "feature")
	e := New()

	result, err := e.Checkout(context.Background(), dir, "feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success || result.Branch != "feature" {
		t.Errorf("unexpected checkout result: %+v", result)
	}
}

func TestCheckout_NonexistentBranch_ReturnsError(t *testing.T) {
	dir := initRepo(t)
	e := New()

	if _, err := e.Checkout(context.Background(), dir, "does-not-exist"); err == nil {
		t.Fatal("expected error checking out a nonexistent branch")
	}
}

func TestListLocalBranches_ListsAllWithCurrentMarked(t *testing.T) {
	dir := initRepo(t)
	runGit(t, dir, "branch", "feature")
	e := New()

	branches, err := e.ListLocalBranches(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byName := map[string]domain.BranchInfo{}
	for _, b := range branches {
		byName[b.Name] = b
	}
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %+v", branches)
	}
	if !byName["main"].IsCurrent {
		t.Errorf("expected main to be marked current, got %+v", byName["main"])
	}
	if byName["feature"].IsCurrent {
		t.Errorf("expected feature NOT to be marked current, got %+v", byName["feature"])
	}
}

func TestFastForward_NilPushTarget_UsesConfiguredUpstream(t *testing.T) {
	remoteDir := initRepo(t)
	e := New()

	cloneDir := t.TempDir()
	if out, err := exec.Command("git", "clone", remoteDir, cloneDir).CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(remoteDir, "upstream.txt"), []byte("upstream\n"), 0o644); err != nil {
		t.Fatalf("writing file in remote: %v", err)
	}
	if _, err := e.Commit(context.Background(), remoteDir, "upstream commit", nil); err != nil {
		t.Fatalf("committing in remote: %v", err)
	}

	result, err := e.FastForward(context.Background(), cloneDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected a successful fast-forward, got %+v", result)
	}
	if _, err := os.Stat(filepath.Join(cloneDir, "upstream.txt")); err != nil {
		t.Errorf("expected upstream.txt to exist after fast-forward: %v", err)
	}
}

func TestFastForward_ExplicitPushTarget(t *testing.T) {
	remoteDir := initRepo(t)
	e := New()

	cloneDir := t.TempDir()
	if out, err := exec.Command("git", "clone", remoteDir, cloneDir).CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(remoteDir, "upstream2.txt"), []byte("upstream2\n"), 0o644); err != nil {
		t.Fatalf("writing file in remote: %v", err)
	}
	if _, err := e.Commit(context.Background(), remoteDir, "second upstream commit", nil); err != nil {
		t.Fatalf("committing in remote: %v", err)
	}

	result, err := e.FastForward(context.Background(), cloneDir, &domain.PushTargetInput{RemoteName: "origin", BranchName: "main"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected a successful fast-forward, got %+v", result)
	}
	if _, err := os.Stat(filepath.Join(cloneDir, "upstream2.txt")); err != nil {
		t.Errorf("expected upstream2.txt to exist after fast-forward: %v", err)
	}
}

// createDivergentBranches sets up a repo with "main" and "feature" branches
// that each add a distinct file — a clean rebase (no conflict) scenario.
func createDivergentBranches(t *testing.T) string {
	t.Helper()
	dir := initRepo(t)
	runGit(t, dir, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatalf("writing feature.txt: %v", err)
	}
	runGit(t, dir, "add", "feature.txt")
	runGit(t, dir, "commit", "-m", "add feature.txt")
	runGit(t, dir, "checkout", "main")
	if err := os.WriteFile(filepath.Join(dir, "main.txt"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("writing main.txt: %v", err)
	}
	runGit(t, dir, "add", "main.txt")
	runGit(t, dir, "commit", "-m", "add main.txt")
	runGit(t, dir, "checkout", "feature")
	return dir
}

func TestRebaseFromBase_CleanRebase_Succeeds(t *testing.T) {
	dir := createDivergentBranches(t)
	e := New()

	result, err := e.RebaseFromBase(context.Background(), dir, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success || result.HadConflicts {
		t.Errorf("expected a clean rebase, got %+v", result)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.txt")); err != nil {
		t.Errorf("expected main.txt to be present after rebase: %v", err)
	}
}

// createConflictingBranches sets up a repo with "main" and "feature"
// branches that both modify README.md, on "feature" currently checked out —
// rebasing/merging one into the other conflicts.
func createConflictingBranches(t *testing.T) string {
	t.Helper()
	dir := initRepo(t)
	runGit(t, dir, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("feature change\n"), 0o644); err != nil {
		t.Fatalf("writing README.md: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "feature change")
	runGit(t, dir, "checkout", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("main change\n"), 0o644); err != nil {
		t.Fatalf("writing README.md: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "main change")
	runGit(t, dir, "checkout", "feature")
	return dir
}

func TestRebaseFromBase_Conflict_ThenAbortRebase(t *testing.T) {
	dir := createConflictingBranches(t)
	e := New()

	result, err := e.RebaseFromBase(context.Background(), dir, "main")
	if err != nil {
		t.Fatalf("unexpected error (a conflict is a domain outcome, not a Go error): %v", err)
	}
	if result.Success || !result.HadConflicts {
		t.Errorf("expected a conflicted rebase, got %+v", result)
	}

	op, err := e.ConflictOperation(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op != "rebase" {
		t.Errorf("expected detector to report \"rebase\" mid-conflict, got %q", op)
	}

	abortResult, err := e.AbortRebase(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !abortResult.Success {
		t.Errorf("expected a successful abort, got %+v", abortResult)
	}

	op, err = e.ConflictOperation(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op != "unknown" {
		t.Errorf("expected detector to report \"unknown\" after abort, got %q", op)
	}
}

func TestConflictOperation_DetectsMergeInProgress_ThenAbortMerge(t *testing.T) {
	dir := createConflictingBranches(t)
	runGit(t, dir, "checkout", "main")
	// Merge (not rebase) conflicts — exits nonzero, which is expected here.
	cmd := exec.Command("git", "merge", "feature")
	cmd.Dir = dir
	_ = cmd.Run() // expected to fail with a conflict; asserted via the detector below

	e := New()
	op, err := e.ConflictOperation(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op != "merge" {
		t.Errorf("expected detector to report \"merge\" mid-conflict, got %q", op)
	}

	abortResult, err := e.AbortMerge(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !abortResult.Success {
		t.Errorf("expected a successful abort, got %+v", abortResult)
	}

	op, err = e.ConflictOperation(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op != "unknown" {
		t.Errorf("expected detector to report \"unknown\" after abort, got %q", op)
	}
}

func TestConflictOperation_CleanRepo_ReturnsUnknown(t *testing.T) {
	dir := initRepo(t)
	e := New()

	op, err := e.ConflictOperation(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op != "unknown" {
		t.Errorf("expected \"unknown\" for a clean repo, got %q", op)
	}
}

func TestResolveConflict_Ours_KeepsCurrentBranchVersion(t *testing.T) {
	dir := createConflictingBranches(t)
	runGit(t, dir, "checkout", "main")
	cmd := exec.Command("git", "merge", "feature")
	cmd.Dir = dir
	_ = cmd.Run() // expected to conflict

	e := New()
	result, err := e.ResolveConflict(context.Background(), dir, "README.md", "ours")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got %+v", result)
	}
	content, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	if string(content) != "main change\n" {
		t.Errorf("expected \"ours\" (main change) to win, got %q", content)
	}
	if strings.Contains(runGit(t, dir, "status", "--porcelain"), "UU") {
		t.Error("expected README.md to no longer be unmerged after resolving")
	}
}

func TestResolveConflict_Theirs_KeepsIncomingVersion(t *testing.T) {
	dir := createConflictingBranches(t)
	runGit(t, dir, "checkout", "main")
	cmd := exec.Command("git", "merge", "feature")
	cmd.Dir = dir
	_ = cmd.Run() // expected to conflict

	e := New()
	result, err := e.ResolveConflict(context.Background(), dir, "README.md", "theirs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got %+v", result)
	}
	content, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	if string(content) != "feature change\n" {
		t.Errorf("expected \"theirs\" (feature change) to win, got %q", content)
	}
}

func TestResolveConflict_MarkResolved_StagesWithoutChangingWorktree(t *testing.T) {
	dir := createConflictingBranches(t)
	runGit(t, dir, "checkout", "main")
	cmd := exec.Command("git", "merge", "feature")
	cmd.Dir = dir
	_ = cmd.Run() // expected to conflict

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("resolved by hand\n"), 0o644); err != nil {
		t.Fatalf("writing README.md: %v", err)
	}

	e := New()
	result, err := e.ResolveConflict(context.Background(), dir, "README.md", "markResolved")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got %+v", result)
	}
	content, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	if string(content) != "resolved by hand\n" {
		t.Errorf("expected the hand-edited content to survive markResolved, got %q", content)
	}
	if strings.Contains(runGit(t, dir, "status", "--porcelain"), "UU") {
		t.Error("expected README.md to no longer be unmerged after markResolved")
	}
}

func TestResolveConflict_UnknownOperation_ReturnsError(t *testing.T) {
	dir := createConflictingBranches(t)
	runGit(t, dir, "checkout", "main")
	cmd := exec.Command("git", "merge", "feature")
	cmd.Dir = dir
	_ = cmd.Run() // expected to conflict

	e := New()
	if _, err := e.ResolveConflict(context.Background(), dir, "README.md", "bogus"); err == nil {
		t.Fatal("expected error for an unknown conflict operation")
	}
}

func TestDiscard_TrackedFile_RestoresFromHEAD(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("modifying README.md: %v", err)
	}
	e := New()

	result, err := e.Discard(context.Background(), dir, "README.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got %+v", result)
	}
	content, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	if string(content) != "hello\n" {
		t.Errorf("expected README.md to be restored to its committed content, got %q", content)
	}
}

func TestDiscard_UntrackedFile_RemovesIt(t *testing.T) {
	dir := initRepo(t)
	untrackedPath := filepath.Join(dir, "untracked.txt")
	if err := os.WriteFile(untrackedPath, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("writing untracked.txt: %v", err)
	}
	e := New()

	result, err := e.Discard(context.Background(), dir, "untracked.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got %+v", result)
	}
	if _, err := os.Stat(untrackedPath); !os.IsNotExist(err) {
		t.Errorf("expected untracked.txt to be removed, stat err = %v", err)
	}
}

func TestBulkDiscard_MixedTrackedAndUntracked_AllSucceed(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("modifying README.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("writing untracked.txt: %v", err)
	}
	e := New()

	result, err := e.BulkDiscard(context.Background(), dir, []string{"README.md", "untracked.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success || len(result.FailedPaths) != 0 {
		t.Errorf("expected all paths to succeed, got %+v", result)
	}
	if _, err := os.Stat(filepath.Join(dir, "untracked.txt")); !os.IsNotExist(err) {
		t.Errorf("expected untracked.txt to be removed, stat err = %v", err)
	}
}

func TestBulkDiscard_NonexistentPath_ReportsPartialFailure(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("modifying README.md: %v", err)
	}
	e := New()

	result, err := e.BulkDiscard(context.Background(), dir, []string{"README.md", "does/not/exist.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Errorf("expected partial failure (Success=false), got %+v", result)
	}
	if len(result.FailedPaths) != 1 || result.FailedPaths[0] != "does/not/exist.txt" {
		t.Errorf("expected does/not/exist.txt to be reported as failed, got %+v", result.FailedPaths)
	}
}

// ── TASK-209 real shape redesign: CommitCompare/BranchCompare/CommitDiff/
// BranchDiff/SubmoduleStatus + TASK-210's Fetch, against a real git binary ─

func revParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse %s failed: %v\n%s", ref, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestCommitCompare_DiffsAgainstParent(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("new file\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	e := New()
	if _, err := e.Commit(context.Background(), dir, "add file2", nil); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	commitOID := revParse(t, dir, "HEAD")

	got, err := e.CommitCompare(context.Background(), dir, commitOID)
	if err != nil {
		t.Fatalf("CommitCompare: %v", err)
	}
	if got.Status != "ready" {
		t.Fatalf("expected status=ready, got %+v", got)
	}
	if got.ParentOID == "" {
		t.Error("expected a non-empty parent OID for a non-root commit")
	}
	if got.ChangedFiles != 1 || len(got.Entries) != 1 || got.Entries[0].Path != "file2.txt" || got.Entries[0].Status != "added" {
		t.Errorf("unexpected entries: %+v", got)
	}
}

func TestCommitCompare_RootCommit_HasEmptyParentOID(t *testing.T) {
	dir := initRepo(t)
	e := New()
	rootOID := revParse(t, dir, "HEAD")

	got, err := e.CommitCompare(context.Background(), dir, rootOID)
	if err != nil {
		t.Fatalf("CommitCompare: %v", err)
	}
	if got.Status != "ready" || got.ParentOID != "" {
		t.Errorf("expected a root commit with empty ParentOID, got %+v", got)
	}
	if got.ChangedFiles != 1 || len(got.Entries) != 1 || got.Entries[0].Path != "README.md" {
		t.Errorf("expected README.md as the sole entry vs. the empty tree, got %+v", got)
	}
}

func TestCommitCompare_InvalidCommitID_ReturnsInvalidCommitStatus(t *testing.T) {
	dir := initRepo(t)
	e := New()
	bogus := strings.Repeat("a", 40)

	got, err := e.CommitCompare(context.Background(), dir, bogus)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if got.Status != "invalid-commit" {
		t.Errorf("expected status=invalid-commit, got %+v", got)
	}
}

func TestBranchCompare_ComparesCurrentHeadAgainstBaseRef(t *testing.T) {
	dir := initRepo(t)
	cmd := exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout -b feature: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("new file\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	e := New()
	if _, err := e.Commit(context.Background(), dir, "add file2", nil); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	got, err := e.BranchCompare(context.Background(), dir, "main")
	if err != nil {
		t.Fatalf("BranchCompare: %v", err)
	}
	if got.Status != "ready" || got.CompareRef != "feature" || got.CommitsAhead != 1 {
		t.Errorf("unexpected result: %+v", got)
	}
	if got.ChangedFiles != 1 || len(got.Entries) != 1 || got.Entries[0].Path != "file2.txt" {
		t.Errorf("unexpected entries: %+v", got)
	}
}

func TestBranchCompare_InvalidBaseRef_ReturnsInvalidBaseStatus(t *testing.T) {
	dir := initRepo(t)
	e := New()

	got, err := e.BranchCompare(context.Background(), dir, "does-not-exist")
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if got.Status != "invalid-base" {
		t.Errorf("expected status=invalid-base, got %+v", got)
	}
}

func TestBranchCompare_RejectsBaseRefStartingWithDash(t *testing.T) {
	dir := initRepo(t)
	e := New()

	if _, err := e.BranchCompare(context.Background(), dir, "--evil"); err == nil {
		t.Fatal("expected an error for a baseRef starting with -")
	}
}

func TestCommitDiff_ReturnsFileContentAgainstParent(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello again\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	e := New()
	if _, err := e.Commit(context.Background(), dir, "update readme", nil); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	commitOID := revParse(t, dir, "HEAD")
	parentOID := revParse(t, dir, "HEAD^")

	got, err := e.CommitDiff(context.Background(), dir, commitOID, parentOID, "README.md", "")
	if err != nil {
		t.Fatalf("CommitDiff: %v", err)
	}
	if got.Kind != "text" || got.OriginalContent != "hello\n" || got.ModifiedContent != "hello again\n" {
		t.Errorf("unexpected diff result: %+v", got)
	}
}

func TestCommitDiff_RootCommit_EmptyOriginalContent(t *testing.T) {
	dir := initRepo(t)
	e := New()
	rootOID := revParse(t, dir, "HEAD")

	got, err := e.CommitDiff(context.Background(), dir, rootOID, "", "README.md", "")
	if err != nil {
		t.Fatalf("CommitDiff: %v", err)
	}
	if got.Kind != "text" || got.OriginalContent != "" || got.ModifiedContent != "hello\n" {
		t.Errorf("expected empty original content for a root commit, got %+v", got)
	}
}

func TestCommitDiff_RejectsNonFullObjectID(t *testing.T) {
	dir := initRepo(t)
	e := New()

	if _, err := e.CommitDiff(context.Background(), dir, "short", "", "README.md", ""); err == nil {
		t.Fatal("expected an error for a non-full-length commitOid")
	}
}

func TestBranchDiff_ReturnsFileContentAgainstMergeBase(t *testing.T) {
	dir := initRepo(t)
	cmd := exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout -b feature: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("new file\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	e := New()
	if _, err := e.Commit(context.Background(), dir, "add file2", nil); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	got, err := e.BranchDiff(context.Background(), dir, "main", "file2.txt", "")
	if err != nil {
		t.Fatalf("BranchDiff: %v", err)
	}
	if got.Kind != "text" || got.OriginalContent != "" || got.ModifiedContent != "new file\n" {
		t.Errorf("unexpected diff result: %+v", got)
	}
}

func TestSubmoduleStatus_RealSubmoduleFixture(t *testing.T) {
	submoduleSrc := initRepo(t)
	dir := initRepo(t)
	cmd := exec.Command("git", "-c", "protocol.file.allow=always", "submodule", "add", submoduleSrc, "vendor/lib")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("submodule add: %v\n%s", err, out)
	}
	e := New()

	got, err := e.SubmoduleStatus(context.Background(), dir, "vendor/lib", "")
	if err != nil {
		t.Fatalf("SubmoduleStatus: %v", err)
	}
	if len(got.Files) != 0 {
		t.Errorf("expected a clean submodule to report no file changes, got %+v", got)
	}
}

func TestFetch_UpdatesRemoteTrackingRef(t *testing.T) {
	remoteDir := initRepo(t)
	cloneDir := t.TempDir()
	cmd := exec.Command("git", "clone", remoteDir, cloneDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v\n%s", err, out)
	}
	e := New()

	// Advance the remote independently of the clone.
	if err := os.WriteFile(filepath.Join(remoteDir, "upstream-only.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := e.Commit(context.Background(), remoteDir, "upstream-only commit", nil); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	remoteHead := revParse(t, remoteDir, "HEAD")

	got, err := e.Fetch(context.Background(), cloneDir, &domain.PushTargetInput{RemoteName: "origin"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !got.Success {
		t.Errorf("expected Success=true, got %+v", got)
	}
	if trackingRef := revParse(t, cloneDir, "origin/main"); trackingRef != remoteHead {
		t.Errorf("expected origin/main to advance to %s, got %s", remoteHead, trackingRef)
	}
}

func TestFetch_NilPushTargetUsesDefaultRemote(t *testing.T) {
	remoteDir := initRepo(t)
	cloneDir := t.TempDir()
	cmd := exec.Command("git", "clone", remoteDir, cloneDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v\n%s", err, out)
	}
	e := New()

	got, err := e.Fetch(context.Background(), cloneDir, nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !got.Success {
		t.Errorf("expected Success=true, got %+v", got)
	}
}
