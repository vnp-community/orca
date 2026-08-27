package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

func newReconcileTestAccount(id string, status domain.AccountStatus, healthDetail *string) domain.ProviderAccount {
	return domain.ProviderAccount{
		ID: id, TenantID: "tenant-1", ProviderType: domain.ProviderTypeAnthropic,
		Status: status, HealthDetail: healthDetail, CredentialRef: "cred-" + id, DevServerID: "dev-1",
	}
}

func strptr(s string) *string { return &s }

func TestReconcileProviderHealth_ClassifiesUnreachable(t *testing.T) {
	account := newReconcileTestAccount("acc-1", domain.AccountStatusActive, strptr(domain.HealthDetailHealthy))
	batch := &fakeHealthCheckBatch{accounts: []domain.ProviderAccount{account}}
	claimer := &fakeHealthCheckClaimer{batch: batch}
	infra := &fakeInfraFleetClient{relayErr: errBoom}
	outboxFake := &fakeOutboxEnqueuer{}
	uc := NewReconcileProviderHealth(claimer, infra, outboxFake, nil)

	if err := uc.Execute(context.Background(), 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batch.recordCalls) != 1 {
		t.Fatalf("expected exactly one RecordResult call, got %d", len(batch.recordCalls))
	}
	rec := batch.recordCalls[0]
	if rec.Status != domain.AccountStatusError || rec.HealthDetailOrEmpty() != domain.HealthDetailUnreachable {
		t.Errorf("expected status=error/detail=unreachable, got status=%s detail=%s", rec.Status, rec.HealthDetailOrEmpty())
	}
	if len(outboxFake.enqueued) != 1 {
		t.Errorf("expected one outbox event for the new failure classification, got %d", len(outboxFake.enqueued))
	}
	if !batch.committed {
		t.Error("expected the claim batch to be committed")
	}
}

func TestReconcileProviderHealth_ClassifiesInvalidKey(t *testing.T) {
	account := newReconcileTestAccount("acc-1", domain.AccountStatusActive, strptr(domain.HealthDetailHealthy))
	batch := &fakeHealthCheckBatch{accounts: []domain.ProviderAccount{account}}
	claimer := &fakeHealthCheckClaimer{batch: batch}
	infra := &fakeInfraFleetClient{relayResult: map[string]any{"success": false, "message": "bad key"}}
	outboxFake := &fakeOutboxEnqueuer{}
	uc := NewReconcileProviderHealth(claimer, infra, outboxFake, nil)

	if err := uc.Execute(context.Background(), 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rec := batch.recordCalls[0]
	if rec.Status != domain.AccountStatusError || rec.HealthDetailOrEmpty() != domain.HealthDetailInvalidKey {
		t.Errorf("expected status=error/detail=invalid_key, got status=%s detail=%s", rec.Status, rec.HealthDetailOrEmpty())
	}
}

func TestReconcileProviderHealth_NoAlertOnRepeatFailure(t *testing.T) {
	account := newReconcileTestAccount("acc-1", domain.AccountStatusError, strptr(domain.HealthDetailUnreachable))
	batch := &fakeHealthCheckBatch{accounts: []domain.ProviderAccount{account}}
	claimer := &fakeHealthCheckClaimer{batch: batch}
	infra := &fakeInfraFleetClient{relayErr: errBoom}
	outboxFake := &fakeOutboxEnqueuer{}
	uc := NewReconcileProviderHealth(claimer, infra, outboxFake, nil)

	if err := uc.Execute(context.Background(), 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outboxFake.enqueued) != 0 {
		t.Errorf("expected no outbox event for a repeat failure classification, got %d", len(outboxFake.enqueued))
	}
}

func TestReconcileProviderHealth_QuotaExceededSurvivesHealthyPing(t *testing.T) {
	account := newReconcileTestAccount("acc-1", domain.AccountStatusError, strptr(domain.HealthDetailQuotaExceeded))
	batch := &fakeHealthCheckBatch{accounts: []domain.ProviderAccount{account}}
	claimer := &fakeHealthCheckClaimer{batch: batch}
	infra := &fakeInfraFleetClient{relayResult: map[string]any{"success": true, "message": "reachable"}}
	outboxFake := &fakeOutboxEnqueuer{}
	uc := NewReconcileProviderHealth(claimer, infra, outboxFake, nil)

	if err := uc.Execute(context.Background(), 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rec := batch.recordCalls[0]
	if rec.Status != domain.AccountStatusError || rec.HealthDetailOrEmpty() != domain.HealthDetailQuotaExceeded {
		t.Errorf("expected quota_exceeded to survive a healthy ping, got status=%s detail=%s", rec.Status, rec.HealthDetailOrEmpty())
	}
}

func TestReconcileProviderHealth_LatencyRecorded(t *testing.T) {
	account := newReconcileTestAccount("acc-1", domain.AccountStatusActive, strptr(domain.HealthDetailHealthy))
	batch := &fakeHealthCheckBatch{accounts: []domain.ProviderAccount{account}}
	claimer := &fakeHealthCheckClaimer{batch: batch}
	infra := &fakeInfraFleetClient{relayResult: map[string]any{"success": true, "message": "reachable"}}
	outboxFake := &fakeOutboxEnqueuer{}
	uc := NewReconcileProviderHealth(claimer, infra, outboxFake, nil)

	if err := uc.Execute(context.Background(), 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rec := batch.recordCalls[0]
	if rec.LatencyMs == nil {
		t.Error("expected a non-nil latency_ms on a successful check")
	}
}

func TestReconcileProviderHealth_ClaimErrorPropagates(t *testing.T) {
	claimer := &fakeHealthCheckClaimer{claimErr: errBoom}
	uc := NewReconcileProviderHealth(claimer, &fakeInfraFleetClient{}, &fakeOutboxEnqueuer{}, func() time.Time { return time.Now() })
	if err := uc.Execute(context.Background(), 10); err == nil {
		t.Fatal("expected the claim error to propagate")
	}
}
