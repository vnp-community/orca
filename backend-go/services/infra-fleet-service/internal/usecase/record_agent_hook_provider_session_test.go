package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestRecordAgentHookProviderSession_NoProviderSession_NoOp(t *testing.T) {
	sessions := &fakeAgentSessionRepository{}
	uc := NewRecordAgentHookProviderSession(sessions)

	if err := uc.Handle(context.Background(), "tenant-1", AgentHookEvent{WorktreeID: "wt-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessions.updateProviderSessionCall != nil {
		t.Error("expected no UpdateProviderSession call when the hook carries no providerSession")
	}
}

func TestRecordAgentHookProviderSession_NoActiveSession_NoOpNoError(t *testing.T) {
	sessions := &fakeAgentSessionRepository{}
	uc := NewRecordAgentHookProviderSession(sessions)

	err := uc.Handle(context.Background(), "tenant-1", AgentHookEvent{WorktreeID: "wt-1", ProviderSessionKey: "session_id", ProviderSessionID: "provider-1"})
	if err != nil {
		t.Fatalf("expected no error when there is no active session to correlate against, got: %v", err)
	}
	if sessions.updateProviderSessionCall != nil {
		t.Error("expected no UpdateProviderSession call when no active session exists")
	}
}

func TestRecordAgentHookProviderSession_HappyPath(t *testing.T) {
	sessions := &fakeAgentSessionRepository{byID: map[string]domain.AgentSession{
		"sess-1": {ID: "sess-1", TenantID: "tenant-1", WorktreeID: "wt-1", Status: domain.AgentStatusRunning, StartedAt: time.Now(), LastActiveAt: time.Now()},
	}}
	uc := NewRecordAgentHookProviderSession(sessions)

	err := uc.Handle(context.Background(), "tenant-1", AgentHookEvent{WorktreeID: "wt-1", ProviderSessionKey: "session_id", ProviderSessionID: "provider-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessions.updateProviderSessionCall == nil || sessions.updateProviderSessionCall.ID != "provider-1" || sessions.updateProviderSessionCall.Key != "session_id" {
		t.Fatalf("expected UpdateProviderSession called with key=session_id id=provider-1, got %+v", sessions.updateProviderSessionCall)
	}
}
