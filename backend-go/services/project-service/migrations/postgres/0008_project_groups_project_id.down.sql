DROP INDEX IF EXISTS project.idx_project_groups_project_id;
ALTER TABLE project.project_groups DROP COLUMN IF EXISTS project_id;
