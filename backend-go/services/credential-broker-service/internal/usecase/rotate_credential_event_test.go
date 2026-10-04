package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// eventAuditRepo is an AuditRepository that also accepts outbox events, like the real tx-scoped repos.
type eventAuditRepo struct {
	*fakeAuditRepo
	events []domain.OutboxEvent
}

func (r *eventAuditRepo) EnqueueEvent(_ context.Context, e domain.OutboxEvent) error {
	r.events = append(r.events, e)
	return nil
}

type eventTxRunner struct {
	metadataRepo *fakeMetadataRepo
	auditRepo    *eventAuditRepo
}

func (f eventTxRunner) RunInTx(ctx context.Context, fn func(context.Context, CredentialMetadataRepository, AuditRepository) error) error {
	return fn(ctx, f.metadataRepo, f.auditRepo)
}

func TestRotateCredential_EnqueuesRotatedEventWithoutSecrets(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	store := newFakeSecretStore(rec)
	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryAiProviderKey, domain.StatusActive, "old-key")

	audit := &eventAuditRepo{fakeAuditRepo: newFakeAuditRepo(rec)}
	uc := NewRotateCredential(metadataRepo, store, eventTxRunner{metadataRepo, audit})
	if _, err := uc.Execute(context.Background(), RotateCredentialInput{
		CredentialID: m.ID, NewEncryptedEnvelope: []byte("brand-new-secret"), RequestingService: "ai-provider-service",
	}); err != nil {
		t.Fatal(err)
	}

	if len(audit.events) != 1 {
		t.Fatalf("events = %d, want 1", len(audit.events))
	}
	e := audit.events[0]
	if e.Subject != SubjectCredentialRotated || e.TenantID != m.TenantID || e.ID == "" {
		t.Fatalf("unexpected event %+v", e)
	}
	var p map[string]string
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		t.Fatal(err)
	}
	if p["credential_id"] != m.ID || p["category"] != string(m.Category) {
		t.Fatalf("payload %v", p)
	}
	if strings.Contains(string(e.Payload), "brand-new-secret") || strings.Contains(string(e.Payload), "old-key") {
		t.Fatal("payload must not contain secret material")
	}
}

func TestRotateCredential_NoEventWhenRepoHasNoOutbox(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)
	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryAiProviderKey, domain.StatusActive, "k")
	uc := NewRotateCredential(metadataRepo, store, newFakeTxRunner(rec, metadataRepo, auditRepo))
	if _, err := uc.Execute(context.Background(), RotateCredentialInput{CredentialID: m.ID, NewEncryptedEnvelope: []byte("n"), RequestingService: "ai-provider-service"}); err != nil {
		t.Fatalf("rotation must still succeed without an outbox: %v", err)
	}
}
