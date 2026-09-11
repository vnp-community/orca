-- Mirrors the Postgres variant's 0003_owner_id_text.up.sql: owner_id was
-- declared CHAR(36) (this dialect's UUID stand-in) in 0001_init, but real
-- callers write bare provider names ("bitbucket", "jira") or
-- provider-derived composite strings, never a UUID-only value — see the
-- Postgres migration's comment for the full TASK-043 cross-service finding
-- this widening fixes. VARCHAR(255) accepts every existing caller's
-- format, including UUID-shaped ones (36 chars), so this is a widening
-- change with no data loss for any caller, same as the Postgres ALTER.
ALTER TABLE credential_metadata MODIFY COLUMN owner_id VARCHAR(255) NOT NULL;
