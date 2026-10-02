-- Rows already using the two newer categories would violate the restored
-- CHECK; fail loudly instead of discarding them.
ALTER TABLE credential.credential_metadata DROP CONSTRAINT IF EXISTS credential_metadata_category_check;
ALTER TABLE credential.credential_metadata ADD CONSTRAINT credential_metadata_category_check CHECK (category IN
    ('scm_oauth', 'issue_tracker_oauth', 'ai_provider_key', 'ssh', 'service_secret'));
