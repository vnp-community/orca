package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestGetAgentTerminalSession_ExactCwdMatch_ReturnsFound(t *testing.T) {
	resolver := &fakeConnectionResolver{
		byWorktreeID:   map[string]domain.DevServer{"wt-1": {ID: "ds-1"}},
		connByWorktree: map[string]domain.Connection{"wt-1": {ID: "conn-1", RepoPath: "/repo"}},
	}
	sessions := &fakeTerminalSessionRepository{byPtyID: map[string]domain.TerminalSession{
		"pty-1": {PtyID: "pty-1", TenantID: "tenant-1", ConnectionID: "conn-1", Cwd: "/repo"},
	}}
	uc := NewGetAgentTerminalSession(resolver, sessions)

	got, found, err := uc.Execute(withTenant(context.Background(), "tenant-1"), "wt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if got.PtyID != "pty-1" {
		t.Errorf("expected pty-1, got %q", got.PtyID)
	}
}

func TestGetAgentTerminalSession_SubdirectoryCwd_DoesNotMatch(t *testing.T) {
	resolver := &fakeConnectionResolver{
		byWorktreeID:   map[string]domain.DevServer{"wt-1": {ID: "ds-1"}},
		connByWorktree: map[string]domain.Connection{"wt-1": {ID: "conn-1", RepoPath: "/repo"}},
	}
	sessions := &fakeTerminalSessionRepository{byPtyID: map[string]domain.TerminalSession{
		"pty-1": {PtyID: "pty-1", TenantID: "tenant-1", ConnectionID: "conn-1", Cwd: "/repo/subdir"},
	}}
	uc := NewGetAgentTerminalSession(resolver, sessions)

	_, found, err := uc.Execute(withTenant(context.Background(), "tenant-1"), "wt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected found=false for a subdirectory cwd")
	}
}

func TestGetAgentTerminalSession_MultipleMatches_ReturnsLatestLastActive(t *testing.T) {
	resolver := &fakeConnectionResolver{
		byWorktreeID:   map[string]domain.DevServer{"wt-1": {ID: "ds-1"}},
		connByWorktree: map[string]domain.Connection{"wt-1": {ID: "conn-1", RepoPath: "/repo"}},
	}
	older := time.Now().Add(-time.Hour)
	newer := time.Now()
	sessions := &fakeTerminalSessionRepository{byPtyID: map[string]domain.TerminalSession{
		"pty-old": {PtyID: "pty-old", TenantID: "tenant-1", ConnectionID: "conn-1", Cwd: "/repo", LastActiveAt: older},
		"pty-new": {PtyID: "pty-new", TenantID: "tenant-1", ConnectionID: "conn-1", Cwd: "/repo", LastActiveAt: newer},
	}}
	uc := NewGetAgentTerminalSession(resolver, sessions)

	got, found, err := uc.Execute(withTenant(context.Background(), "tenant-1"), "wt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if got.PtyID != "pty-new" {
		t.Errorf("expected the later-active session pty-new, got %q", got.PtyID)
	}
}

func TestGetAgentTerminalSession_NoConnectionResolved_ReturnsFoundFalseNotError(t *testing.T) {
	resolver := &fakeConnectionResolver{}
	sessions := &fakeTerminalSessionRepository{}
	uc := NewGetAgentTerminalSession(resolver, sessions)

	_, found, err := uc.Execute(withTenant(context.Background(), "tenant-1"), "wt-unknown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected found=false when no connection resolves for the worktree")
	}
}
