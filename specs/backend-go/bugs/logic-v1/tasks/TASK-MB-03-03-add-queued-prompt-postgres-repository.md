# TASK-MB-03-03: Add `infra.queued_prompts` table + repository

**From Solution:** SOL-MB-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/migrations/0007_queued_prompts.up.sql`, `backend-go/services/infra-fleet-service/internal/adapter/postgres/queued_prompt_repository.go`
**Depends on:** TASK-MB-03-02
**Status:** [x] DONE — migration `0008_queued_prompts` (renumbered from the spec's 0007 since 0007 was already taken by `terminal_session_created_by`; `pty_id` typed TEXT to match `terminal_sessions.pty_id`'s real type, not UUID) + `QueuedPromptStore` (Get/Upsert/Delete/GetAndDelete) + `QueuedPromptRepository` port added; `go build`/`go vet` pass.

---

## Context

A queued prompt must survive until the agent becomes ready, which (per
this service's own per-pod connection-ownership caveat) could outlast the
pod that received the original `DispatchPrompt` call — durable Postgres
storage, not the in-memory quiescence registry SOL-MB-02 uses. One row per
`pty_id` enforces BR-MB-12's "overwrite requires confirmation" rule: a
second insert without `overwrite=true` must be rejected by the usecase
before ever reaching this table.

## Changes to make

`backend-go/services/infra-fleet-service/migrations/0007_queued_prompts.up.sql`:

```sql
CREATE TABLE infra.queued_prompts (
    pty_id                   UUID PRIMARY KEY REFERENCES infra.terminal_sessions(pty_id) ON DELETE CASCADE,
    tenant_id                UUID NOT NULL,
    prompt                   TEXT NOT NULL,
    dispatched_by_device_id  UUID,
    queued_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`.down.sql`: `DROP TABLE IF EXISTS infra.queued_prompts;`

`backend-go/services/infra-fleet-service/internal/adapter/postgres/queued_prompt_repository.go`:

```go
package postgres

type QueuedPromptStore struct{ pool *pgxpool.Pool }

func NewQueuedPromptStore(pool *pgxpool.Pool) *QueuedPromptStore { return &QueuedPromptStore{pool: pool} }

func (s *QueuedPromptStore) Get(ctx context.Context, ptyID string) (domain.QueuedPrompt, bool, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT pty_id, tenant_id, prompt, dispatched_by_device_id, queued_at
		FROM infra.queued_prompts WHERE pty_id = $1`, ptyID)
	var p domain.QueuedPrompt
	err := row.Scan(&p.PtyID, &p.TenantID, &p.Prompt, &p.DispatchedByDeviceID, &p.QueuedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.QueuedPrompt{}, false, nil
	}
	return p, err == nil, err
}

func (s *QueuedPromptStore) Upsert(ctx context.Context, p domain.QueuedPrompt) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.queued_prompts (pty_id, tenant_id, prompt, dispatched_by_device_id, queued_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (pty_id) DO UPDATE SET prompt=$3, dispatched_by_device_id=$4, queued_at=$5`,
		p.PtyID, p.TenantID, p.Prompt, p.DispatchedByDeviceID, p.QueuedAt)
	return err
}

func (s *QueuedPromptStore) Delete(ctx context.Context, ptyID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM infra.queued_prompts WHERE pty_id = $1`, ptyID)
	return err
}

// GetAndDelete atomically reads and removes the row — the regression guard
// TASK-MB-03-04's queue-drain hook needs against a double-delivery race
// between the AttachPty ready-transition hook and a concurrent
// DispatchPrompt call.
func (s *QueuedPromptStore) GetAndDelete(ctx context.Context, ptyID string) (domain.QueuedPrompt, bool, error) {
	row := s.pool.QueryRow(ctx, `
		DELETE FROM infra.queued_prompts WHERE pty_id = $1
		RETURNING pty_id, tenant_id, prompt, dispatched_by_device_id, queued_at`, ptyID)
	var p domain.QueuedPrompt
	err := row.Scan(&p.PtyID, &p.TenantID, &p.Prompt, &p.DispatchedByDeviceID, &p.QueuedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.QueuedPrompt{}, false, nil
	}
	return p, err == nil, err
}
```

Add `QueuedPromptRepository` port to `usecase/ports.go`:

```go
type QueuedPromptRepository interface {
	Get(ctx context.Context, ptyID string) (domain.QueuedPrompt, bool, error)
	Upsert(ctx context.Context, p domain.QueuedPrompt) error
	Delete(ctx context.Context, ptyID string) error
	GetAndDelete(ctx context.Context, ptyID string) (domain.QueuedPrompt, bool, error)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/... && go vet ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/postgres/... -run QueuedPrompt
```
