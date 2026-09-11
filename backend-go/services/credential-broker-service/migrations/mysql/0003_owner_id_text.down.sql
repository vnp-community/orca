-- Reverts owner_id to CHAR(36). Only safe if every existing row's owner_id
-- is still UUID-shaped — unlike the Postgres variant's `::uuid` cast
-- (which fails loudly on non-UUID data), MySQL's MODIFY COLUMN silently
-- truncates any value longer than 36 characters instead of erroring. This
-- is a real behavior difference from the Postgres down-migration's
-- fail-loudly guarantee — documented here rather than silently accepted,
-- since MySQL has no built-in equivalent to a validating type cast on
-- ALTER. Operators must verify data manually before running this down
-- migration in the MySQL/TiDB dialect.
ALTER TABLE credential_metadata MODIFY COLUMN owner_id CHAR(36) NOT NULL;
