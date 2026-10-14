-- What an authored OpenSpec change was built from: its digest, the knowledge the agent says it read, the skills that were
-- installed, and the files themselves, so the proposal stays readable after its worktree is removed.
ALTER TABLE openspec_changes
    ADD COLUMN tasks_md_digest VARCHAR(128) NULL,
    ADD COLUMN kg_available BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN kg_snapshot VARCHAR(128) NULL,
    ADD COLUMN kg_read_ids JSON NULL,
    ADD COLUMN kg_gaps JSON NULL,
    ADD COLUMN kg_conflicts JSON NULL,
    ADD COLUMN skill_hash VARCHAR(128) NULL,
    ADD COLUMN bundle_json JSON NULL,
    ADD COLUMN workspace_id VARCHAR(64) NULL;
