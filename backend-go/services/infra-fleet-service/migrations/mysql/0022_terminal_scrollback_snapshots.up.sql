-- One row per (tenant, worktree, pane) — pane_key, not worktree_id alone,
-- because one worktree can have multiple simultaneous panes (F02 splits).
-- data_gzip is the CLIENT-produced @xterm/addon-serialize ANSI blob,
-- gzip-compressed; this service never parses it (see SOL-TM-03's
-- "serialize/deserialize is a client concern" rationale). Cursor
-- position/text attributes (BR-TM-11) are encoded INLINE in that ANSI blob
-- by the serializer itself — no separate cursor_row/cursor_col columns.
CREATE TABLE terminal_scrollback_snapshots (
    id                  CHAR(36) PRIMARY KEY,
    tenant_id           CHAR(36) NOT NULL,
    worktree_id         CHAR(36) NOT NULL,
    -- VARCHAR(255), not Postgres's unbounded TEXT — pane_key sits in the
    -- UNIQUE table constraint below; MySQL/InnoDB needs a fixed indexable
    -- width, and unlike a plain lookup index, a prefix-length index on a
    -- UNIQUE constraint would only guarantee uniqueness of the PREFIX, not
    -- the full value — this bounds the actual column instead.
    pane_key            VARCHAR(255) NOT NULL,
    cols                INTEGER NOT NULL,
    -- Backtick-quoted: MySQL 8.0.19+ reserves ROWS as a keyword (window-
    -- function frame clause, e.g. `ROWS BETWEEN`) — Postgres has no such
    -- reservation on this identifier. internal/adapter/mysql must
    -- backtick-quote it the same way on every read/write.
    `rows`              INTEGER NOT NULL,
    data_gzip           LONGBLOB NOT NULL,
    uncompressed_bytes  INTEGER NOT NULL,  -- pre-compression size, for BR-TM-10's cap check
    -- MySQL 8.0.13+ requires TEXT column defaults to be a parenthesized
    -- expression — see migrations/mysql/0002_connections.up.sql's comment.
    last_title          TEXT NOT NULL DEFAULT (''),
    created_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    UNIQUE (tenant_id, worktree_id, pane_key)
);

CREATE INDEX idx_infra_scrollback_snapshots_worktree
    ON terminal_scrollback_snapshots (tenant_id, worktree_id);
-- Backs BR-TM-12's expiry sweep (updated_at-based proxy for "not opened" —
-- see SOL-TM-03's BR-TM-12 caveat).
CREATE INDEX idx_infra_scrollback_snapshots_updated_at
    ON terminal_scrollback_snapshots (updated_at);

-- No RLS equivalent — see migrations/mysql/0001_init.up.sql's comment.
