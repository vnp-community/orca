# TASK-FLEET-03-03: `FleetHealthWriter`/`PollLockPort`/`HealthEventPublisher`/`WebhookAlerter` ports + `ListAllForPolling`

**From Solution:** SOL-FLEET-03
**Priority:** P0
**Service:** `infra-fleet-service` (usecase ports)
**File:** `backend-go/services/infra-fleet-service/internal/usecase/ports.go`
**Depends on:** TASK-FLEET-03-01
**Status:** `[ ]` TODO

---

## Context

`PollFleetHealth` (TASK-FLEET-03-05) needs four new narrow ports plus one
new read method on `DevServerRepository` — defining all of them first lets
the writer, postgres, eventbus, and webhook tasks proceed independently
against a stable interface.

## Changes to make

```go
// internal/usecase/ports.go

// FleetHealthPort (existing, read side) is unchanged.

// FleetHealthWriter is the write side PollFleetHealth needs — split from
// FleetHealthPort the same way other narrow ports already split a single
// Repository's read/write concerns in this file.
type FleetHealthWriter interface {
    UpsertFleetHealth(ctx context.Context, sample domain.DevServerHealth) error
    GetPrevious(ctx context.Context, devServerID string) (sample domain.DevServerHealth, found bool, err error)
}

// PollLockPort wraps a Postgres session-level advisory lock keyed by a
// hash of devServerID — TryLock is non-blocking (pg_try_advisory_lock, not
// pg_advisory_lock): a replica that loses the race skips this server this
// tick rather than queueing.
type PollLockPort interface {
    TryLock(ctx context.Context, devServerID string) (locked bool, unlock func(), err error)
}

type HealthEventPublisher interface {
    PublishStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus)
}
type WebhookAlerter interface {
    NotifyStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus, sample domain.DevServerHealth)
}
```

`DevServerRepository` gains one more read method:

```go
// ListAllForPolling is cross-tenant by design (the poller is not
// answering one tenant's request), unlike every other DevServerRepository
// method's tenantID parameter.
ListAllForPolling(ctx context.Context) ([]domain.DevServer, error)
```

Update any existing fake implementing `DevServerRepository` in
`internal/usecase/*_test.go` fakes to add `ListAllForPolling`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
```

Expected: clean build (interfaces only — implementations land in
subsequent tasks; a build failure here means a call site elsewhere in the
package implements `DevServerRepository`/`FleetHealthPort` and needs the
new method stubbed).
