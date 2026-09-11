ALTER TABLE project_groups ADD COLUMN project_id CHAR(36);
ALTER TABLE project_groups
    ADD CONSTRAINT fk_project_groups_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

-- Postgres source uses a PARTIAL unique index (`WHERE project_id IS NOT
-- NULL`) — MySQL has no partial index, but a plain UNIQUE index already
-- has the same effective semantics here: MySQL treats every NULL in a
-- UNIQUE index as distinct from every other NULL (unlike a non-NULL
-- value), so multiple groups with project_id = NULL are still allowed,
-- and only non-NULL values are constrained to be unique — exactly
-- UpsertLeafGroupForProject's "at most one leaf group per project"
-- invariant. No behavior gap.
CREATE UNIQUE INDEX idx_project_groups_project_id ON project_groups (project_id);
