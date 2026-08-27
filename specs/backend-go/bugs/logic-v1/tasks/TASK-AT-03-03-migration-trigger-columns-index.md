# TASK-AT-03-03: Migration — `trigger_type`/`trigger_event`/`trigger_filter_json` columns + partial index

**From Solution:** SOL-AT-03
**Priority:** P0
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/adapter/postgres/repository.go` (+ new migration file), `backend-go/services/automation-service/internal/usecase/ports.go`
**Depends on:** TASK-AT-03-02
**Status:** `[ ]` TODO

---

## Context

Persist the new trigger fields and add the partial index
`ListByTrigger` (TASK-AT-03-04) needs: `WHERE tenant_id = $1 AND
trigger_type = 'event' AND trigger_event = $2 AND enabled = true`.

## Changes to make

Add a new migration in the service's migrations directory:

```sql
ALTER TABLE automation.automations
  ADD COLUMN trigger_type TEXT NOT NULL DEFAULT 'cron',
  ADD COLUMN trigger_event TEXT,
  ADD COLUMN trigger_filter_json JSONB;

CREATE INDEX idx_automations_trigger ON automation.automations (tenant_id, trigger_type, trigger_event)
  WHERE trigger_type = 'event';
```

Update `repository.go`'s insert/select column lists (`Create`, `GetByID`,
`List`, `Update`) to read/write the three new columns, and add the new
repository method:

```go
// ListByTrigger returns enabled automations for tenantID whose trigger_type
// is 'event' and trigger_event matches eventName — backs event dispatch.
func (r *Repository) ListByTrigger(ctx context.Context, tenantID string, eventName domain.EventName) ([]domain.Automation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, ... FROM automation.automations
		WHERE tenant_id = $1 AND trigger_type = 'event' AND trigger_event = $2 AND enabled = true`,
		tenantID, string(eventName),
	)
	// ... scan into []domain.Automation, matching this file's existing scan helper pattern ...
}
```

Add `ListByTrigger` to the `AutomationRepository` port in `ports.go`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/adapter/postgres/... -run TestListByTrigger
```

Expected: migration applies cleanly; `ListByTrigger` returns only
enabled, matching-event automations for the given tenant, none from other
tenants or non-matching events.
