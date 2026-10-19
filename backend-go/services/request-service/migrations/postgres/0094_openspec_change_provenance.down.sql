ALTER TABLE request.openspec_changes
    DROP COLUMN IF EXISTS workspace_id,
    DROP COLUMN IF EXISTS bundle_json,
    DROP COLUMN IF EXISTS skill_hash,
    DROP COLUMN IF EXISTS kg_conflicts,
    DROP COLUMN IF EXISTS kg_gaps,
    DROP COLUMN IF EXISTS kg_read_ids,
    DROP COLUMN IF EXISTS kg_snapshot,
    DROP COLUMN IF EXISTS kg_available,
    DROP COLUMN IF EXISTS tasks_md_digest;
