-- What an authored OpenSpec change was built from: its digest, the knowledge the agent says it read, the skills that were
-- installed, and the files themselves, so the proposal stays readable after its worktree is removed.
ALTER TABLE request.openspec_changes
    ADD COLUMN tasks_md_digest TEXT NULL,
    ADD COLUMN kg_available BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN kg_snapshot TEXT NULL,
    ADD COLUMN kg_read_ids JSONB NULL,
    ADD COLUMN kg_gaps JSONB NULL,
    ADD COLUMN kg_conflicts JSONB NULL,
    ADD COLUMN skill_hash TEXT NULL,
    ADD COLUMN bundle_json JSONB NULL,
    ADD COLUMN workspace_id TEXT NULL;
