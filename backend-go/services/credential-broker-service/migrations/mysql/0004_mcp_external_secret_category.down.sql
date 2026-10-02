-- Fails loudly if rows with the newer categories exist (CHECK re-validation).
ALTER TABLE credential_metadata DROP CHECK credential_metadata_category_check;
ALTER TABLE credential_metadata ADD CONSTRAINT credential_metadata_category_check CHECK (category IN
    ('scm_oauth', 'issue_tracker_oauth', 'ai_provider_key', 'ssh', 'service_secret'));
