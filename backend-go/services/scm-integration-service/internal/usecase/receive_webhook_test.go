package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// fakeWebhookVerifier is an in-memory WebhookVerifier.
type fakeWebhookVerifier struct {
	valid bool
	calls int
}

func (f *fakeWebhookVerifier) Verify(_ context.Context, _ domain.ScmProvider, _ []byte, _ string) bool {
	f.calls++
	return f.valid
}

// fakeWebhookDeliveryStore is an in-memory WebhookDeliveryStore.
type fakeWebhookDeliveryStore struct {
	exists    bool
	existsErr error
	recordErr error

	existsCalls int
	recordCalls int
	lastStatus  string
}

func (f *fakeWebhookDeliveryStore) Exists(_ context.Context, _ domain.ScmProvider, _ string) (bool, error) {
	f.existsCalls++
	if f.existsErr != nil {
		return false, f.existsErr
	}
	return f.exists, nil
}

func (f *fakeWebhookDeliveryStore) Record(_ context.Context, _ domain.ScmProvider, _, status string) error {
	f.recordCalls++
	f.lastStatus = status
	return f.recordErr
}

// fakeOutboxEnqueuer is an in-memory OutboxEnqueuer.
type fakeOutboxEnqueuer struct {
	err        error
	calls      int
	lastEvent  domain.OutboxEvent
	lastTenant string
}

func (f *fakeOutboxEnqueuer) Enqueue(_ context.Context, tenantID string, event domain.OutboxEvent) error {
	f.calls++
	f.lastTenant, f.lastEvent = tenantID, event
	return f.err
}

func githubMergedPRPayload(repo string, number int32) []byte {
	payload, _ := json.Marshal(map[string]any{
		"action": "closed",
		"number": number,
		"pull_request": map[string]any{
			"number": number,
			"merged": true,
			"base":   map[string]any{"repo": map[string]any{"full_name": repo}},
		},
	})
	return payload
}

func githubOpenedPRPayload(repo string, number int32) []byte {
	payload, _ := json.Marshal(map[string]any{
		"action": "opened",
		"number": number,
		"pull_request": map[string]any{
			"number": number,
			"merged": false,
			"base":   map[string]any{"repo": map[string]any{"full_name": repo}},
		},
	})
	return payload
}

func TestReceiveWebhook_BadSignatureRejectedBeforeDedupCheck(t *testing.T) {
	verifier := &fakeWebhookVerifier{valid: false}
	deliveries := &fakeWebhookDeliveryStore{}
	outbox := &fakeOutboxEnqueuer{}
	uc := NewReceiveWebhook(verifier, deliveries, outbox)

	_, err := uc.Execute(context.Background(), ReceiveWebhookInput{
		Provider: domain.ScmProviderGitHub, RawBody: githubMergedPRPayload("o/r", 42),
		SignatureHeader: "sha256=bad", DeliveryIDHeader: "delivery-1",
	})
	if err == nil {
		t.Fatal("expected a bad-signature error")
	}
	if verifier.calls != 1 {
		t.Fatalf("expected exactly one verification attempt, got %d", verifier.calls)
	}
	if deliveries.existsCalls != 0 {
		t.Fatalf("expected no dedup check for a rejected signature, got %d Exists calls", deliveries.existsCalls)
	}
	if deliveries.recordCalls != 0 {
		t.Fatalf("expected no delivery record for a rejected signature, got %d Record calls", deliveries.recordCalls)
	}
	if outbox.calls != 0 {
		t.Fatalf("expected no outbox enqueue for a rejected signature, got %d calls", outbox.calls)
	}
}

