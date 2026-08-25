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

	got, err := e.UpstreamStatus(context.Background(), dir, "")
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
