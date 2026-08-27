# TASK-PRF-01-04: Add `AuditPublisher` port and `PublishAuditEvent` to tenant-service's eventbus adapter

**From Solution:** SOL-PRF-01
**Priority:** P1
**Service:** `tenant-service`
**File:** `backend-go/services/tenant-service/internal/adapter/eventbus/publisher.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BUG-PRF-01 found zero audit-logging call sites in `tenant-service` despite
BL-PRF-01 requiring an audit entry on company/department mutations.
`auth-service` owns the system-wide audit log but exposes no write RPC
(only `QueryAuditLog`), and cross-database writes are forbidden
(`05-data-architecture.md`), so per `07-security-architecture.md`'s outbox
pattern this publishes an event rather than calling another service
synchronously — the same best-effort shape `PublishProfileInvalidated`
already uses in this same file.

## Changes to make

In `backend-go/services/tenant-service/internal/adapter/eventbus/publisher.go`,
add alongside the existing `Subject`/`Publisher`/`PublishProfileInvalidated`:

```go
// AuditSubject is the event subject UpdateCompany/UpdateDepartment/
// CreateDepartment publish to after a successful write. Something must
// consume this and append into auth.audit_log for these events to reach
// QueryAuditLog — that consumer is auth-service's own follow-up work (it
// already owns writes to that table), not built by this task.
const AuditSubject = "orca.tenant.audit.recorded"

type auditPayload struct {
	Action  string `json:"action"`
	ActorID string `json:"actor_id"`
	Target  string `json:"target"`
}

// PublishAuditEvent implements usecase.AuditPublisher — best-effort, same
// posture as PublishProfileInvalidated above (a missed publish degrades the
// audit trail, it never blocks or fails the write it's reporting on).
func (p *Publisher) PublishAuditEvent(ctx context.Context, tenantID, actorID, action, target string) error {
	payload, err := json.Marshal(auditPayload{Action: action, ActorID: actorID, Target: target})
	if err != nil {
		return fmt.Errorf("eventbus: marshal audit payload: %w", err)
	}
	return p.pub.Publish(ctx, AuditSubject, commoneventbus.Event{
		ID:         uuid.NewString(),
		TenantID:   tenantID,
		OccurredAt: time.Now().UTC(),
		Version:    1,
		Payload:    payload,
	})
}
```

Add to `backend-go/services/tenant-service/internal/usecase/ports.go`:

```go
// AuditPublisher is the outbound port UpdateCompany/UpdateDepartment/
// CreateDepartment call after a successful write to emit a security-relevant
// audit event — outbox pattern, not a synchronous call to auth-service (see
// internal/adapter/eventbus.Publisher.PublishAuditEvent's doc comment). A
// nil AuditPublisher is valid — callers must nil-check, same convention as
// CacheInvalidationPublisher above.
type AuditPublisher interface {
	PublishAuditEvent(ctx context.Context, tenantID, actorID, action, target string) error
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/tenant-service/...
go test ./services/tenant-service/internal/adapter/eventbus/... -v
```

Add a `publisher_test.go` case asserting `PublishAuditEvent` marshals the
expected payload shape and calls `Publish` with `AuditSubject`.
