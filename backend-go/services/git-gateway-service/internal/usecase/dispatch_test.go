package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// fakeConnectionResolver is an in-memory ConnectionResolver — test against a
// fake, never a real infra-fleet-service call, per
// specs/backend-go/standards/testing-strategy.md's unit-test section.
type fakeConnectionResolver struct {
	conn ResolvedConnection
	err  error
}

func (f *fakeConnectionResolver) ResolveConnection(ctx context.Context, worktreeID string) (ResolvedConnection, error) {
	return f.conn, f.err
}

// fakeGitExecutor is a GitExecutor that records which of its methods was
// called, so tests can assert *which* executor (local vs. relay) a usecase
// dispatched to without either implementation doing real work.
type fakeGitExecutor struct {
	name string // "local" or "relay", for assertion messages

	calledGetStatus bool
	calledGetDiff   bool
	calledCommit    bool
	calledPush      bool
	calledPull      bool

	gotRepoPath string

	statusErr error
	diffErr   error
	commitErr error
	pushErr   error
	pullErr   error
}

func (f *fakeGitExecutor) GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error) {
	f.calledGetStatus = true
	f.gotRepoPath = repoPath
	if f.statusErr != nil {
		return domain.GitStatus{}, f.statusErr
	}
	return domain.GitStatus{Branch: "main"}, nil
}

func (f *fakeGitExecutor) GetDiff(ctx context.Context, repoPath string, staged bool) (domain.DiffResult, error) {
	f.calledGetDiff = true
	f.gotRepoPath = repoPath
	if f.diffErr != nil {
		return domain.DiffResult{}, f.diffErr
	}
	return domain.DiffResult{UnifiedDiff: "diff"}, nil
}

func (f *fakeGitExecutor) Commit(ctx context.Context, repoPath, message string, paths []string) (domain.CommitResult, error) {
	f.calledCommit = true
	f.gotRepoPath = repoPath
	if f.commitErr != nil {
		return domain.CommitResult{}, f.commitErr
	}
	return domain.CommitResult{CommitSHA: "abc123"}, nil
}

func (f *fakeGitExecutor) Push(ctx context.Context, repoPath, remote, branch string) (domain.PushResult, error) {
	f.calledPush = true
	f.gotRepoPath = repoPath
	if f.pushErr != nil {
		return domain.PushResult{}, f.pushErr
	}
	return domain.PushResult{Success: true}, nil
}

func (f *fakeGitExecutor) Pull(ctx context.Context, repoPath string) (domain.PullResult, error) {
	f.calledPull = true
	f.gotRepoPath = repoPath
	if f.pullErr != nil {
		return domain.PullResult{}, f.pullErr
	}
	return domain.PullResult{Success: true}, nil
}

