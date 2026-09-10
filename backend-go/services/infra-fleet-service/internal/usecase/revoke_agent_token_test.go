package usecase

import (
	"context"
	"testing"
)

func TestRevokeAgentToken_RequiresTenantContext(t *testing.T) {
	uc := NewRevokeAgentToken(newFakeAgentTokenRepository(), &fakeLiveSessionCloser{})
	err := uc.Execute(context.Background(), "ds-1", "tok-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestRevokeAgentToken_ClosesLiveSessionExactlyOnce(t *testing.T) {
	repo := newFakeAgentTokenRepository()
	sessions := &fakeLiveSessionCloser{closed: 1}
	uc := NewRevokeAgentToken(repo, sessions)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "ds-1", "tok-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessions.calls != 1 {
		t.Fatalf("expected CloseSessionsForDevServerToken to be called exactly once, got %d", sessions.calls)
	}
	if sessions.lastDevID != "ds-1" || sessions.lastTokenID != "tok-1" {
		t.Errorf("close called with (%q, %q), want (ds-1, tok-1)", sessions.lastDevID, sessions.lastTokenID)
	}
	if len(repo.revoked) != 1 || repo.revoked[0] != "tok-1" {
		t.Errorf("expected repo.Revoke to be called with tok-1, got %v", repo.revoked)
	}
}

func TestRevokeAgentToken_RepositoryFailurePropagates(t *testing.T) {
	repo := newFakeAgentTokenRepository()
	repo.revokeErr = context.DeadlineExceeded
	sessions := &fakeLiveSessionCloser{}
	uc := NewRevokeAgentToken(repo, sessions)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "ds-1", "tok-1"); err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
	if sessions.calls != 0 {
		t.Error("expected CloseSessionsForDevServerToken not to be called when Revoke fails")
	}
}
