DROP INDEX uq_repos_registration_key ON repos;
ALTER TABLE repos DROP COLUMN registration_key;