func TestReceiveWebhook_DuplicateDeliveryReturnsDuplicateWithoutSecondEnqueue(t *testing.T) {
	verifier := &fakeWebhookVerifier{valid: true}
	deliveries := &fakeWebhookDeliveryStore{exists: true}
	outbox := &fakeOutboxEnqueuer{}
	uc := NewReceiveWebhook(verifier, deliveries, outbox)

	out, err := uc.Execute(context.Background(), ReceiveWebhookInput{
		Provider: domain.ScmProviderGitHub, RawBody: githubMergedPRPayload("o/r", 42),
		SignatureHeader: "sha256=whatever", DeliveryIDHeader: "delivery-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Accepted || !out.Duplicate {
		t.Fatalf("expected Accepted=true Duplicate=true, got %+v", out)
	}
	if deliveries.recordCalls != 0 {
		t.Fatalf("expected no second Record call for an already-seen delivery, got %d", deliveries.recordCalls)
	}
	if outbox.calls != 0 {
		t.Fatalf("expected no outbox enqueue for a duplicate delivery, got %d calls", outbox.calls)
	}
}

func TestReceiveWebhook_NonMergeEventRecordedButNotEnqueued(t *testing.T) {
	verifier := &fakeWebhookVerifier{valid: true}
	deliveries := &fakeWebhookDeliveryStore{exists: false}
	outbox := &fakeOutboxEnqueuer{}
	uc := NewReceiveWebhook(verifier, deliveries, outbox)

	out, err := uc.Execute(context.Background(), ReceiveWebhookInput{
		Provider: domain.ScmProviderGitHub, RawBody: githubOpenedPRPayload("o/r", 42),
		SignatureHeader: "sha256=whatever", DeliveryIDHeader: "delivery-2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Accepted || out.Duplicate {
		t.Fatalf("expected Accepted=true Duplicate=false, got %+v", out)
	}
	if deliveries.recordCalls != 1 || deliveries.lastStatus != "processed" {
		t.Fatalf("expected the delivery to be recorded once as processed, got calls=%d status=%q", deliveries.recordCalls, deliveries.lastStatus)
	}
	if outbox.calls != 0 {
		t.Fatalf("expected no outbox enqueue for a non-merge event, got %d calls", outbox.calls)
	}
}

func TestReceiveWebhook_MergeEventEnqueuesLifecycleEvent(t *testing.T) {
	verifier := &fakeWebhookVerifier{valid: true}
	deliveries := &fakeWebhookDeliveryStore{exists: false}
	outbox := &fakeOutboxEnqueuer{}
	uc := NewReceiveWebhook(verifier, deliveries, outbox)

	out, err := uc.Execute(context.Background(), ReceiveWebhookInput{
		Provider: domain.ScmProviderGitHub, RawBody: githubMergedPRPayload("o/r", 42),
		SignatureHeader: "sha256=whatever", DeliveryIDHeader: "delivery-3",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Accepted || out.Duplicate {
		t.Fatalf("expected Accepted=true Duplicate=false, got %+v", out)
	}
	if deliveries.recordCalls != 1 {
		t.Fatalf("expected the delivery to be recorded, got %d calls", deliveries.recordCalls)
	}
	if outbox.calls != 1 {
		t.Fatalf("expected exactly one outbox enqueue for a merge event, got %d calls", outbox.calls)
	}
	if outbox.lastEvent.Subject != subjectPullRequestMerged {
		t.Fatalf("expected subject %q, got %q", subjectPullRequestMerged, outbox.lastEvent.Subject)
	}
	var payload prLifecycleEventPayload
	if err := json.Unmarshal(outbox.lastEvent.PayloadJSON, &payload); err != nil {
		t.Fatalf("failed to unmarshal enqueued payload: %v", err)
	}
	if payload.Repo != "o/r" || payload.PrNumber != 42 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestReceiveWebhook_RecordFailureFailsExecute(t *testing.T) {
	verifier := &fakeWebhookVerifier{valid: true}
	deliveries := &fakeWebhookDeliveryStore{recordErr: context.DeadlineExceeded}
	outbox := &fakeOutboxEnqueuer{}
	uc := NewReceiveWebhook(verifier, deliveries, outbox)

	_, err := uc.Execute(context.Background(), ReceiveWebhookInput{
		Provider: domain.ScmProviderGitHub, RawBody: githubMergedPRPayload("o/r", 42),
		SignatureHeader: "sha256=whatever", DeliveryIDHeader: "delivery-4",
	})
	if err == nil {
		t.Fatal("expected a failed delivery record to fail Execute")
	}
	if outbox.calls != 0 {
		t.Fatalf("expected no outbox enqueue when the delivery record itself failed, got %d calls", outbox.calls)
	}
}

func TestReceiveWebhook_OutboxEnqueueFailureDoesNotFailExecute(t *testing.T) {
	verifier := &fakeWebhookVerifier{valid: true}
	deliveries := &fakeWebhookDeliveryStore{exists: false}
	outbox := &fakeOutboxEnqueuer{err: context.DeadlineExceeded}
	uc := NewReceiveWebhook(verifier, deliveries, outbox)

	out, err := uc.Execute(context.Background(), ReceiveWebhookInput{
		Provider: domain.ScmProviderGitHub, RawBody: githubMergedPRPayload("o/r", 42),
		SignatureHeader: "sha256=whatever", DeliveryIDHeader: "delivery-5",
	})
	if err != nil {
		t.Fatalf("expected a failed outbox enqueue to be best-effort, not fail Execute: %v", err)
	}
	if !out.Accepted {
		t.Fatal("expected Accepted=true despite the enqueue failure")
	}
}
