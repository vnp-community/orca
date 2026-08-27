# TASK-TM-03-01: Add `terminal_scrollback_snapshots` migration

**From Solution:** SOL-TM-03
**Priority:** P0 — every later task in this set reads/writes this table
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/migrations/0009_terminal_scrollback_snapshots.up.sql`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

SOL-TM-03 stores client-serialized (xterm.js `@xterm/addon-serialize`) terminal
scrollback keyed by `(tenant_id, worktree_id, pane_key)` — not `worktree_id`
alone, since one worktree can have multiple panes (F02 splits). This task
adds the table, its indexes (one for worktree lookup, one to back the
BR-TM-12 expiry sweep), and RLS tenant isolation, following the existing
`infra` schema migration numbering (`0008` is the latest existing migration
in this service).

## Changes to make

Check the latest existing migration number first — bump `0009` if a later
one already exists:

```bash
ls /opt/repos/orca/backend-go/services/infra-fleet-service/migrations | tail -5
```

Create `backend-go/services/infra-fleet-service/migrations/0009_terminal_scrollback_snapshots.up.sql`:

```sql
-- One row per (tenant, worktree, pane) — pane_key, not worktree_id alone,
-- because one worktree can have multiple simultaneous panes (F02 splits).
-- data_gzip is the CLIENT-produced @xterm/addon-serialize ANSI blob,
-- gzip-compressed; this service never parses it (see SOL-TM-03's
-- "serialize/deserialize is a client concern" rationale). Cursor
-- position/text attributes (BR-TM-11) are encoded INLINE in that ANSI blob
-- by the serializer itself — no separate cursor_row/cursor_col columns.
CREATE TABLE infra.terminal_scrollback_snapshots (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    worktree_id         UUID NOT NULL,
    pane_key            TEXT NOT NULL,
    cols                INTEGER NOT NULL,
    rows                INTEGER NOT NULL,
    data_gzip           BYTEA NOT NULL,
    uncompressed_bytes  INTEGER NOT NULL,  -- pre-compression size, for BR-TM-10's cap check
    last_title          TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, worktree_id, pane_key)
);

CREATE INDEX idx_infra_scrollback_snapshots_worktree
    ON infra.terminal_scrollback_snapshots (tenant_id, worktree_id);
-- Backs BR-TM-12's expiry sweep (updated_at-based proxy for "not opened" —
-- see SOL-TM-03's BR-TM-12 caveat).
CREATE INDEX idx_infra_scrollback_snapshots_updated_at
    ON infra.terminal_scrollback_snapshots (updated_at);

ALTER TABLE infra.terminal_scrollback_snapshots ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.terminal_scrollback_snapshots
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

Create the matching `backend-go/services/infra-fleet-service/migrations/0009_terminal_scrollback_snapshots.down.sql`:

```sql
DROP TABLE IF EXISTS infra.terminal_scrollback_snapshots;
```

## Verify

```bash
cd /opt/repos/orca/backend-go
# golang-migrate must accept the pair without error (adjust DSN/env to match
# this service's existing migration-test tooling if it differs):
go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
  -path services/infra-fleet-service/migrations \
  -database "$INFRA_FLEET_TEST_DATABASE_URL" up 1
go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
  -path services/infra-fleet-service/migrations \
  -database "$INFRA_FLEET_TEST_DATABASE_URL" down 1
```

Expected: both directions apply cleanly; if this service already runs
migrations via `testcontainers-go` in its repository tests (see
`internal/adapter/postgres/repository_test.go`), running that test suite
after this migration lands is sufficient in place of a manual `migrate` run.
