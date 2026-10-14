package usecase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// gitDirWorkspace is a ProposalWorkspace over a plain git repository on disk, so the sync and archive run against real git
// and a real `openspec` instead of a fake. The production adapter goes through git-gateway.
type gitDirWorkspace struct {
	t   *testing.T
	dir string
}

func (g gitDirWorkspace) git(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	cmd.Dir = g.dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (g gitDirWorkspace) Ensure(context.Context, EnsureWorkspaceInput) (Workspace, error) {
	return Workspace{WorktreeID: "wt", Path: g.dir, Branch: "request/42-proposal"}, nil
}
func (g gitDirWorkspace) ReadFile(_ context.Context, _ Workspace, rel string, _ int) ([]byte, error) {
	return os.ReadFile(filepath.Join(g.dir, rel))
}
func (g gitDirWorkspace) WriteFile(_ context.Context, _ Workspace, rel string, b []byte) error {
	return os.WriteFile(filepath.Join(g.dir, rel), b, 0o644)
}
func (g gitDirWorkspace) Stat(_ context.Context, _ Workspace, rel string) (PathInfo, error) {
	fi, err := os.Stat(filepath.Join(g.dir, rel))
	return PathInfo{Exists: err == nil, IsDir: err == nil && fi.IsDir()}, nil
}
func (g gitDirWorkspace) ListDir(context.Context, Workspace, string) ([]DirEntry, error) {
	return nil, nil
}
func (g gitDirWorkspace) ChangedPaths(context.Context, Workspace) ([]ChangedPath, error) {
	out, err := g.git("status", "--porcelain", "-uall")
	if err != nil || out == "" {
		return nil, err
	}
	var paths []ChangedPath
	for _, line := range strings.Split(out, "\n") {
		paths = append(paths, ChangedPath{Path: strings.TrimSpace(line[2:]), State: strings.TrimSpace(line[:2])})
	}
	return paths, nil
}
func (g gitDirWorkspace) DiscardAll(context.Context, Workspace) error {
	if _, err := g.git("checkout", "--", "."); err != nil {
		return err
	}
	_, err := g.git("clean", "-fdq")
	return err
}
func (g gitDirWorkspace) Commit(_ context.Context, _ Workspace, msg string, paths []string) (string, error) {
	if _, err := g.git(append([]string{"add", "-A", "--"}, paths...)...); err != nil {
		return "", err
	}
	if out, err := g.git(append([]string{"commit", "-m", msg, "--"}, paths...)...); err != nil {
		return "", fmt.Errorf("%w: %s", err, out)
	}
	return g.git("rev-parse", "HEAD")
}
func (g gitDirWorkspace) Remove(context.Context, Workspace) error { return nil }

type localExec struct{}

func (localExec) Exec(_ context.Context, _ AnalysisConnection, in ExecInput) (ExecResult, error) {
	cmd := exec.Command(in.Binary, in.Args...)
	cmd.Dir = in.Cwd
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	switch {
	case errors.As(err, &ee):
		return ExecResult{Stderr: string(out), ExitCode: ee.ExitCode()}, nil
	case err != nil:
		return ExecResult{SpawnError: err.Error()}, nil
	}
	return ExecResult{Stdout: string(out)}, nil
}

// TestTasksMdSyncer_RealOpenSpecAndGit runs the whole step on a temporary repository: the ticks, the archive that moves
// the change and creates the spec, and the scope check against what the real tools changed. It is skipped without both tools.
func TestTasksMdSyncer_RealOpenSpecAndGit(t *testing.T) {
	for _, bin := range []string{"git", "openspec"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s is not installed", bin)
		}
	}
	dir := t.TempDir()
	g := gitDirWorkspace{t: t, dir: dir}
	if out, err := g.git("init", "-q", "-b", "main"); err != nil {
		t.Fatal(out, err)
	}
	change := filepath.Join(dir, "openspec", "changes", syncChangeID)
	if err := os.MkdirAll(filepath.Join(change, "specs", "ai"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"proposal.md": "## Why\nThe product needs an AI client so teams can chat with a provider.\n\n## What Changes\n- Add an AI client.\n\n## Impact\n- Affected specs: ai\n",
		"tasks.md":    syncTasksMd,
		"specs/ai/spec.md": "## ADDED Requirements\n### Requirement: Chat round trip\nThe system SHALL send a chat request to the configured provider and return its reply.\n\n" +
			"#### Scenario: AC-AI-01 reply returned\n- **WHEN** a chat request is sent\n- **THEN** the provider's reply is returned\n",
	}
	for rel, body := range files {
		if err := os.WriteFile(filepath.Join(change, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := g.git("add", "-A"); err != nil {
		t.Fatal(out, err)
	}
	if out, err := g.git("commit", "-qm", "proposal"); err != nil {
		t.Fatal(out, err)
	}

	w := newSyncWorld(t)
	completeRequest(w)
	w.s.Workspace, w.s.Exec = g, localExec{}
	c := w.stored()
	c.Bundle = map[string]string{"tasks.md": syncTasksMd}
	_, _ = w.changes.Upsert(context.Background(), c)

	res, err := w.s.Flush(context.Background(), "req-1")
	if err != nil || !res.Archived || res.Ticked != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	archived, _ := filepath.Glob(filepath.Join(dir, "openspec", "changes", "archive", "*-"+syncChangeID, "tasks.md"))
	if len(archived) != 1 {
		t.Fatalf("the change must be under changes/archive: %v", archived)
	}
	if b, _ := os.ReadFile(archived[0]); !strings.Contains(string(b), "- [x] T1.1 one") || !strings.Contains(string(b), "- [x] T1.2 two") {
		t.Fatalf("the archived tasks.md must carry the ticks:\n%s", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "openspec", "specs", "ai", "spec.md")); err != nil {
		t.Fatalf("the spec delta must be merged into openspec/specs: %v", err)
	}
	if left, _ := g.git("status", "--porcelain"); left != "" {
		t.Fatalf("everything the archive changed must be committed:\n%s", left)
	}
	if got := w.stored(); got.Status != domain.ChangeStatusArchived || got.Commit == "" {
		t.Fatalf("%+v", got)
	}
	if log, _ := g.git("log", "--format=%s"); !strings.Contains(log, "archive "+syncChangeID) || !strings.Contains(log, "tick finished tasks") {
		t.Fatalf("the branch history:\n%s", log)
	}
}
