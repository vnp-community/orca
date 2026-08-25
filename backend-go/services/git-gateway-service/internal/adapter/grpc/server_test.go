package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

// fakeResolver/fakeExecutor let this test exercise wire<->usecase
// translation without touching a real ConnectionResolver/GitExecutor
// implementation.
type fakeResolver struct{ conn usecase.ResolvedConnection }

func (f *fakeResolver) ResolveConnection(context.Context, string) (usecase.ResolvedConnection, error) {
	return f.conn, nil
}

type fakeExecutor struct{}

func (fakeExecutor) GetStatus(context.Context, string) (domain.GitStatus, error) {
	return domain.GitStatus{
		Branch: "main",
		Files:  []domain.FileStatus{{Path: "a.txt", State: domain.FileStateModified}},
	}, nil
}

func (fakeExecutor) GetDiff(context.Context, string, bool) (domain.DiffResult, error) {
	return domain.DiffResult{UnifiedDiff: "diff --git a/a.txt b/a.txt"}, nil
}

func (fakeExecutor) Commit(context.Context, string, string, []string) (domain.CommitResult, error) {
	return domain.CommitResult{CommitSHA: "deadbeef"}, nil
}

func (fakeExecutor) Push(context.Context, string, string, string) (domain.PushResult, error) {
	return domain.PushResult{Success: true}, nil
}

func (fakeExecutor) Pull(context.Context, string) (domain.PullResult, error) {
	return domain.PullResult{Success: true, HadConflicts: false}, nil
}

func (fakeExecutor) Clone(context.Context, string, string) (string, string, error) {
	return "/repo/cloned", "main", nil
}

func (fakeExecutor) InitRepo(context.Context, string, string) (string, string, error) {
	return "/repo/init", "main", nil
}

func (fakeExecutor) BaseRefDefault(context.Context, string) (string, error) {
	return "main", nil
}

func (fakeExecutor) SearchRefs(context.Context, string, string) ([]string, error) {
	return []string{"main", "feature/x"}, nil
}

func (fakeExecutor) CheckHooks(context.Context, string) ([]string, bool, error) {
	return []string{"pre-commit", "post-checkout"}, true, nil
}

func (fakeExecutor) ReadIssueCommand(context.Context, string) (string, bool, error) {
	return `{"command":"gh issue view"}`, true, nil
}

func (fakeExecutor) WriteIssueCommand(context.Context, string, string) error {
	return nil
}

func (fakeExecutor) ScanSetupScriptImports(context.Context, string) ([]string, error) {
	return []string{"source ./lib.sh"}, nil
}

// fakeReachability is a usecase.DevServerReachability stub for exercising
// Clone/InitRepo's wire<->usecase translation.
type fakeReachability struct{ reachable bool }

func (f fakeReachability) IsReachable(context.Context, string) (bool, error) {
	return f.reachable, nil
}

// fakeAICompleter is a usecase.AICompleter stub for exercising
// GenerateCommitMessage's wire<->usecase translation.
type fakeAICompleter struct{ message string }

func (f fakeAICompleter) Complete(context.Context, string, string) (string, error) {
	return f.message, nil
}

