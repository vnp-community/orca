package localgit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type statusRow struct {
	path  string
	state domain.FileState
	area  string
}

func rowsOf(s domain.GitStatus) []statusRow {
	out := make([]statusRow, 0, len(s.Files))
	for _, f := range s.Files {
		out = append(out, statusRow{f.Path, f.State, f.Area})
	}
	return out
}

func assertRows(t *testing.T, got domain.GitStatus, want []statusRow) {
	t.Helper()
	rows := rowsOf(got)
	if len(rows) != len(want) {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}
}

// The G5c scenario on real git: a task's linked worktree `<repo>-task-<id>`
// with one edited tracked file and one new file in a directory that does not
// exist in the commit. Source Control must see both files, by file path.
func TestGetStatus_LinkedTaskWorktree_ReportsEditedAndNewFileInNewDir(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "src", "calc.py"), "x = 1\n")
	gitIn(t, repo, "add", ".")
	gitIn(t, repo, "commit", "-m", "calc")

	worktree := repo + "-task-abc"
	t.Cleanup(func() { _ = os.RemoveAll(worktree) })
	gitIn(t, repo, "worktree", "add", "-b", "task/abc", worktree)

	writeFile(t, filepath.Join(worktree, "src", "calc.py"), "x = 2\n")
	writeFile(t, filepath.Join(worktree, "tests", "test_calc.py"), "def test(): pass\n")

	status, err := New().GetStatus(context.Background(), worktree)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status.Branch != "task/abc" {
		t.Errorf("Branch = %q", status.Branch)
	}
	assertRows(t, status, []statusRow{
		{"src/calc.py", domain.FileStateModified, "unstaged"},
		{"tests/test_calc.py", domain.FileStateUntracked, "untracked"},
	})
}

// The same worktree reached through a symlinked parent (macOS /var vs
// /private/var, Linux /tmp links): git must still run in the worktree.
func TestGetStatus_LinkedTaskWorktree_ViaSymlinkedPath(t *testing.T) {
	real := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(real); err == nil {
		real = resolved
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	repo := filepath.Join(real, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "init", "-b", "main")
	gitIn(t, repo, "config", "user.email", "t@example.com")
	gitIn(t, repo, "config", "user.name", "T")
	writeFile(t, filepath.Join(repo, "a.txt"), "1\n")
	gitIn(t, repo, "add", ".")
	gitIn(t, repo, "commit", "-m", "a")
	gitIn(t, repo, "worktree", "add", "-b", "task/x", filepath.Join(real, "repo-task-x"))
	writeFile(t, filepath.Join(real, "repo-task-x", "a.txt"), "2\n")

	status, err := New().GetStatus(context.Background(), filepath.Join(link, "repo-task-x"))
	if err != nil {
		t.Fatalf("GetStatus via symlink: %v", err)
	}
	assertRows(t, status, []statusRow{{"a.txt", domain.FileStateModified, "unstaged"}})
}

func TestGetStatus_StagedThenEditedYieldsOneRowPerArea(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, filepath.Join(dir, "b.txt"), "1\n")
	gitIn(t, dir, "add", "b.txt")
	writeFile(t, filepath.Join(dir, "b.txt"), "2\n")

	status, err := New().GetStatus(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, status, []statusRow{
		{"b.txt", domain.FileStateAdded, "staged"},
		{"b.txt", domain.FileStateModified, "unstaged"},
	})
}

func TestGetStatus_RenameKeepsOldPath(t *testing.T) {
	dir := initRepo(t)
	gitIn(t, dir, "mv", "README.md", "README2.md")

	status, err := New().GetStatus(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Files) != 1 {
		t.Fatalf("rows = %+v", status.Files)
	}
	f := status.Files[0]
	if f.Path != "README2.md" || f.OldPath != "README.md" || f.State != domain.FileStateRenamed || f.Area != "staged" {
		t.Errorf("unexpected rename row: %+v", f)
	}
}

func TestParsePorcelainStatus_ConflictRowCarriesKind(t *testing.T) {
	status := parsePorcelainStatus("## main\nUU a.go\nDU b.go\n")
	if len(status.Files) != 2 {
		t.Fatalf("rows = %+v", status.Files)
	}
	if f := status.Files[0]; f.State != domain.FileStateConflicted || f.ConflictKind != "both_modified" || f.ConflictStatus != "unresolved" || f.Area != "unstaged" {
		t.Errorf("UU row: %+v", f)
	}
	if f := status.Files[1]; f.ConflictKind != "deleted_by_us" {
		t.Errorf("DU row: %+v", f)
	}
}
