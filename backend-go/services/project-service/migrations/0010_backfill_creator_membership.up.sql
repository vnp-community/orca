-- Backfills a missing owner membership row for every project created
-- before the api-gateway "project.create" wscompat channel started calling
-- AddMember for the creator (specs/backend-go/crs/v0/dev-server-access-
-- control/solutions/README.md, "Twenty-third"). Without this, a project's
-- own creator gets PROJECT_NOT_AUTHORIZED the moment they select it —
-- live-reproduced for "Vnp-asm", created before that fix shipped.
INSERT INTO project.project_members (project_id, user_id, role, added_at)
SELECT p.id, p.created_by, 'owner', now()
FROM project.projects p
WHERE NOT EXISTS (
    SELECT 1 FROM project.project_members m
    WHERE m.project_id = p.id AND m.user_id = p.created_by
)
ON CONFLICT (project_id, user_id) DO NOTHING;
