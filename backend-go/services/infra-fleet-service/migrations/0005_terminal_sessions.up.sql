-- infra.terminal_sessions backs the Terminal/PTY RPC surface added to
-- infrafleet.proto (SpawnTerminalSession et al. — see
-- proto/orca/infrafleet/v1/infrafleet.proto's TerminalSession message and
-- internal/domain/terminal_session.go). pty_id is TEXT, not UUID: it is
-- assigned by the Dev Server Agent's pty-daemon (pty.create's "id" result
-- field), not generated here — see internal/adapter/devserveragent/methods.go.
--
-- tenant_id is stored explicitly (not left to a transitive join through
-- connection_id) because connection_id is nullable: this table's own doc
-- comment on host-local sessions in internal/domain/terminal_session.go, and
-- specs/backend-go/services/infra-fleet-service.md §9's "every lookup must
-- join through tenant_id" rule, both need a tenant_id column ResolveConnection
-- doesn't hand back once a connection could theoretically be empty.
CREATE TABLE infra.terminal_sessions (
    pty_id              TEXT PRIMARY KEY,
    tenant_id           UUID NOT NULL,
    connection_id       UUID REFERENCES infra.connections(id),
    cwd                 TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at           TIMESTAMPTZ
);

CREATE INDEX idx_infra_terminal_sessions_tenant ON infra.terminal_sessions (tenant_id, created_at DESC);
CREATE INDEX idx_infra_terminal_sessions_connection ON infra.terminal_sessions (connection_id)
    WHERE connection_id IS NOT NULL;

ALTER TABLE infra.terminal_sessions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.terminal_sessions
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
