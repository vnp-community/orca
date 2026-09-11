-- project.folder_workspaces — standalone, non-git filesystem paths added
-- directly to the workspace. Distinct from project.repos: no project_id
-- (a folder workspace isn't owned by a Project), and dev_server_id is
-- required since there's no owning Project to inherit it from. See
-- domain.FolderWorkspace's doc comment and
-- specs/backend-go/bugs/missing-v1/solutions/SOL-010-folderworkspace-channels.md.
--
-- UNIQUE (tenant_id, dev_server_id, path) is the authoritative conflict
-- guard CreateFolderWorkspace relies on — GetFolderWorkspacePathStatus is a
-- pre-flight UX convenience that queries the same uniqueness, not the only
-- enforcement point.
CREATE TABLE project.folder_workspaces (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    dev_server_id UUID NOT NULL, -- logical FK -> infra-fleet-service.dev_servers
    path          TEXT NOT NULL,
    name          TEXT NOT NULL,
    added_by      UUID NOT NULL, -- logical FK -> tenant-service.users
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, dev_server_id, path)
);
CREATE INDEX idx_folder_workspaces_tenant ON project.folder_workspaces (tenant_id);

ALTER TABLE project.folder_workspaces ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project.folder_workspaces
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
