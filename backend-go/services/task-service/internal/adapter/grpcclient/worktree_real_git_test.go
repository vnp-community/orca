package grpcclient

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// runGit runs real git with a hermetic identity and no user or system config, so the test reads the same on any machine.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"HOME="+t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitFile(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", name)
	runGit(t, dir, "commit", "-m", name)
	return runGit(t, dir, "rev-parse", "HEAD")
}

// realGitGateway is git-gateway as far as the provisioner can tell, backed by real git commands on local repositories.
// CreateWorktree omits the start point when base_ref is empty, which is git's own "start from HEAD" behavior: a provisioner
// that stopped sending a ref would therefore land on the checked-out branch and fail these tests.
type realGitGateway struct {
	gitgatewayv1.GitGatewayServiceClient
	t         *testing.T
	repos     map[string]string // repo id -> path
	worktrees string            // directory the worktrees are created in
}

func (g *realGitGateway) CreateWorktree(_ context.Context, in *gitgatewayv1.CreateWorktreeRequest, _ ...grpc.CallOption) (*gitgatewayv1.CreateWorktreeResponse, error) {
	repo := g.repos[in.GetRepoId()]
	target := filepath.Join(g.worktrees, strings.ReplaceAll(in.GetBranch(), "/", "-"))
	args := []string{"worktree", "add", target, "-b", in.GetBranch()}
	if in.GetBaseRef() != "" {
		args = append(args, in.GetBaseRef())
	}
	runGit(g.t, repo, args...)
	return &gitgatewayv1.CreateWorktreeResponse{WorktreeId: "wt-" + in.GetBranch(), Path: target}, nil
}

// BaseRefDefault reads the remote's default branch from origin/HEAD, the way the real executor's first lookup does; a
// repository without that ref cannot answer, and the error is returned as it is.
func (g *realGitGateway) BaseRefDefault(_ context.Context, in *gitgatewayv1.BaseRefDefaultRequest, _ ...grpc.CallOption) (*gitgatewayv1.BaseRefDefaultResponse, error) {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = g.repos[in.GetRepoId()]
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("symbolic-ref origin/HEAD: %w: %s", err, out)
	}
	return &gitgatewayv1.BaseRefDefaultResponse{Ref: strings.TrimPrefix(strings.TrimSpace(string(out)), "refs/remotes/origin/")}, nil
}

type gitFixture struct {
	repo                     string
	mainSHA, releaseSHA, now string // now: the checked-out third branch
	provisioner              *WorktreeProvisioner
	gateway                  *realGitGateway
}

// newGitFixture builds origin (main, release/x) and a clone whose main checkout sits on a third branch, so "HEAD", the default
// branch and the chosen branch are three different commits.
func newGitFixture(t *testing.T) gitFixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("these tests run real git: %v", err)
	}
	root := t.TempDir()
	origin := filepath.Join(root, "origin")
	if err := os.MkdirAll(origin, 0o750); err != nil {
		t.Fatal(err)
	}
	runGit(t, origin, "init")
	runGit(t, origin, "symbolic-ref", "HEAD", "refs/heads/main") // `init -b` needs git 2.28
	mainSHA := commitFile(t, origin, "base.txt")
	runGit(t, origin, "checkout", "-b", "release/x")
	releaseSHA := commitFile(t, origin, "release.txt")
	runGit(t, origin, "checkout", "main")

	repo := filepath.Join(root, "repo")
	runGit(t, root, "clone", origin, repo)
	runGit(t, repo, "branch", "release/x", "origin/release/x")
	runGit(t, repo, "checkout", "-b", "feature/third")
	thirdSHA := commitFile(t, repo, "third.txt")
	if mainSHA == releaseSHA || mainSHA == thirdSHA || releaseSHA == thirdSHA {
		t.Fatal("the three branches must be three different commits")
	}

	worktrees := filepath.Join(root, "worktrees")
	if err := os.MkdirAll(worktrees, 0o750); err != nil {
		t.Fatal(err)
	}
	gw := &realGitGateway{t: t, repos: map[string]string{"repo-1": repo}, worktrees: worktrees}
	projects := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{Repos: []*projectv1.Repo{{Id: "repo-1", ProjectId: "proj-1"}}}}
	return gitFixture{repo: repo, mainSHA: mainSHA, releaseSHA: releaseSHA, now: thirdSHA, provisioner: NewWorktreeProvisioner(gw, projects), gateway: gw}
}

func TestEnsureWorktree_RealGit_StartsFromTheChosenRefNotTheCheckedOutBranch(t *testing.T) {
	f := newGitFixture(t)
	_, path, err := f.provisioner.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1", RequestID: "req-1"}, "release/x")
	if err != nil {
		t.Fatal(err)
	}
	if got := runGit(t, path, "rev-parse", "HEAD"); got != f.releaseSHA {
		t.Fatalf("worktree HEAD = %s, want release/x %s (main %s, checked out %s)", got, f.releaseSHA, f.mainSHA, f.now)
	}
	if got := runGit(t, path, "rev-parse", "--abbrev-ref", "HEAD"); got != "task/task-1" {
		t.Fatalf("the task keeps its own branch, got %q", got)
	}
}

func TestEnsureWorktree_RealGit_EmptyRefStartsFromTheRepositoryDefaultBranch(t *testing.T) {
	f := newGitFixture(t)
	for name, task := range map[string]domain.Task{
		"task of a Request":   {ID: "task-1", ProjectID: "proj-1", RequestID: "req-1"},
		"task run on its own": {ID: "task-2", ProjectID: "proj-1"},
	} {
		t.Run(name, func(t *testing.T) {
			_, path, err := f.provisioner.EnsureWorktree(ctxWithTenant(t), "tenant-1", task, "")
			if err != nil {
				t.Fatal(err)
			}
			got := runGit(t, path, "rev-parse", "HEAD")
			if got == f.now {
				t.Fatal("the worktree started from the main checkout's current branch")
			}
			if got != f.mainSHA {
				t.Fatalf("worktree HEAD = %s, want the default branch main %s", got, f.mainSHA)
			}
		})
	}
}

func TestEnsureWorktree_RealGit_UnknownDefaultBranchIsAnErrorAndCreatesNothing(t *testing.T) {
	f := newGitFixture(t)
	runGit(t, f.repo, "remote", "remove", "origin") // no origin/HEAD any more: the default cannot be answered
	_, _, err := f.provisioner.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1", RequestID: "req-1"}, "")
	if err == nil || !strings.Contains(err.Error(), "default branch") {
		t.Fatalf("want an explicit default-branch error, got %v", err)
	}
	if entries, _ := os.ReadDir(f.gateway.worktrees); len(entries) != 0 {
		t.Fatalf("nothing may be created from HEAD: %v", entries)
	}
	if got := runGit(t, f.repo, "branch", "--list", "task/task-1"); got != "" {
		t.Fatalf("the task branch must not exist, got %q", got)
	}
	// An explicit ref still works with no remote: the default is only looked up when none is given.
	if _, path, err := f.provisioner.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"}, "release/x"); err != nil || runGit(t, path, "rev-parse", "HEAD") != f.releaseSHA {
		t.Fatalf("explicit ref after the failure: %v", err)
	}
}
