ALTER TABLE folder_workspaces ADD COLUMN project_group_id CHAR(36);
ALTER TABLE folder_workspaces
    ADD CONSTRAINT fk_folder_workspaces_project_group FOREIGN KEY (project_group_id) REFERENCES project_groups (id) ON DELETE SET NULL;
CREATE INDEX idx_folder_workspaces_project_group ON folder_workspaces (project_group_id);
