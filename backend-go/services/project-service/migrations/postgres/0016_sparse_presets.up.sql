-- sparse_presets: saved directory sets for sparse worktree creation,
-- scoped to one specific repo — ports backend/src/main/persistence.ts's
-- (legacy TS) getSparsePresets/saveSparsePreset/removeSparsePreset. No
-- name-uniqueness constraint here, matching the legacy implementation
-- exactly: collision detection is a frontend-only UX nicety
-- (SparsePresetSettingsSection.tsx's collidingPreset check), never enforced
-- server-side.
CREATE TABLE project.sparse_presets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id     UUID NOT NULL REFERENCES project.repos(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    directories TEXT[] NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sparse_presets_repo ON project.sparse_presets (repo_id);

-- No tenant_id column of its own — same convention as repo_members
-- (migrations/0011's comment): tenant scoping is transitive through
-- repo_id -> project.repos.project_id -> project.projects.tenant_id.
ALTER TABLE project.sparse_presets ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project.sparse_presets
    USING (repo_id IN (
        SELECT r.id FROM project.repos r
        JOIN project.projects p ON p.id = r.project_id
        WHERE p.tenant_id = current_setting('app.tenant_id', true)::uuid
    ));
