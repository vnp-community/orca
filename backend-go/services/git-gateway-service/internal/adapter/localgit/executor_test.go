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

	unstaged, err := e.GetDiff(context.Background(), dir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(unstaged.UnifiedDiff, "changed content") {
		t.Errorf("expected unstaged diff to contain the change, got: %s", unstaged.UnifiedDiff)
	}

	staged, err := e.GetDiff(context.Background(), dir, true)
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
