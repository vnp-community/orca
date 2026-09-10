-- Adds diff-side, multi-line range, code snapshot, worktree scoping, and
-- sent-to-agent state to annotations — see BUG-CR-02 and SOL-CR-02.
-- All columns are additive with safe defaults; existing rows get
-- side=SIDE_UNSPECIFIED (0), end_line=0 (single-line), sent_to_agent=false
-- — "this data didn't exist for old rows", not a fabricated value.
ALTER TABLE annotation.annotations
    ADD COLUMN worktree_id   TEXT,
    ADD COLUMN end_line      INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN side          SMALLINT NOT NULL DEFAULT 0, -- 0=unspecified,1=old,2=new
    ADD COLUMN original_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN sent_to_agent BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN sent_at       TIMESTAMPTZ;

CREATE INDEX idx_annotations_worktree ON annotation.annotations (tenant_id, worktree_id)
    WHERE worktree_id IS NOT NULL;
