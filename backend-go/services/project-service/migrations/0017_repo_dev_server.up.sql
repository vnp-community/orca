-- Move dev-server ownership from project.projects (one per OrcaProject) to
-- project.repos (one per repo) — see Phase 10's plan doc. Root cause of a
-- whole class of live bugs this fixes: a repo's actual host was only ever
-- INFERRED from its parent project's binding, never stated directly, so a
-- repo checked out on a different host than its project's dev_server_id
-- had no way to say so — git-gateway-service silently dispatched to the
-- wrong host.
--
-- project.projects.dev_server_id is intentionally left in place (unused
-- from this migration forward) rather than dropped here — see this
-- migration's down script and Phase 10-3's plan note: drop it only once
-- the frontend no longer reads it, so a partial rollout never has a
-- service reading a column that's already gone.
ALTER TABLE project.repos ADD COLUMN dev_server_id UUID;

-- Backfill: every existing repo inherits its current parent project's
-- binding — this is the exact value git-gateway-service was already
-- resolving indirectly via GetRepo's project join, so the backfill is a
-- no-op from the running system's point of view, not a behavior change.
UPDATE project.repos r
SET dev_server_id = p.dev_server_id
FROM project.projects p
WHERE p.id = r.project_id
  AND p.dev_server_id IS NOT NULL;

CREATE INDEX idx_repos_dev_server ON project.repos (dev_server_id);
