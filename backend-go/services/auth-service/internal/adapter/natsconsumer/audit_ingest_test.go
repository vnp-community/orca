package natsconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

// fakeAuditRepository is a minimal in-memory usecase.AuditRepository, local
// to this package (auth-service/internal/usecase's own fakeAuditRepository
// is unexported and lives in a _test.go file, unreachable from here).
type fakeAuditRepository struct {
	entries []domain.AuditEntry
}

func (f *fakeAuditRepository) Append(ctx context.Context, entry domain.AuditEntry) error {
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeAuditRepository) Query(ctx context.Context, filter usecase.AuditQueryFilter, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error) {
	return f.entries, "", nil
}

// TestAuditIngestConsumer_WellFormedEventAppendsAuditEntry is this task's
// Verify script's core assertion: a well-formed ssh.connect NATS message
// produces exactly one Append call with action "ssh.connect", target_type
// "ssh_host".
func TestAuditIngestConsumer_WellFormedEventAppendsAuditEntry(t *testing.T) {
	audit := &fakeAuditRepository{}
	consumer := New(usecase.NewHandleSSHConnectedEvent(audit), nil)

	occurredAt := time.Now().UTC()
	payload, err := json.Marshal(sshConnectedPayload{ActorUserID: "user-1", ConnectionID: "conn-1", Host: "10.0.0.9"})
	if err != nil {
		t.Fatalf("marshaling test payload: %v", err)
	}
	event := commoneventbus.Event{
		ID:         "evt-1",
		TenantID:   "t1",
		OccurredAt: occurredAt,
		Version:    1,
		Payload:    payload,
	}

	if err := consumer.handleEvent(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(audit.entries) != 1 {
		t.Fatalf("expected exactly 1 Append call, got %d", len(audit.entries))
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
}

// TestAuditIngestConsumer_MalformedEventIsDroppedNotFatal confirms a
// malformed message is dropped without panicking or blocking the consumer
// loop — no Append call, and handleEvent returns nil (Ack, not Nak/retry)
// since a poison message will never become well-formed on redelivery.
func TestAuditIngestConsumer_MalformedEventIsDroppedNotFatal(t *testing.T) {
	audit := &fakeAuditRepository{}
	consumer := New(usecase.NewHandleSSHConnectedEvent(audit), nil)

	event := commoneventbus.Event{
		ID:         "evt-2",
		TenantID:   "t1",
		OccurredAt: time.Now().UTC(),
		Version:    1,
		Payload:    []byte(`{not valid json`),
	}

	err := consumer.handleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("expected a malformed payload to be dropped (nil error), got: %v", err)
	}
	if len(audit.entries) != 0 {
		t.Errorf("expected no Append call for a malformed event, got %d", len(audit.entries))
	}
}

// TestAuditIngestConsumer_HandlerFailurePropagatesForRedelivery confirms a
// genuine downstream failure (not a malformed payload) is surfaced so
// common/eventbus's consume loop NAKs the message for retry.
func TestAuditIngestConsumer_HandlerFailurePropagatesForRedelivery(t *testing.T) {
	consumer := New(usecase.NewHandleSSHConnectedEvent(failingAuditRepository{}), nil)

	payload, _ := json.Marshal(sshConnectedPayload{ActorUserID: "user-1", ConnectionID: "conn-1", Host: "h1"})
	event := commoneventbus.Event{ID: "evt-3", TenantID: "t1", OccurredAt: time.Now().UTC(), Payload: payload}

	if err := consumer.handleEvent(context.Background(), event); err == nil {
		t.Fatal("expected the audit-append failure to propagate")
	}
}

type failingAuditRepository struct{}

func (failingAuditRepository) Append(ctx context.Context, entry domain.AuditEntry) error {
	return errors.New("db unavailable")
}

func (failingAuditRepository) Query(ctx context.Context, filter usecase.AuditQueryFilter, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error) {
	return nil, "", nil
}
