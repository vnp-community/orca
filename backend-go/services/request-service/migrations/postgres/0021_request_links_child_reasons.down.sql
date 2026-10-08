-- Rows created for child requests cannot satisfy the narrower CHECK, so they go with it.
DELETE FROM request.request_links WHERE reason IN ('spawned_by_spike','spawned_by_question','followup_hotfix','escalation');
ALTER TABLE request.request_links DROP COLUMN IF EXISTS created_at;
ALTER TABLE request.request_links DROP COLUMN IF EXISTS created_by;
ALTER TABLE request.request_links DROP CONSTRAINT IF EXISTS request_links_reason_check;
ALTER TABLE request.request_links ADD CONSTRAINT request_links_reason_check CHECK (reason IN ('relates_to','blocks','is_blocked_by','duplicates'));
