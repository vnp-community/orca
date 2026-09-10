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
