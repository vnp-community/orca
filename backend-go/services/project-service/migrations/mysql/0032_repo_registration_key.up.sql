-- See migrations/postgres/0032: key written only for new registrations, legacy
-- duplicates stay NULL (MySQL unique indexes also allow many NULLs).
ALTER TABLE repos ADD COLUMN registration_key CHAR(64) NULL;
CREATE UNIQUE INDEX uq_repos_registration_key ON repos (registration_key);
