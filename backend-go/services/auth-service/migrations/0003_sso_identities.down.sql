DROP TABLE IF EXISTS auth.sso_identities;
ALTER TABLE auth.users DROP COLUMN IF EXISTS sso_provider;
