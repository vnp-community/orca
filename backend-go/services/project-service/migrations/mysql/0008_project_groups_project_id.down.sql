DROP INDEX idx_project_groups_project_id ON project_groups;
ALTER TABLE project_groups DROP FOREIGN KEY fk_project_groups_project;
ALTER TABLE project_groups DROP COLUMN project_id;
