CREATE TABLE ephemeral_vm_runtimes (
  id              CHAR(36) PRIMARY KEY,
  tenant_id       CHAR(36) NOT NULL,
  repo_id         CHAR(36) NOT NULL,              -- logical FK -> project-service's repo, not enforced here (cross-service, per database-per-service rule)
  recipe_id       TEXT NOT NULL,               -- matches OrcaVmRecipe.id from orca.yaml, not a Postgres FK (recipes are repo-authored, not backend-owned rows)
  -- VARCHAR, not TEXT, for these two CHECK-constrained enum columns — MySQL
  -- 8.0.13+ requires TEXT column defaults to be a parenthesized expression
  -- (see migrations/mysql/0002_connections.up.sql's comment); a short
  -- bounded VARCHAR with a plain default is simpler and equally correct
  -- for a fixed enum set.
  connection_type VARCHAR(16) NOT NULL DEFAULT '' CHECK (connection_type IN ('', 'orca-server', 'ssh')),
  status          VARCHAR(16) NOT NULL DEFAULT 'provisioning' CHECK (status IN
                     ('provisioning', 'active', 'suspended', 'error', 'destroyed')),
  environment_id  TEXT,                        -- set once an orca-server-type recipe's pairing succeeds; NULL for ssh-type (permanently blocked, see TASK-006)
  workspace_id    TEXT,                        -- set by AttachWorkspace (TASK-004); NULL until a workspace/worktree is attached
  last_error      TEXT,
  created_at      TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at      TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);
-- MySQL/TiDB has no partial index — full index instead of Postgres's
-- `WHERE status <> 'destroyed'` on both.
CREATE INDEX idx_ephemeral_vm_runtimes_tenant ON ephemeral_vm_runtimes (tenant_id);
CREATE INDEX idx_ephemeral_vm_runtimes_repo ON ephemeral_vm_runtimes (repo_id);
