# TASK-FLEET-03-06: `eventbus.HealthPublisher` — `dev_server.health_degraded`

**From Solution:** SOL-FLEET-03
**Priority:** P2
**Service:** `infra-fleet-service` (eventbus adapter)
**File:** `backend-go/services/infra-fleet-service/internal/adapter/eventbus/health_publisher.go` (new)
**Depends on:** TASK-FLEET-03-03
**Status:** `[ ]` TODO

---

## Context

`infra-fleet-service.md` §7 already lists `dev_server.health_degraded` as
an intended NATS JetStream event published via the transactional outbox
pattern — this is not a new integration point, it fills in a documented one
that had no writer to trigger it. Follows `backend-go/common/outbox`'s
existing package precedent (already used by `usage-service`/
`issue-tracking-service`) and `tenant-service`'s
`internal/adapter/eventbus/publisher.go` naming convention.

## Changes to make

```go
// internal/adapter/eventbus/health_publisher.go
package eventbus

type HealthPublisher struct {
    outbox *outbox.Outbox // backend-go/common/outbox
}

func NewHealthPublisher(outbox *outbox.Outbox) *HealthPublisher {
    return &HealthPublisher{outbox: outbox}
}

func (p *HealthPublisher) PublishStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus) {
    payload, _ := json.Marshal(map[string]any{
        "devServerId": ds.ID, "host": ds.Host, "tenantId": ds.TenantID,
        "from": from, "to": to, "timestamp": time.Now().UTC().Format(time.RFC3339),
    })
    if err := p.outbox.Enqueue(ctx, "dev_server.health_degraded", payload); err != nil {
        slog.Default().ErrorContext(ctx, "health_publisher: enqueue failed", slog.Any("error", err))
    }
}
```

Wire `usecase.HealthEventPublisher` to this type at bootstrap
(`cmd/server/main.go`), constructed with the service's existing
`outbox.Outbox` instance (already constructed for other event publishers in
this service — reuse it, do not create a second outbox).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/eventbus/... -run TestHealthPublisher -v
```

Expected: `PublishStatusChange` enqueues exactly one outbox row with event
type `dev_server.health_degraded` and the expected JSON payload shape.
