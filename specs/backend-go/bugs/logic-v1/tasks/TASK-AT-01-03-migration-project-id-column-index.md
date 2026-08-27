# TASK-AT-01-03: Migration — `project_id` column, actions storage, and per-project index

**From Solution:** SOL-AT-01
**Priority:** P0
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/adapter/postgres/repository.go` (+ new migration file)
**Depends on:** TASK-AT-01-02
**Status:** `[ ]` TODO

---

## Context

`repository.go`'s current schema/insert column list has no `project_id`
column at all. This task adds the column, a JSONB column to persist the
`Actions` chain, and the index BR-AT-02's per-project count query needs.

## Changes to make

Locate the existing migrations directory for `automation-service` (sibling
to `repository.go`, typically `internal/adapter/postgres/migrations/`) and
find the latest numbered migration to follow its naming convention. Add a
new migration:

```sql
ALTER TABLE automation.automations
  ADD COLUMN project_id UUID,
  ADD COLUMN actions_json JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX idx_automations_project ON automation.automations (tenant_id, project_id);
```

Update `repository.go`:
- Extend the insert/select column lists (`Create`, `GetByID`, `List`, etc.)
  to read/write `project_id` and `actions_json` (marshal `[]domain.AutomationAction`
  to/from JSON).
- Add the new repository method used by TASK-AT-01-04:

```go
// CountByProject returns the number of automations for tenantID scoped to
// projectID — backs BR-AT-02's per-project cap.
func (r *Repository) CountByProject(ctx context.Context, tenantID, projectID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM automation.automations WHERE tenant_id = $1 AND project_id = $2`,
		tenantID, projectID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
```

Add `CountByProject` to the `AutomationRepository` port in
`internal/usecase/ports.go`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/adapter/postgres/... -run TestRepository
```

Expected: migration applies cleanly against a fresh test database (however
this service's integration tests bootstrap Postgres — check
`repository_test.go`'s existing setup helper); `CountByProject` returns
correct counts scoped per-project, no cross-project leakage (add/extend a
test asserting this if the file's existing test suite doesn't already cover
it).
