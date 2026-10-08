ALTER TABLE request.request_links DROP CONSTRAINT IF EXISTS request_links_reason_check;
ALTER TABLE request.request_links ADD CONSTRAINT request_links_reason_check CHECK (reason IN
    ('relates_to','blocks','is_blocked_by','duplicates','spawned_by_spike','spawned_by_question','followup_hotfix','escalation'));
ALTER TABLE request.request_links ADD COLUMN created_by UUID;
ALTER TABLE request.request_links ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
