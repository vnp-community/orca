-- Adds the columns EstablishConnection (ssh.connect, TASK-164) needs to
-- record a connection's live handshake state — infra.connections
-- previously only tracked the static worktree/dev-server binding written
-- by CreateConnection. See specs/backend-go/bugs/missing-v1/BUG-024.
ALTER TABLE infra.connections
    ADD COLUMN status TEXT NOT NULL DEFAULT '',
    ADD COLUMN last_activity_at TIMESTAMPTZ;
