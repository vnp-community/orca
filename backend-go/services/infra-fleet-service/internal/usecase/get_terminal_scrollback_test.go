package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func newScrollbackFixture(t *testing.T, events []PtyEvent) (*GetTerminalScrollback, *fakeDevServerAgentClient) {
	t.Helper()
	resolver := &fakeConnectionResolver{
		byConnectionID: map[string]domain.DevServer{"conn-1": {ID: "ds-1"}},
	}
	sessions := &fakeTerminalSessionRepository{byPtyID: map[string]domain.TerminalSession{
		"pty-1": {PtyID: "pty-1", TenantID: "tenant-1", ConnectionID: "conn-1"},
	}}
	ch := make(chan PtyEvent, len(events))
	for _, ev := range events {
		ch <- ev
	}
	agent := &fakeDevServerAgentClient{streamPtyEvents: ch}
	uc := NewGetTerminalScrollback(sessions, resolver, agent)
	uc.drainWindow = 30 * time.Millisecond // bound the test, not a real 500ms sleep
	return uc, agent
}

func TestGetTerminalScrollback_AssemblesBufferedChunksInOrder(t *testing.T) {
	uc, _ := newScrollbackFixture(t, []PtyEvent{
		{PtyID: "pty-1", Data: []byte("hello ")},
		{PtyID: "pty-1", Data: []byte("world")},
	})

	got, err := uc.Execute(withTenant(context.Background(), "tenant-1"), "pty-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Text != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", got.Text)
	}
}

func TestGetTerminalScrollback_ExcludesExitEventFromText(t *testing.T) {
	uc, _ := newScrollbackFixture(t, []PtyEvent{
		{PtyID: "pty-1", Data: []byte("output")},
		{PtyID: "pty-1", Exited: true, ExitCode: 0, Data: []byte("should-not-appear")},
	})

	got, err := uc.Execute(withTenant(context.Background(), "tenant-1"), "pty-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Text != "output" {
		t.Errorf("expected exit event excluded, got %q", got.Text)
	}
}

func TestGetTerminalScrollback_DrainWindowBoundsTheCall(t *testing.T) {
	uc, _ := newScrollbackFixture(t, nil)

	start := time.Now()
	_, err := uc.Execute(withTenant(context.Background(), "tenant-1"), "pty-1")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("expected the call to be bounded by the overridden drain window (~30ms), took %v", elapsed)
	}
}
