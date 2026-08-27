package usecase

import (
	"context"
	"testing"
	"time"
)

func TestWaitTerminalSession_RequiresTenantContext(t *testing.T) {
	uc := NewWaitTerminalSession(&fakeTerminalSessionRepository{}, &fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	_, err := uc.Execute(context.Background(), WaitTerminalSessionInput{PtyID: "pty-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestWaitTerminalSession_UnknownPtyID_ReturnsError(t *testing.T) {
	uc := NewWaitTerminalSession(&fakeTerminalSessionRepository{}, &fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, WaitTerminalSessionInput{PtyID: "pty-unknown"})
	if err == nil {
		t.Fatal("expected a not-found error")
	}
}

func TestWaitTerminalSession_ExitEvent_ReturnsExited(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	events := make(chan PtyEvent, 1)
	agent := &fakeDevServerAgentClient{streamPtyEvents: events}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	uc := NewWaitTerminalSession(sessions, resolver, agent)

	events <- PtyEvent{PtyID: "pty-1", Exited: true, ExitCode: 3}
	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, WaitTerminalSessionInput{PtyID: "pty-1", TimeoutMs: 5000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Exited || result.ExitCode != 3 {
		t.Errorf("expected Exited=true ExitCode=3, got %+v", result)
	}
	if result.TimedOut {
		t.Error("expected TimedOut=false on a real exit")
	}
}

func TestWaitTerminalSession_IgnoresOutputEvents_KeepsWaitingForExit(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	events := make(chan PtyEvent, 2)
	agent := &fakeDevServerAgentClient{streamPtyEvents: events}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	uc := NewWaitTerminalSession(sessions, resolver, agent)

	events <- PtyEvent{PtyID: "pty-1", Data: []byte("still running")}
	events <- PtyEvent{PtyID: "pty-1", Exited: true, ExitCode: 0}
	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, WaitTerminalSessionInput{PtyID: "pty-1", TimeoutMs: 5000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Exited {
		t.Errorf("expected an intervening output event not to short-circuit the wait, got %+v", result)
	}
}

func TestWaitTerminalSession_NoExit_TimesOut(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	agent := &fakeDevServerAgentClient{streamPtyEvents: make(chan PtyEvent)}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	uc := NewWaitTerminalSession(sessions, resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	start := time.Now()
	result, err := uc.Execute(ctx, WaitTerminalSessionInput{PtyID: "pty-1", TimeoutMs: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.TimedOut {
		t.Errorf("expected TimedOut=true, got %+v", result)
	}
	if time.Since(start) > 2*time.Second {
		t.Error("expected the wait to be bounded by the requested timeout, not the 30s cap")
	}
}
