package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestAttachScreencast_RequiresStartFirstFrame(t *testing.T) {
	uc := NewAttachScreencast(&fakeConnectionResolver{}, &fakeDevServerAgentClient{}, NewConnectionStreamLimiter(0))

	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan ScreencastClientMessage, 1)
	inbound <- ScreencastClientMessage{Stop: true}
	_, errCh := uc.Execute(ctx, inbound)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected an error for a non-start first frame")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for errCh")
	}
}

func TestAttachScreencast_NoConnectionBoundToWorktree_ReturnsError(t *testing.T) {
	uc := NewAttachScreencast(&fakeConnectionResolver{}, &fakeDevServerAgentClient{}, NewConnectionStreamLimiter(0))

	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan ScreencastClientMessage, 1)
	inbound <- ScreencastClientMessage{Start: &ScreencastStartMessage{Params: ScreencastParams{WorktreeID: "wt-unknown"}}}
	_, errCh := uc.Execute(ctx, inbound)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected a no-connection error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for errCh")
	}
}

// seedWorktreeConnection registers a worktree-bound connection in the
// resolver — the common setup this file's tests need (mirrors
// attach_pty_test.go's seedSession, but keyed by worktree, matching
// ResolveConnectionByWorktree rather than a pre-existing session lookup).
func seedWorktreeConnection(t *testing.T, resolver *fakeConnectionResolver, worktreeID, connectionID string) {
	t.Helper()
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if resolver.byWorktreeID == nil {
		resolver.byWorktreeID = map[string]domain.DevServer{}
	}
	if resolver.connByWorktree == nil {
		resolver.connByWorktree = map[string]domain.Connection{}
	}
	resolver.byWorktreeID[worktreeID] = ds
	resolver.connByWorktree[worktreeID] = domain.Connection{ID: connectionID, WorktreeID: worktreeID}
}

func TestAttachScreencast_RelaysReadyThenFrame(t *testing.T) {
	resolver := &fakeConnectionResolver{}
	events := make(chan ScreencastEvent, 2)
	agent := &fakeDevServerAgentClient{streamScreencastEvents: events}
	seedWorktreeConnection(t, resolver, "wt-1", "conn-1")
	uc := NewAttachScreencast(resolver, agent, NewConnectionStreamLimiter(0))

	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan ScreencastClientMessage, 1)
	inbound <- ScreencastClientMessage{Start: &ScreencastStartMessage{Params: ScreencastParams{WorktreeID: "wt-1", Format: "jpeg", Quality: 70}}}
	outbound, errCh := uc.Execute(ctx, inbound)

	events <- ScreencastEvent{Ready: true, SubscriptionID: "sub-1", BrowserPageID: "page-1", Format: "jpeg"}
	select {
	case msg := <-outbound:
		if !msg.Ready || msg.SubscriptionID != "sub-1" {
			t.Errorf("expected a ready event with subscriptionId sub-1, got %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ready event")
	}

	events <- ScreencastEvent{Frame: []byte("jpeg-bytes")}
	select {
	case msg := <-outbound:
		if string(msg.Frame) != "jpeg-bytes" {
			t.Errorf("expected frame bytes to pass through opaquely, got %q", msg.Frame)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for frame event")
	}

	agent.mu.Lock()
	calledWith := agent.streamScreencastCalls
	agent.mu.Unlock()
	if len(calledWith) != 1 || calledWith[0].WorktreeID != "wt-1" {
		t.Errorf("expected StreamScreencast to be called once with worktree wt-1, got %+v", calledWith)
	}

	events <- ScreencastEvent{Ended: true}
	select {
	case msg, ok := <-outbound:
		if !ok || !msg.Ended {
			t.Fatalf("expected an ended event before outbound closes, got ok=%v msg=%+v", ok, msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ended event")
	}

	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for errCh to close after ended")
	}

	agent.mu.Lock()
	unsub := agent.streamScreencastUnsubscribed
	agent.mu.Unlock()
	if !unsub {
		t.Error("expected StreamScreencast's unsubscribe to have been called")
	}
}

func TestAttachScreencast_StreamLimitReached(t *testing.T) {
	resolver := &fakeConnectionResolver{}
	agent := &fakeDevServerAgentClient{streamScreencastEvents: make(chan ScreencastEvent)}
	seedWorktreeConnection(t, resolver, "wt-1", "conn-1")
	limiter := NewConnectionStreamLimiter(1)
	release, err := limiter.Acquire("conn-1")
	if err != nil {
		t.Fatalf("priming the limiter: %v", err)
	}
	defer release()

	uc := NewAttachScreencast(resolver, agent, limiter)
	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan ScreencastClientMessage, 1)
	inbound <- ScreencastClientMessage{Start: &ScreencastStartMessage{Params: ScreencastParams{WorktreeID: "wt-1"}}}
	_, errCh := uc.Execute(ctx, inbound)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected a stream-limit error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for errCh")
	}
}
