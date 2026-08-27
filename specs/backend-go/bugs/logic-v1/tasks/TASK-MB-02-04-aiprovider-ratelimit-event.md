# TASK-MB-02-04: Publish `orca.aiprovider.account.rate_limited` when a provider connection test reports a rate limit

**From Solution:** SOL-MB-02
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/usecase/test_connection.go`, `backend-go/services/ai-provider-service/internal/adapter/eventbus/publisher.go` (new)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

**Confirmed gap, larger than SOL-MB-02 assumed**: `ai-provider-service`
makes NO direct HTTP calls to any AI provider itself — `TestConnection`
(`internal/usecase/test_connection.go`) relays through `infra.Relay(ctx,
devServerID, "ai.testProviderConnection", ...)`, whose own doc comment
says the agent-side method "does not exist yet... this call is inert until
the agent implements it." There is no "existing error-mapping path" that
classifies a provider response as rate-limited anywhere in this service's
Go code (confirmed: no `429`/`RateLimit` hits in
`services/ai-provider-service/internal`). This task wires the REAL,
available integration point — `TestConnection`'s relay-result parsing —
so that once the agent's `ai.testProviderConnection` method (already a
separately tracked gap, not this task's to close) reports a rate-limit
signal in its result map, this service publishes the event
notification-service needs. This is honest, real backend-go work: it does
not fabricate a provider HTTP call path that doesn't exist.

## Changes to make

`backend-go/services/ai-provider-service/internal/adapter/eventbus/publisher.go`:

```go
// Package eventbus implements usecase.RateLimitEventPublisher against NATS
// JetStream via common/eventbus — mirrors tenant-service's
// internal/adapter/eventbus/publisher.go shape.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

const SubjectRateLimited = "orca.aiprovider.account.rate_limited"

type RateLimitPayload struct {
	AccountID string `json:"account_id"`
	Provider  string `json:"provider"`
	UserID    string `json:"user_id"`
	ResetAt   *int64 `json:"reset_at_unix_ms,omitempty"`
}

type Publisher struct{ pub *commoneventbus.Publisher }

func New(pub *commoneventbus.Publisher) *Publisher { return &Publisher{pub: pub} }

func (p *Publisher) PublishRateLimited(ctx context.Context, tenantID string, payload RateLimitPayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("eventbus: marshal rate-limit payload: %w", err)
	}
	return p.pub.Publish(ctx, SubjectRateLimited, commoneventbus.Event{
		ID: uuid.NewString(), TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: raw,
	})
}
```

In `internal/usecase/ports.go`, add:

```go
type RateLimitEventPublisher interface {
	PublishRateLimited(ctx context.Context, tenantID string, payload eventbus.RateLimitPayload) error
}
```

In `test_connection.go`, extend `parseConnectionTestResult` and `Execute`:

```go
type ConnectionTestResult struct {
	Success     bool
	Message     string
	RateLimited bool   // new
	ResetAtMs   *int64 // new
}

func parseConnectionTestResult(result map[string]any) ConnectionTestResult {
	out := ConnectionTestResult{}
	if v, ok := result["success"].(bool); ok {
		out.Success = v
	}
	if v, ok := result["message"].(string); ok {
		out.Message = v
	}
	if v, ok := result["rateLimited"].(bool); ok {
		out.RateLimited = v
	}
	if v, ok := result["resetAtUnixMs"].(float64); ok {
		ms := int64(v)
		out.ResetAtMs = &ms
	}
	return out
}
```

At the end of `TestConnection.Execute`, after `parseConnectionTestResult`:

```go
parsed := parseConnectionTestResult(result)
if parsed.RateLimited {
	userID := tenant.UserIDFromContext(ctx) // adjust to this repo's actual accessor
	_ = uc.rateLimitEvents.PublishRateLimited(ctx, tenantID, eventbus.RateLimitPayload{
		AccountID: in.AccountID, Provider: string(account.ProviderType), UserID: userID, ResetAt: parsed.ResetAtMs,
	}) // best-effort — a publish failure must not fail the connection-test result itself
}
return parsed, nil
```

Add `rateLimitEvents RateLimitEventPublisher` to `TestConnection`'s
constructor, wired from `cmd/server/main.go`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/ai-provider-service/... && go vet ./services/ai-provider-service/...
go test ./services/ai-provider-service/internal/usecase/... -run TestConnection
```

Test: relay result `{"rateLimited": true, "resetAtUnixMs": 123}` → fake
`RateLimitEventPublisher.PublishRateLimited` called once with matching
payload; `{"success": true}` (no `rateLimited` key) → publisher NOT called.
