-- The branch or tag every worktree of a Request starts from (CR-REQ-039). Empty means the repository default, resolved when
-- a worktree is created; the column stores the name, never a SHA, and existing rows keep the default.
-- Length is bounded by VARCHAR(255): under the default STRICT_TRANS_TABLES an overlong value is rejected, and the repository
-- also refuses it before writing, so a non-strict server cannot truncate a ref silently.
ALTER TABLE requests
    ADD COLUMN base_ref VARCHAR(255) NOT NULL DEFAULT '';
