-- Rows created for child requests cannot satisfy the narrower CHECK, so they go with it.
DELETE FROM request_links WHERE reason IN ('spawned_by_spike','spawned_by_question','followup_hotfix','escalation');
ALTER TABLE request_links DROP CHECK request_links_reason_check;
ALTER TABLE request_links DROP COLUMN created_at, DROP COLUMN created_by,
    ADD CONSTRAINT request_links_reason_check CHECK (reason IN ('relates_to','blocks','is_blocked_by','duplicates'));
