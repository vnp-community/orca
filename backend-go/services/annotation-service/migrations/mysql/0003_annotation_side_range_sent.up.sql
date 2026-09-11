-- Adds diff-side, multi-line range, code snapshot, worktree scoping, and
-- sent-to-agent state to annotations — mirrors the Postgres variant (see
-- BUG-CR-02/SOL-CR-02). All columns are additive; this migration always
-- runs against an empty `annotations` table on a fresh MySQL-dialect
-- deployment (see 0002's comment), so — unlike the Postgres variant —
-- there is no pre-existing row whose NOT NULL columns need a same-file
-- DEFAULT to satisfy; sent_to_agent/end_line/side still declare one
-- anyway purely as the column's steady-state value for future inserts
-- that omit it, matching the Postgres variant's documented intent.
ALTER TABLE annotations
    ADD COLUMN worktree_id   VARCHAR(255) NULL,
    ADD COLUMN end_line      INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN side          SMALLINT NOT NULL DEFAULT 0, -- 0=unspecified,1=old,2=new
    ADD COLUMN original_code TEXT NOT NULL,
    ADD COLUMN sent_to_agent BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN sent_at       TIMESTAMP(6) NULL;

-- MySQL has no partial index (Postgres's `WHERE worktree_id IS NOT NULL`)
-- — indexes the full (tenant_id, worktree_id) pair instead. A row with
-- worktree_id NULL simply isn't excluded from the index the way Postgres
-- excludes it; this index is smaller in Postgres but still correct (and
-- still selective on tenant_id first) in MySQL.
CREATE INDEX idx_annotations_worktree ON annotations (tenant_id, worktree_id);
