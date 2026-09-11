DROP TABLE IF EXISTS sso_identities;
ALTER TABLE users DROP CONSTRAINT users_sso_provider_check;
ALTER TABLE users DROP COLUMN sso_provider;
