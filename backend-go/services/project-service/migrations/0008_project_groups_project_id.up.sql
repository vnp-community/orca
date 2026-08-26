-- project.project_groups.project_id — links a group to the specific
-- project it was created for during nested-repo import (MoveProject's
-- leaf-group node). Nullable: most groups stay pure organizational folders.
ALTER TABLE project.project_groups
    ADD COLUMN project_id UUID REFERENCES project.projects (id) ON DELETE CASCADE;

-- Partial unique index: at most one leaf group per project — enforces
-- UpsertLeafGroupForProject's find-or-create invariant at the DB layer,
-- not just in application code.
CREATE UNIQUE INDEX idx_project_groups_project_id
    ON project.project_groups (project_id) WHERE project_id IS NOT NULL;
