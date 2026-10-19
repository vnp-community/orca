-- One registration per (tenant, dev server, normalised path): the key is a
-- SHA-256 hex computed by the application (domain.RepoRegistrationKey) and is
-- written ONLY for repos registered from now on. Existing rows stay NULL on
-- purpose: some already have duplicates (the same folder in two projects), and
-- NULLs never collide in a unique index, so nothing is migrated or removed.
ALTER TABLE project.repos ADD COLUMN registration_key VARCHAR(64);
CREATE UNIQUE INDEX uq_repos_registration_key ON project.repos (registration_key);
