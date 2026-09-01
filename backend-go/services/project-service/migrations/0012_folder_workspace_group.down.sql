DROP INDEX IF EXISTS project.idx_folder_workspaces_project_group;
ALTER TABLE project.folder_workspaces DROP COLUMN IF EXISTS project_group_id;
