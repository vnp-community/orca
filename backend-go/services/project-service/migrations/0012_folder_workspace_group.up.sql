-- Adds real persistence for "which project group does this folder
-- workspace belong to" — the sidebar's own organizational concept already
-- existed client-side (FolderWorkspace.projectGroupId in the frontend
-- type), but project.folder_workspaces had nowhere to store it, so every
-- folderWorkspace.create call that tried to pass a group silently
-- discarded it. Nullable: a folder workspace with no group is valid,
-- matching today's optional grouping.
ALTER TABLE project.folder_workspaces
    ADD COLUMN project_group_id UUID REFERENCES project.project_groups(id) ON DELETE SET NULL;
CREATE INDEX idx_folder_workspaces_project_group ON project.folder_workspaces (project_group_id);
