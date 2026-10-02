-- Durability for Engine 1 (direct_agent): the run lives in a goroutine inside
-- one task-service process, so a restart used to leave the task stuck at
-- in_progress forever. A lease (renewed by a heartbeat while the run is alive)
-- lets any instance notice an abandoned run and revert the task.
--
-- previous_status: what the task was before this dispatch, so recovery can put
-- it back exactly (the in-memory snapshot dies with the process).
-- Only rows that were given a lease are ever swept; rows written before this
-- migration keep lease_expires_at NULL and are left alone.
ALTER TABLE task.execution_links
  ADD COLUMN lease_expires_at TIMESTAMPTZ,
  ADD COLUMN lease_owner      TEXT NOT NULL DEFAULT '',
  ADD COLUMN previous_status  TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_execution_links_expired_lease
  ON task.execution_links (lease_expires_at)
  WHERE engine = 'direct_agent' AND status_mirror = 'in_progress' AND lease_expires_at IS NOT NULL;
