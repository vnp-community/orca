-- JSONB -> JSON (MySQL has no binary-JSON distinction).
ALTER TABLE repos ADD COLUMN hook_settings JSON;
