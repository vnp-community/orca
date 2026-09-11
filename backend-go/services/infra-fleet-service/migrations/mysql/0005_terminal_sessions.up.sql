-- terminal_sessions backs the Terminal/PTY RPC surface added to
-- infrafleet.proto (SpawnTerminalSession et al. — see
-- proto/orca/infrafleet/v1/infrafleet.proto's TerminalSession message and
-- internal/domain/terminal_session.go). pty_id is TEXT, not CHAR(36): it is
-- assigned by the Dev Server Agent's pty-daemon (pty.create's "id" result
-- field), not generated here — see internal/adapter/devserveragent/methods.go.
--
-- tenant_id is stored explicitly (not left to a transitive join through
-- connection_id) because connection_id is nullable: this table's own doc
-- comment on host-local sessions in internal/domain/terminal_session.go, and
-- specs/backend-go/services/infra-fleet-service.md §9's "every lookup must
-- join through tenant_id" rule, both need a tenant_id column ResolveConnection
-- doesn't hand back once a connection could theoretically be empty.
-- pty_id is VARCHAR(255), not Postgres's unbounded TEXT — InnoDB cannot use
-- a TEXT/BLOB column as a PRIMARY KEY at all (no prefix-length escape hatch
-- for primary keys, unlike ordinary indexes). 255 chars is well beyond any
-- real pty.create-assigned id (see internal/adapter/devserveragent/methods.go).
CREATE TABLE terminal_sessions (
    pty_id              VARCHAR(255) PRIMARY KEY,
    tenant_id           CHAR(36) NOT NULL,
    connection_id       CHAR(36),
    -- MySQL 8.0.13+ requires TEXT column defaults to be a parenthesized
    -- expression — see migrations/mysql/0002_connections.up.sql's comment.
    cwd                 TEXT NOT NULL DEFAULT (''),
    created_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    last_active_at      TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    closed_at           TIMESTAMP(6),
    -- Named explicitly (unlike an inline REFERENCES clause, which MySQL
    -- would auto-name unpredictably) because migrations/0034 drops this
    -- exact constraint by name later — see that migration's comment.
    CONSTRAINT terminal_sessions_connection_id_fkey
        FOREIGN KEY (connection_id) REFERENCES connections(id)
);

CREATE INDEX idx_infra_terminal_sessions_tenant ON terminal_sessions (tenant_id, created_at DESC);
-- MySQL/TiDB has no partial index — full index over the nullable column
-- instead of Postgres's `WHERE connection_id IS NOT NULL`.
CREATE INDEX idx_infra_terminal_sessions_connection ON terminal_sessions (connection_id);

-- No RLS equivalent — see migrations/mysql/0001_init.up.sql's comment.
