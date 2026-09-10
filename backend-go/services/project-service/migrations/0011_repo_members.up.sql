-- repo_members: a second, repo-scoped authorization tier layered on top of
-- project_members. project_members.role (member/owner) decides who's in a
-- project at all; repo_members.functional_role decides what a project
-- member can do on one specific repo (a developer might be granted onto
-- repo X but have no grant at all on repo Y in the same project). A
-- project owner always bypasses this tier on their own project's repos —
-- enforced in Go (requireRepoAccess), not here.
CREATE TABLE project.repo_members (
    repo_id         UUID NOT NULL REFERENCES project.repos(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL,
    functional_role TEXT NOT NULL CHECK (functional_role IN ('developer', 'lead', 'admin')),
    added_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (repo_id, user_id)
);
CREATE INDEX idx_repo_members_user ON project.repo_members (user_id);

-- No tenant_id column of its own — same convention as project_members
-- (migrations/0001's comment): tenant scoping is transitive through
-- repo_id -> project.repos.project_id -> project.projects.tenant_id, one
-- hop further than project_members' own RLS subquery (0001), since
-- repo_members is keyed on repo_id, not project_id directly.
ALTER TABLE project.repo_members ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project.repo_members
    USING (repo_id IN (
        SELECT r.id FROM project.repos r
        JOIN project.projects p ON p.id = r.project_id
        WHERE p.tenant_id = current_setting('app.tenant_id', true)::uuid
    ));
