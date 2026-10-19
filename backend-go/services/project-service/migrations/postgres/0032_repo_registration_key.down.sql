DROP INDEX IF EXISTS project.uq_repos_registration_key;
ALTER TABLE project.repos DROP COLUMN IF EXISTS registration_key;
