-- Deviation from the Postgres source (documented, not silent): `idempotency_key`
-- is TEXT there (unbounded) but participates in a UNIQUE index here.
-- VARCHAR(255) comfortably covers orca-cli's own default value
-- (sha256 hex digest, 64 chars) and any realistic caller-supplied custom
-- key — see 0006_folder_workspaces.up.sql for the identical tradeoff on
-- `path`.
ALTER TABLE worktrees ADD COLUMN idempotency_key VARCHAR(255);

-- Postgres source is a PARTIAL unique index (`WHERE idempotency_key IS NOT
-- NULL`) — a plain MySQL UNIQUE index has the same effective semantics
-- (NULLs are mutually distinct, only non-NULL values are constrained), see
-- 0008_project_groups_project_id.up.sql's identical reasoning.
CREATE UNIQUE INDEX worktrees_project_idempotency_key_idx ON worktrees (project_id, idempotency_key);