func newTestServer() *Server {
	resolver := &fakeResolver{conn: usecase.ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	exec := fakeExecutor{}
	reachability := fakeReachability{reachable: false}
	getDiffUC := usecase.NewGetDiff(resolver, exec, exec)
	return New(
		usecase.NewGetStatus(resolver, exec, exec),
		getDiffUC,
		usecase.NewCommit(resolver, exec, exec),
		usecase.NewPush(resolver, exec, exec),
		usecase.NewPull(resolver, exec, exec),
		usecase.NewGenerateCommitMessage(resolver, getDiffUC, fakeAICompleter{message: "generated message"}),
		usecase.NewClone(reachability, exec, exec),
		usecase.NewInitRepo(reachability, exec, exec),
		usecase.NewBaseRefDefault(resolver, exec, exec),
		usecase.NewSearchRefs(resolver, exec, exec),
		usecase.NewCheckHooks(resolver, exec, exec),
		usecase.NewReadIssueCommand(resolver, exec, exec),
		usecase.NewWriteIssueCommand(resolver, exec, exec),
		usecase.NewScanSetupScriptImports(resolver, exec, exec),
	)
}

func TestServer_GetStatus_TranslatesResult(t *testing.T) {
	s := newTestServer()
	resp, err := s.GetStatus(context.Background(), &gitgatewayv1.GetStatusRequest{WorktreeId: "wt-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetBranch() != "main" {
		t.Errorf("expected branch=main, got %q", resp.GetBranch())
	}
	if len(resp.GetFiles()) != 1 || resp.GetFiles()[0].GetState() != "modified" {
		t.Errorf("unexpected files: %+v", resp.GetFiles())
	}
}

func TestServer_GetStatus_MissingWorktreeID_ReturnsInvalidArgument(t *testing.T) {
	s := newTestServer()
	_, err := s.GetStatus(context.Background(), &gitgatewayv1.GetStatusRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestServer_Commit_TranslatesResult(t *testing.T) {
	s := newTestServer()
	resp, err := s.Commit(context.Background(), &gitgatewayv1.CommitRequest{WorktreeId: "wt-1", Message: "fix"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetCommitSha() != "deadbeef" {
		t.Errorf("expected commit_sha=deadbeef, got %q", resp.GetCommitSha())
	}
}

func TestServer_Pull_TranslatesResult(t *testing.T) {
	s := newTestServer()
	resp, err := s.Pull(context.Background(), &gitgatewayv1.PullRequest{WorktreeId: "wt-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.GetSuccess() || resp.GetHadConflicts() {
		t.Errorf("unexpected pull response: %+v", resp)
	}
}

func TestServer_GenerateCommitMessage_NotConnected_ReturnsFailedPrecondition(t *testing.T) {
	// newTestServer's resolver reports Connected=false, i.e. no dev server
	// for this worktree — GenerateCommitMessage has no AI-relay path in
	// that case (see usecase.GenerateCommitMessage's doc comment).
	s := newTestServer()
	_, err := s.GenerateCommitMessage(context.Background(), &gitgatewayv1.GenerateCommitMessageRequest{WorktreeId: "wt-1"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
}

func TestServer_GenerateCommitMessage_Connected_TranslatesResult(t *testing.T) {
	resolver := &fakeResolver{conn: usecase.ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo"}}
	exec := fakeExecutor{}
	reachability := fakeReachability{reachable: true}
	getDiffUC := usecase.NewGetDiff(resolver, exec, exec)
	s := New(
		usecase.NewGetStatus(resolver, exec, exec),
		getDiffUC,
		usecase.NewCommit(resolver, exec, exec),
		usecase.NewPush(resolver, exec, exec),
		usecase.NewPull(resolver, exec, exec),
		usecase.NewGenerateCommitMessage(resolver, getDiffUC, fakeAICompleter{message: "generated message"}),
		usecase.NewClone(reachability, exec, exec),
		usecase.NewInitRepo(reachability, exec, exec),
		usecase.NewBaseRefDefault(resolver, exec, exec),
		usecase.NewSearchRefs(resolver, exec, exec),
		usecase.NewCheckHooks(resolver, exec, exec),
		usecase.NewReadIssueCommand(resolver, exec, exec),
		usecase.NewWriteIssueCommand(resolver, exec, exec),
		usecase.NewScanSetupScriptImports(resolver, exec, exec),
	)

	resp, err := s.GenerateCommitMessage(context.Background(), &gitgatewayv1.GenerateCommitMessageRequest{WorktreeId: "wt-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetMessage() != "generated message" {
		t.Errorf("expected message=%q, got %q", "generated message", resp.GetMessage())
	}
}
