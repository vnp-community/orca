package localgit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Real `git` binary, no fakes — same convention as executor_test.go.
func TestExecutor_InitRepo(t *testing.T) {
	e := &Executor{}

	t.Run("initializes a repo at an existing non-git folder", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
			t.Fatalf("writing seed file: %v", err)
		}

		path, branch, remoteAdded, err := e.InitRepo(context.Background(), dir, "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != dir {
			t.Errorf("got path %q, want %q", path, dir)
		}
		if remoteAdded {
			t.Error("expected remoteAdded to be false when no remote URL given")
		}
		if branch == "" {
			t.Error("expected a resolved default branch")
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			t.Errorf(".git directory not created: %v", err)
		}
	})

	t.Run("creates destPath if it doesn't exist yet", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested", "new-folder")
		path, _, _, err := e.InitRepo(context.Background(), dir, "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != dir {
			t.Errorf("got path %q, want %q", path, dir)
		}
	})

	t.Run("respects an explicit default branch", func(t *testing.T) {
		dir := t.TempDir()
		_, branch, _, err := e.InitRepo(context.Background(), dir, "trunk", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if branch != "trunk" {
			t.Errorf("got branch %q, want trunk", branch)
		}
	})

	t.Run("adds a remote in the same call when remoteURL is given", func(t *testing.T) {
		dir := t.TempDir()
		_, _, remoteAdded, err := e.InitRepo(context.Background(), dir, "", "", "https://example.com/org/repo.git")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !remoteAdded {
			t.Error("expected remoteAdded to be true")
		}
		cmd := exec.Command("git", "remote", "get-url", "origin")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git remote get-url failed: %v\n%s", err, out)
		}
		if got := strings.TrimSpace(string(out)); got != "https://example.com/org/repo.git" {
			t.Errorf("got remote url %q, want https://example.com/org/repo.git", got)
		}
	})

	t.Run("uses a custom remote name when given", func(t *testing.T) {
		dir := t.TempDir()
		if _, _, _, err := e.InitRepo(context.Background(), dir, "", "upstream", "https://example.com/org/repo.git"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cmd := exec.Command("git", "remote", "get-url", "upstream")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git remote get-url upstream failed: %v\n%s", err, out)
		}
	})

	t.Run("is idempotent against an already-initialized repo", func(t *testing.T) {
		dir := t.TempDir()
		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init failed: %v\n%s", err, out)
		}
		if _, _, _, err := e.InitRepo(context.Background(), dir, "", "", ""); err != nil {
			t.Fatalf("unexpected error re-initializing an existing repo: %v", err)
		}
	})
}
