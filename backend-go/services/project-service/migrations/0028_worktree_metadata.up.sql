-- UI-authored worktree metadata (displayName/comment/isPinned/pushTarget/
-- sparse*/... — frontend/src/shared/types.ts's WorktreeMeta) as a single
-- opaque JSONB blob, not one column per field: WorktreeMeta gains fields
-- often and independently of this schema (desktop already persists all of
-- them today, per-field, in its own local orca-data.json), and a typed
-- column per key would need a migration on every frontend addition for no
-- real validation this backend acts on. Same JSONB-blob convention as
-- tenant.tenants/user_profiles/team_profiles's settings_json.
ALTER TABLE project.worktrees
    ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}'::jsonb;
