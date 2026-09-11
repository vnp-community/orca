-- Adds the columns EstablishConnection (ssh.connect, TASK-164) needs to
-- record a connection's live handshake state — connections
-- previously only tracked the static worktree/dev-server binding written
-- by CreateConnection. See specs/backend-go/bugs/missing-v1/BUG-024.
-- MySQL 8.0.13+ requires TEXT column defaults to be a parenthesized
-- expression — see migrations/mysql/0002_connections.up.sql's comment.
ALTER TABLE connections
    ADD COLUMN status TEXT NOT NULL DEFAULT (''),
    ADD COLUMN last_activity_at TIMESTAMP(6);
