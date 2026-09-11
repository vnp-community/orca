ALTER TABLE repos ADD COLUMN dev_server_id CHAR(36);

-- MySQL has no `UPDATE ... FROM` multi-table syntax — translated to a
-- multi-table UPDATE ... JOIN, same semantics (backfill every existing
-- repo with its parent project's current dev_server_id binding).
UPDATE repos r
JOIN projects p ON p.id = r.project_id
SET r.dev_server_id = p.dev_server_id
WHERE p.dev_server_id IS NOT NULL;

CREATE INDEX idx_repos_dev_server ON repos (dev_server_id);
