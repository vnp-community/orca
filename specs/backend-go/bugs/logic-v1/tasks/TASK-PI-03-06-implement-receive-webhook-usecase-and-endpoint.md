# TASK-PI-03-06: `ReceiveWebhook` usecase + `api-gateway` HTTP endpoint

**From Solution:** SOL-PI-03
**Priority:** P1
**Service:** `scm-integration-service`, `api-gateway`
**File:** `backend-go/services/scm-integration-service/internal/usecase/receive_webhook.go` (new), `backend-go/services/scm-integration-service/internal/usecase/ports.go`, `backend-go/services/api-gateway/internal/adapter/httpgateway/scm_webhook_routes.go` (new)
**Depends on:** TASK-PI-03-01, TASK-PI-03-04
**Status:** `[ ]` TODO

---

## Context

BUG-PI-03: an externally-merged PR (merged directly on github.com) has no
webhook receiver, polling job, or event. `webhook_delivery_log`
(`scm-integration-service.md:155`) exists with no writer — this gives it
one. Only merge events are relevant to this flow; other event types are
recorded (idempotency) but do not publish anything.

## Changes to make

### `ports.go` — new ports

```go
// WebhookVerifier verifies a provider's webhook signature — GitHub's
// X-Hub-Signature-256 (HMAC-SHA256 over raw_body), GitLab's X-Gitlab-Token
// (constant-time string compare). Per-tenant secret resolution reuses
// CredentialResolver's existing per-tenant lookup.
type WebhookVerifier interface {
	Verify(ctx context.Context, provider domain.ScmProvider, rawBody []byte, signatureHeader string) bool
}

// WebhookDeliveryStore implements idempotent delivery tracking against
// scm.webhook_delivery_log's existing (provider, external_event_id)
// uniqueness — no schema change, only this task's first writer.
type WebhookDeliveryStore interface {
	Exists(ctx context.Context, provider domain.ScmProvider, deliveryID string) (bool, error)
	Record(ctx context.Context, provider domain.ScmProvider, deliveryID, status string) error
}

// OutboxEnqueuer — scm-integration-service's own outbox port (TASK-PI-03-04).
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, tenantID string, event domain.OutboxEvent) error
}
```

### `internal/usecase/receive_webhook.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ReceiveWebhookInput struct {
	Provider          domain.ScmProvider
	RawBody           []byte
	SignatureHeader   string
	DeliveryIDHeader  string
}

type ReceiveWebhookOutput struct {
	Accepted  bool
	Duplicate bool
}

type ReceiveWebhook struct {
	verifier   WebhookVerifier
	deliveries WebhookDeliveryStore
	outbox     OutboxEnqueuer
}

func NewReceiveWebhook(verifier WebhookVerifier, deliveries WebhookDeliveryStore, outbox OutboxEnqueuer) *ReceiveWebhook {
	return &ReceiveWebhook{verifier: verifier, deliveries: deliveries, outbox: outbox}
}

func (uc *ReceiveWebhook) Execute(ctx context.Context, in ReceiveWebhookInput) (ReceiveWebhookOutput, error) {
	if !uc.verifier.Verify(ctx, in.Provider, in.RawBody, in.SignatureHeader) {
		return ReceiveWebhookOutput{}, apperrors.New(apperrors.KindPermissionDenied, "SCM_WEBHOOK_BAD_SIGNATURE", "signature verification failed", nil)
	}
	if seen, err := uc.deliveries.Exists(ctx, in.Provider, in.DeliveryIDHeader); err == nil && seen {
		return ReceiveWebhookOutput{Accepted: true, Duplicate: true}, nil
	}

	parsed, isMerge := parseMergeEvent(in.Provider, in.RawBody) // only "PR/MR merged" events are relevant
	if err := uc.deliveries.Record(ctx, in.Provider, in.DeliveryIDHeader, "processed"); err != nil {
		return ReceiveWebhookOutput{}, apperrors.New(apperrors.KindInternal, "SCM_WEBHOOK_RECORD_FAILED", "failed to record webhook delivery", err)
	}
	if isMerge {
		event := prMergedEventFromWebhook(parsed)
		if err := uc.outbox.Enqueue(ctx, parsed.TenantID, event); err != nil {
			// Best-effort, same posture as CreatePullRequest's own enqueue
			// (TASK-PI-03-05) — a failed enqueue must not turn an already-
			// accepted, already-recorded webhook into a 5xx.
		}
	}
	return ReceiveWebhookOutput{Accepted: true}, nil
}
```

`parseMergeEvent` is a small per-provider parser (GitHub: `pull_request`
event with `action == "closed" && merged == true`; GitLab: `merge_request`
event with `object_attributes.action == "merge"`) — implement in a
`webhook_parse.go` sibling file.

### `api-gateway`: `POST /v1/scm/webhooks/{provider}` (new, unauthenticated route)

```go
// scm_webhook_routes.go — new
func (h *Handler) ReceiveScmWebhook(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	resp, err := h.scmClient.ReceiveWebhook(r.Context(), &scmintegrationv1.ReceiveWebhookRequest{
		Provider: provider, RawBody: body,
		SignatureHeader:  r.Header.Get("X-Hub-Signature-256"), // or X-Gitlab-Token for gitlab
		DeliveryIdHeader: r.Header.Get("X-GitHub-Delivery"),    // or X-Gitlab-Event-UUID for gitlab
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
```

Mount this route OUTSIDE the JWT-authenticated router group — webhooks
arrive from GitHub/GitLab's own servers, which never carry an Orca JWT
(mirror however this file's other unauthenticated routes, e.g. the OAuth
callback, are already mounted).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/scm-integration-service/... ./services/api-gateway/...
go vet ./services/scm-integration-service/... ./services/api-gateway/...
```
