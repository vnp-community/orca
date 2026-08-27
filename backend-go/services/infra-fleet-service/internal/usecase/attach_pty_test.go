package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestAttachPty_RequiresAttachFirstFrame(t *testing.T) {
	uc := NewAttachPty(&fakeTerminalSessionRepository{}, &fakeConnectionResolver{}, &fakeDevServerAgentClient{}, NewConnectionStreamLimiter(0))

	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan PtyClientMessage, 1)
	inbound <- PtyClientMessage{Input: []byte("x")}
	_, errCh := uc.Execute(ctx, inbound)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected an error for a non-attach first frame")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for errCh")
	}
}

func TestAttachPty_UnknownPtyID_ReturnsError(t *testing.T) {
	uc := NewAttachPty(&fakeTerminalSessionRepository{}, &fakeConnectionResolver{}, &fakeDevServerAgentClient{}, NewConnectionStreamLimiter(0))

	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan PtyClientMessage, 1)
	inbound <- PtyClientMessage{Attach: &PtyAttachMessage{PtyID: "pty-unknown"}}
	_, errCh := uc.Execute(ctx, inbound)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected a not-found error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for errCh")
	}
}

// seedSession registers a connection-bound terminal session both in
// sessions and resolver — the common setup every terminal-usecase test
// (attach/wait/resize/kill/...) needs.
func seedSession(t *testing.T, sessions *fakeTerminalSessionRepository, resolver *fakeConnectionResolver, tenantID, ptyID, connectionID string) domain.DevServer {
	t.Helper()
	ds, err := domain.NewDevServer("ds1", tenantID, "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if resolver.byConnectionID == nil {
		resolver.byConnectionID = map[string]domain.DevServer{}
	}
	resolver.byConnectionID[connectionID] = ds
	now := time.Now().UTC()
	if _, err := sessions.Create(context.Background(), domain.TerminalSession{PtyID: ptyID, TenantID: tenantID, ConnectionID: connectionID, CreatedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("seeding session: %v", err)
	}
	return ds
}

func TestAttachPty_RelaysInputAndResizeToAgent(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	agent := &fakeDevServerAgentClient{streamPtyEvents: make(chan PtyEvent)}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	uc := NewAttachPty(sessions, resolver, agent, NewConnectionStreamLimiter(0))

	ctx, cancel := context.WithCancel(withTenant(context.Background(), "tenant-1"))
	defer cancel()
	inbound := make(chan PtyClientMessage, 4)
	inbound <- PtyClientMessage{Attach: &PtyAttachMessage{PtyID: "pty-1"}}
	inbound <- PtyClientMessage{Input: []byte("ls\n")}
	inbound <- PtyClientMessage{Resize: &PtyResizeMessage{Cols: 100, Rows: 40}}
	outbound, errCh := uc.Execute(ctx, inbound)

	deadline := time.After(2 * time.Second)
	for {
		agent.mu.Lock()
		gotWrite := len(agent.writePtyCalls) == 1
		gotResize := len(agent.resizePtyCalls) == 1
		agent.mu.Unlock()
		if gotWrite && gotResize {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for input/resize to reach the agent")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-outbound:
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the stream to end after cancel")
	}
}

func TestAttachPty_RelaysAgentOutputAndExit(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	events := make(chan PtyEvent, 2)
	agent := &fakeDevServerAgentClient{streamPtyEvents: events}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	uc := NewAttachPty(sessions, resolver, agent, NewConnectionStreamLimiter(0))

	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan PtyClientMessage, 1)
	inbound <- PtyClientMessage{Attach: &PtyAttachMessage{PtyID: "pty-1"}}
	outbound, errCh := uc.Execute(ctx, inbound)

	events <- PtyEvent{PtyID: "pty-1", Data: []byte("hello")}
	select {
	case msg := <-outbound:
		if string(msg.Output) != "hello" {
			t.Errorf("expected output %q, got %q", "hello", msg.Output)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for output event")
	}

	events <- PtyEvent{PtyID: "pty-1", Exited: true, ExitCode: 7}
	select {
	case msg, ok := <-outbound:
		if !ok {
			t.Fatal("expected an exited message before outbound closes")
		}
		if !msg.Exited || msg.ExitCode != 7 {
			t.Errorf("expected Exited=true ExitCode=7, got %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for exit event")
	}

	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for errCh to close after exit")
	}

	agent.mu.Lock()
	unsub := agent.streamPtyUnsubscribed
	agent.mu.Unlock()
	if !unsub {
		t.Error("expected StreamPty's unsubscribe to have been called")
	}
}

func TestAttachPty_StreamLimitReached(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	agent := &fakeDevServerAgentClient{streamPtyEvents: make(chan PtyEvent)}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	limiter := NewConnectionStreamLimiter(1)
	release, err := limiter.Acquire("conn-1")
	if err != nil {
		t.Fatalf("priming the limiter: %v", err)
	}
	defer release()

	uc := NewAttachPty(sessions, resolver, agent, limiter)
	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan PtyClientMessage, 1)
	inbound <- PtyClientMessage{Attach: &PtyAttachMessage{PtyID: "pty-1"}}
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
