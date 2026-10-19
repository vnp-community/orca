-- The branch or tag every worktree of a Request starts from (CR-REQ-039). Empty means the repository default, resolved when
-- a worktree is created; the column stores the name, never a SHA, and existing rows keep the default.
ALTER TABLE request.requests
    ADD COLUMN base_ref TEXT NOT NULL DEFAULT ''
        CONSTRAINT requests_base_ref_length CHECK (char_length(base_ref) <= 255);
