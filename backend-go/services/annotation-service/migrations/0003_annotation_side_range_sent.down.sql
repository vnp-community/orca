DROP INDEX IF EXISTS annotation.idx_annotations_worktree;

ALTER TABLE annotation.annotations
    DROP COLUMN worktree_id,
    DROP COLUMN end_line,
    DROP COLUMN side,
    DROP COLUMN original_code,
    DROP COLUMN sent_to_agent,
    DROP COLUMN sent_at;
