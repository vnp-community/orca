package usecase

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHandleSSHConnectedEvent_AppendsAuditEntry(t *testing.T) {
	audit := &fakeAuditRepository{}
	uc := NewHandleSSHConnectedEvent(audit)

	occurredAt := time.Now().UTC()
	err := uc.Execute(context.Background(), HandleSSHConnectedEventInput{
		TenantID:     "t1",
		ActorUserID:  "user-1",
		ConnectionID: "conn-1",
		Host:         "10.0.0.9",
		OccurredAt:   occurredAt,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(audit.entries) != 1 {
		t.Fatalf("expected exactly 1 audit entry, got %d", len(audit.entries))
	}
	entry := audit.entries[0]
	if entry.Action != "ssh.connect" {
		t.Errorf("got action %q, want %q", entry.Action, "ssh.connect")
	}
	if entry.TargetType != "ssh_host" {
		t.Errorf("got target_type %q, want %q", entry.TargetType, "ssh_host")
	}
	if entry.TargetID != "10.0.0.9" {
		t.Errorf("got target_id %q, want %q", entry.TargetID, "10.0.0.9")
	}
	if entry.TenantID != "t1" {
		t.Errorf("got tenant_id %q, want %q", entry.TenantID, "t1")
	}
	if entry.ActorID != "user-1" {
		t.Errorf("got actor_id %q, want %q", entry.ActorID, "user-1")
	}
	if entry.Metadata["connectionId"] != "conn-1" {
		t.Errorf("got metadata connectionId %v, want %q", entry.Metadata["connectionId"], "conn-1")
	}
	if !entry.OccurredAt.Equal(occurredAt) {
		t.Errorf("got occurred_at %v, want %v", entry.OccurredAt, occurredAt)
	}
}

func TestHandleSSHConnectedEvent_EmptyActorUserIDIsAllowed(t *testing.T) {
	audit := &fakeAuditRepository{}
	uc := NewHandleSSHConnectedEvent(audit)

	err := uc.Execute(context.Background(), HandleSSHConnectedEventInput{
		TenantID:     "t1",
		ConnectionID: "conn-1",
		Host:         "h1",
		OccurredAt:   time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error for an empty actor user id: %v", err)
	}
	if len(audit.entries) != 1 || audit.entries[0].ActorID != "" {
		t.Errorf("expected 1 entry with an empty ActorID, got %+v", audit.entries)
	}
}

func TestHandleSSHConnectedEvent_MissingTenantIDFails(t *testing.T) {
	audit := &fakeAuditRepository{}
	uc := NewHandleSSHConnectedEvent(audit)

	err := uc.Execute(context.Background(), HandleSSHConnectedEventInput{
		ConnectionID: "conn-1",
		Host:         "h1",
		OccurredAt:   time.Now(),
	})
	if err == nil {
		t.Fatal("expected an error when tenant_id is empty")
	}
	if len(audit.entries) != 0 {
		t.Error("expected no audit entry to be appended for an invalid event")
	}
}

func TestHandleSSHConnectedEvent_AuditRepositoryFailurePropagates(t *testing.T) {
	audit := &fakeAuditRepository{appendErr: errors.New("db unavailable")}
	uc := NewHandleSSHConnectedEvent(audit)

	err := uc.Execute(context.Background(), HandleSSHConnectedEventInput{
		TenantID:     "t1",
		ConnectionID: "conn-1",
		Host:         "h1",
		OccurredAt:   time.Now(),
	})
	if err == nil {
		t.Fatal("expected error to propagate from AuditRepository.Append failure")
	}
}
