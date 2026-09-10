package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// fakeStreamingGitExecutor is a StreamingGitExecutor that records which of
// its methods was called and replays a fixed sequence of lines (then a
// final frame) to sink — same "record which executor (local vs. relay) was
// dispatched to" shape as fakeGitExecutor (dispatch_test.go), kept separate
// since StreamingGitExecutor is its own narrow port.
type fakeStreamingGitExecutor struct {
	name string // "local" or "relay", for assertion messages

	calledPushStream bool
	calledPullStream bool
	gotRepoPath      string
	gotRemote        string
	gotBranch        string

	lines []domain.GitProgressLine // replayed to sink in order; a final (IsFinal) frame is appended automatically if none is present
	err   error                    // returned instead of replaying lines, if set
}

func (f *fakeStreamingGitExecutor) PushStream(ctx context.Context, repoPath, remote, branch string, sink func(domain.GitProgressLine) error) error {
	f.calledPushStream = true
	f.gotRepoPath = repoPath
	f.gotRemote = remote
	f.gotBranch = branch
	return f.replay(sink)
}

func (f *fakeStreamingGitExecutor) PullStream(ctx context.Context, repoPath string, sink func(domain.GitProgressLine) error) error {
	f.calledPullStream = true
	f.gotRepoPath = repoPath
	return f.replay(sink)
}

func (f *fakeStreamingGitExecutor) replay(sink func(domain.GitProgressLine) error) error {
	if f.err != nil {
		return f.err
	}
	lines := f.lines
	if len(lines) == 0 || !lines[len(lines)-1].IsFinal {
		lines = append(lines, domain.GitProgressLine{IsFinal: true, Success: true})
	}
	for _, l := range lines {
		if err := sink(l); err != nil {
			return err
		}
	}
	return nil
}

func TestPushStream_NotConnected_RoutesToLocalExecutor(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo/wt1"}}
	local := &fakeStreamingGitExecutor{name: "local", lines: []domain.GitProgressLine{
		{Line: "Enumerating objects: 3, done.", Source: "stderr"},
		{Line: "To github.com:org/repo.git", Source: "stderr"},
	}}
	relay := &fakeStreamingGitExecutor{name: "relay"}
	uc := NewPushStream(resolver, local, relay)

	var got []domain.GitProgressLine
	err := uc.Execute(context.Background(), PushInputStream{WorktreeID: "wt1", Remote: "origin", Branch: "main"}, func(l domain.GitProgressLine) error {
		got = append(got, l)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !local.calledPushStream {
		t.Error("expected local executor to be called when Connected=false")
	}
	if relay.calledPushStream {
		t.Error("expected relay executor NOT to be called when Connected=false")
	}
	if local.gotRepoPath != "/repo/wt1" || local.gotRemote != "origin" || local.gotBranch != "main" {
		t.Errorf("unexpected forwarded args: repoPath=%q remote=%q branch=%q", local.gotRepoPath, local.gotRemote, local.gotBranch)
	}
	// 2 progress lines + the auto-appended final frame, all forwarded in order.
	if len(got) != 3 {
		t.Fatalf("want 3 frames delivered in order, got %d: %+v", len(got), got)
	}
	if got[0].Line != "Enumerating objects: 3, done." || got[1].Line != "To github.com:org/repo.git" {
		t.Errorf("unexpected line order: %+v", got)
	}
	if !got[2].IsFinal || !got[2].Success {
		t.Errorf("expected the last frame to be the final success frame, got %+v", got[2])
	}
}

func TestPushStream_Connected_RelayWebSocket_RoutesToRelayExecutor(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{
		Connected: true, ConnectionID: "conn-1", RepoPath: "/repo/wt1",
		Mode: infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_WEBSOCKET,
	}}
	local := &fakeStreamingGitExecutor{name: "local"}
	relay := &fakeStreamingGitExecutor{name: "relay"}
	uc := NewPushStream(resolver, local, relay)

	err := uc.Execute(context.Background(), PushInputStream{WorktreeID: "wt1", Remote: "origin", Branch: "main"}, func(domain.GitProgressLine) error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !relay.calledPushStream || local.calledPushStream {
		t.Error("expected PushStream to route to relay when Connected=true over relay-websocket")
	}
}

// TestPushStream_RelaySSH_ShortCircuitsBeforeAnyExecutorCall is the
// regression guard this task's Verify section calls for: relay-ssh mode
// must return the typed error before ever constructing a
// StreamingGitExecutor call — fakes must record zero calls, matching
// TASK-PW-03-06's MergeBranch precedent.
func TestPushStream_RelaySSH_ShortCircuitsBeforeAnyExecutorCall(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{
		Connected: true, ConnectionID: "conn-1", RepoPath: "/repo/wt1",
		Mode: infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH,
	}}
	local := &fakeStreamingGitExecutor{name: "local"}
	relay := &fakeStreamingGitExecutor{name: "relay"}
	uc := NewPushStream(resolver, local, relay)

	err := uc.Execute(context.Background(), PushInputStream{WorktreeID: "wt1", Remote: "origin", Branch: "main"}, func(domain.GitProgressLine) error {
		t.Fatal("sink must not be called when relay-ssh short-circuits")
		return nil
	})
	if err == nil {
		t.Fatal("expected an error for relay-ssh mode")
	}
	if !errors.Is(err, domain.ErrGitOpUnsupportedOverSSHRelay) {
		t.Fatalf("expected domain.ErrGitOpUnsupportedOverSSHRelay, got %v", err)
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition AppError, got %v", err)
	}
	if local.calledPushStream || relay.calledPushStream {
		t.Fatal("expected zero StreamingGitExecutor calls when relay-ssh short-circuits")
	}
}

func TestPushStream_MissingWorktreeID_ReturnsError(t *testing.T) {
	uc := NewPushStream(&fakeConnectionResolver{}, &fakeStreamingGitExecutor{}, &fakeStreamingGitExecutor{})
	err := uc.Execute(context.Background(), PushInputStream{}, func(domain.GitProgressLine) error { return nil })
	if err == nil {
		t.Fatal("expected error for missing worktree_id")
	}
}

func TestPushStream_ResolverFailure_Propagates(t *testing.T) {
	resolver := &fakeConnectionResolver{err: errors.New("infra-fleet-service unreachable")}
	uc := NewPushStream(resolver, &fakeStreamingGitExecutor{}, &fakeStreamingGitExecutor{})
	err := uc.Execute(context.Background(), PushInputStream{WorktreeID: "wt1"}, func(domain.GitProgressLine) error { return nil })
	if err == nil {
		t.Fatal("expected resolver error to propagate")
	}
}

// TestPushStream_SinkErrorPropagatesUnmodified guards PushStream.Execute's
// "not wrapped in apperrors.New" note — a sink failure (e.g. the gRPC
// adapter's stream.Send failing) must surface exactly as returned.
func TestPushStream_SinkErrorPropagatesUnmodified(t *testing.T) {
	wantErr := errors.New("client disconnected")
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo/wt1"}}
	local := &fakeStreamingGitExecutor{lines: []domain.GitProgressLine{{Line: "first"}, {Line: "second"}}}
	uc := NewPushStream(resolver, local, &fakeStreamingGitExecutor{})

	calls := 0
	err := uc.Execute(context.Background(), PushInputStream{WorktreeID: "wt1"}, func(domain.GitProgressLine) error {
		calls++
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("want sink error to propagate unmodified, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected the stream to stop after the first sink error, got %d calls", calls)
	}
}
