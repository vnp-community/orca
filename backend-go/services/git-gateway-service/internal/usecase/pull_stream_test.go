package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestPullStream_NotConnected_RoutesToLocalExecutor(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo/wt1"}}
	local := &fakeStreamingGitExecutor{name: "local", lines: []domain.GitProgressLine{
		{Line: "Updating a1b2c3..d4e5f6", Source: "stdout"},
		{Line: "Fast-forward", Source: "stdout"},
	}}
	relay := &fakeStreamingGitExecutor{name: "relay"}
	uc := NewPullStream(resolver, local, relay)

	var got []domain.GitProgressLine
	err := uc.Execute(context.Background(), PullInputStream{WorktreeID: "wt1"}, func(l domain.GitProgressLine) error {
		got = append(got, l)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !local.calledPullStream {
		t.Error("expected local executor to be called when Connected=false")
	}
	if relay.calledPullStream {
		t.Error("expected relay executor NOT to be called when Connected=false")
	}
	if local.gotRepoPath != "/repo/wt1" {
		t.Errorf("expected resolved repo path to be passed through, got %q", local.gotRepoPath)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 frames delivered in order, got %d: %+v", len(got), got)
	}
	if !got[2].IsFinal || !got[2].Success {
		t.Errorf("expected the last frame to be the final success frame, got %+v", got[2])
	}
}

func TestPullStream_Connected_RelayWebSocket_RoutesToRelayExecutor(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{
		Connected: true, ConnectionID: "conn-1", RepoPath: "/repo/wt1",
		Mode: infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_WEBSOCKET,
	}}
	local := &fakeStreamingGitExecutor{name: "local"}
	relay := &fakeStreamingGitExecutor{name: "relay"}
	uc := NewPullStream(resolver, local, relay)

	err := uc.Execute(context.Background(), PullInputStream{WorktreeID: "wt1"}, func(domain.GitProgressLine) error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !relay.calledPullStream || local.calledPullStream {
		t.Error("expected PullStream to route to relay when Connected=true over relay-websocket")
	}
}

// TestPullStream_RelaySSH_ShortCircuitsBeforeAnyExecutorCall mirrors
// PushStream's identical regression guard — see that test's doc comment.
func TestPullStream_RelaySSH_ShortCircuitsBeforeAnyExecutorCall(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{
		Connected: true, ConnectionID: "conn-1", RepoPath: "/repo/wt1",
		Mode: infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH,
	}}
	local := &fakeStreamingGitExecutor{name: "local"}
	relay := &fakeStreamingGitExecutor{name: "relay"}
	uc := NewPullStream(resolver, local, relay)

	err := uc.Execute(context.Background(), PullInputStream{WorktreeID: "wt1"}, func(domain.GitProgressLine) error {
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
	if local.calledPullStream || relay.calledPullStream {
		t.Fatal("expected zero StreamingGitExecutor calls when relay-ssh short-circuits")
	}
}

func TestPullStream_MissingWorktreeID_ReturnsError(t *testing.T) {
	uc := NewPullStream(&fakeConnectionResolver{}, &fakeStreamingGitExecutor{}, &fakeStreamingGitExecutor{})
	err := uc.Execute(context.Background(), PullInputStream{}, func(domain.GitProgressLine) error { return nil })
	if err == nil {
		t.Fatal("expected error for missing worktree_id")
	}
}

func TestPullStream_ResolverFailure_Propagates(t *testing.T) {
	resolver := &fakeConnectionResolver{err: errors.New("infra-fleet-service unreachable")}
	uc := NewPullStream(resolver, &fakeStreamingGitExecutor{}, &fakeStreamingGitExecutor{})
	err := uc.Execute(context.Background(), PullInputStream{WorktreeID: "wt1"}, func(domain.GitProgressLine) error { return nil })
	if err == nil {
		t.Fatal("expected resolver error to propagate")
	}
}
