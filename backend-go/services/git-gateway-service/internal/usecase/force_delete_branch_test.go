package usecase

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/adapter/localgit"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// initRepoWithBranch creates a real git repository in a temp dir, with an
// initial commit and an extra local branch to delete — mirrors
// internal/adapter/localgit/executor_test.go's initRepo harness (that
// helper is unexported to its own package, so this is a small,
// deliberately duplicated equivalent for this package's tests).
func initRepoWithBranch(t *testing.T, branch string) string {
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
	run("commit", "--allow-empty", "-m", "initial commit")
	run("branch", branch)

	return dir
}

func branchExists(t *testing.T, repoDir, branch string) bool {
	t.Helper()
	cmd := exec.Command("git", "branch", "--list", branch)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list failed: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out)) != ""
}

// TestForceDeleteBranch_LocalExecutor_DeletesRealBranch is the direct
// regression test for BUG-031's old TS crash-bug class: because
// GitExecutor.ForceDeleteBranch is now a REQUIRED interface method
// (TASK-194), localgit.Executor satisfying usecase.GitExecutor at all
// means it necessarily has this method — there is no way in Go to
// "forget" to implement a required interface method and still compile.
func TestForceDeleteBranch_LocalExecutor_DeletesRealBranch(t *testing.T) {
	dir := initRepoWithBranch(t, "feature-to-delete")
	if !branchExists(t, dir, "feature-to-delete") {
		t.Fatal("test setup: expected branch to exist before delete")
	}

	local := localgit.New()
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: dir}}
	relay := &fakeGitExecutor{}
	uc := NewForceDeleteBranch(resolver, local, relay)

	if err := uc.Execute(context.Background(), "wt-1", "feature-to-delete"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branchExists(t, dir, "feature-to-delete") {
		t.Error("expected branch to be gone after ForceDeleteBranch")
	}
}

func TestForceDeleteBranch_RelayUnsupported_ReturnsFailedPrecondition(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{forceDeleteBranchErr: fmt.Errorf("%w: agent build too old", domain.ErrForceDeleteBranchUnsupported)}
	uc := NewForceDeleteBranch(resolver, local, relay)

	err := uc.Execute(context.Background(), "wt-1", "feature")
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_FORCE_DELETE_UNSUPPORTED" {
		t.Fatalf("expected WORKTREE_FORCE_DELETE_UNSUPPORTED, got %v", err)
	}
	if code := status.Code(apperrors.ToGRPCStatus(err)); code != codes.FailedPrecondition {
		t.Errorf("expected codes.FailedPrecondition, got %v", code)
	}
}

func TestForceDeleteBranch_RelayOtherError_ReturnsGenericFailure(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{forceDeleteBranchErr: errors.New("relay transport error")}
	uc := NewForceDeleteBranch(resolver, local, relay)

	err := uc.Execute(context.Background(), "wt-1", "feature")
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_FORCE_DELETE_FAILED" {
		t.Fatalf("expected WORKTREE_FORCE_DELETE_FAILED, got %v", err)
	}
	if ae.Code == "WORKTREE_FORCE_DELETE_UNSUPPORTED" {
		t.Error("expected this to be distinct from the unsupported case")
	}
}