func TestGetStatus_NotConnected_RoutesToLocalExecutor(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo/wt1"}}
	local := &fakeGitExecutor{name: "local"}
	relay := &fakeGitExecutor{name: "relay"}
	uc := NewGetStatus(resolver, local, relay)

	got, err := uc.Execute(context.Background(), GetStatusInput{WorktreeID: "wt1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !local.calledGetStatus {
		t.Error("expected local executor to be called when Connected=false")
	}
	if relay.calledGetStatus {
		t.Error("expected relay executor NOT to be called when Connected=false")
	}
	if local.gotRepoPath != "/repo/wt1" {
		t.Errorf("expected resolved repo path to be passed through, got %q", local.gotRepoPath)
	}
	if got.Branch != "main" {
		t.Errorf("expected result from local executor, got %+v", got)
	}
}

func TestGetStatus_Connected_RoutesToRelayExecutor(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo/wt1"}}
	local := &fakeGitExecutor{name: "local"}
	relay := &fakeGitExecutor{name: "relay"}
	uc := NewGetStatus(resolver, local, relay)

	_, err := uc.Execute(context.Background(), GetStatusInput{WorktreeID: "wt1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !relay.calledGetStatus {
		t.Error("expected relay executor to be called when Connected=true")
	}
	if local.calledGetStatus {
		t.Error("expected local executor NOT to be called when Connected=true")
	}
}

func TestGetStatus_MissingWorktreeID_ReturnsError(t *testing.T) {
	uc := NewGetStatus(&fakeConnectionResolver{}, &fakeGitExecutor{}, &fakeGitExecutor{})
	_, err := uc.Execute(context.Background(), GetStatusInput{})
	if err == nil {
		t.Fatal("expected error for missing worktree_id")
	}
}

func TestGetStatus_ResolverFailure_Propagates(t *testing.T) {
	resolver := &fakeConnectionResolver{err: errors.New("infra-fleet-service unreachable")}
	uc := NewGetStatus(resolver, &fakeGitExecutor{}, &fakeGitExecutor{})
	_, err := uc.Execute(context.Background(), GetStatusInput{WorktreeID: "wt1"})
	if err == nil {
		t.Fatal("expected resolver error to propagate")
	}
}

func TestGetDiff_RoutesByConnectionState(t *testing.T) {
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	uc := NewGetDiff(&fakeConnectionResolver{conn: ResolvedConnection{Connected: true}}, local, relay)

	got, err := uc.Execute(context.Background(), GetDiffInput{WorktreeID: "wt1", Staged: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !relay.calledGetDiff || local.calledGetDiff {
		t.Error("expected GetDiff to route to relay when Connected=true")
	}
	if got.UnifiedDiff != "diff" {
		t.Errorf("unexpected diff result: %+v", got)
	}
}

func TestCommit_RoutesByConnectionState(t *testing.T) {
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	uc := NewCommit(&fakeConnectionResolver{conn: ResolvedConnection{Connected: false}}, local, relay)

	got, err := uc.Execute(context.Background(), CommitInput{WorktreeID: "wt1", Message: "fix bug"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !local.calledCommit || relay.calledCommit {
		t.Error("expected Commit to route to local when Connected=false")
	}
	if got.CommitSHA != "abc123" {
		t.Errorf("unexpected commit result: %+v", got)
	}
}

func TestCommit_MissingMessage_ReturnsError(t *testing.T) {
	uc := NewCommit(&fakeConnectionResolver{}, &fakeGitExecutor{}, &fakeGitExecutor{})
	_, err := uc.Execute(context.Background(), CommitInput{WorktreeID: "wt1"})
	if err == nil {
		t.Fatal("expected error for missing commit message")
	}
}

func TestPush_RoutesByConnectionState(t *testing.T) {
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	uc := NewPush(&fakeConnectionResolver{conn: ResolvedConnection{Connected: true}}, local, relay)

	got, err := uc.Execute(context.Background(), PushInput{WorktreeID: "wt1", Remote: "origin", Branch: "main"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !relay.calledPush || local.calledPush {
		t.Error("expected Push to route to relay when Connected=true")
	}
	if !got.Success {
		t.Errorf("unexpected push result: %+v", got)
	}
}

func TestPull_RoutesByConnectionState(t *testing.T) {
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	uc := NewPull(&fakeConnectionResolver{conn: ResolvedConnection{Connected: false}}, local, relay)

	got, err := uc.Execute(context.Background(), PullInput{WorktreeID: "wt1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !local.calledPull || relay.calledPull {
		t.Error("expected Pull to route to local when Connected=false")
	}
	if !got.Success {
		t.Errorf("unexpected pull result: %+v", got)
	}
}

func TestGenerateCommitMessage_AlwaysReturnsNotImplemented(t *testing.T) {
	uc := NewGenerateCommitMessage()
	_, err := uc.Execute(context.Background(), GenerateCommitMessageInput{WorktreeID: "wt1"})
	if !errors.Is(err, ErrGenerateCommitMessageNotImplemented) {
		t.Fatalf("expected ErrGenerateCommitMessageNotImplemented, got %v", err)
	}
}
