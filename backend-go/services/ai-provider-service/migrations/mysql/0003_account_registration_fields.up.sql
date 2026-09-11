ALTER TABLE accounts
  ADD COLUMN quota_limit_day      INT NOT NULL DEFAULT 0,  -- 0 = unlimited, per ai-provider-service.md §5
  ADD COLUMN last_health_check_at TIMESTAMP(6) NULL,
  ADD COLUMN created_by           CHAR(36) NULL,
  -- Postgres's TEXT[] has no MySQL/TiDB equivalent array type — stored as
  -- a JSON array of strings instead (MySQL 5.7+/TiDB native JSON type;
  -- internal/adapter/mysql/repository.go marshals/unmarshals []string on
  -- every read/write, same "no JSON operator needed, only whole-document
  -- read/write" scope usage-service's outbox JSONB→JSON translation
  -- already established — nothing here ever queries INTO the array with a
  -- JSON operator). NOT NULL DEFAULT (JSON_ARRAY()) mirrors the Postgres
  -- column's NOT NULL DEFAULT '{}' (empty array, never NULL) — MySQL
  -- requires JSON/TEXT/BLOB defaults to be written as an expression
  -- (parenthesized), a plain literal default isn't accepted for this type.
  ADD COLUMN models               JSON NOT NULL DEFAULT (JSON_ARRAY()),
  ADD COLUMN is_default           BOOLEAN NOT NULL DEFAULT false;

-- At most one default per (tenant, dev_server, provider_type) — enforced
-- at the DB level, mirroring credential-broker-service's unique_vault_path
-- posture (defense in depth, not "trust the usecase layer alone"). The
-- Postgres original does this with a partial unique index
-- (`WHERE is_default AND deleted_at IS NULL`); MySQL/TiDB has no partial
-- index of any kind (unique or not) — this is the first UNIQUE partial
-- index encountered in the CR-DB-002/003 rollout so far (every prior
-- batch-1 service's partial index was non-unique, safely widened to a
-- full index instead, see 0001_init.up.sql's idx_accounts_tenant_scope_*
-- above). Simply dropping the WHERE clause here would be wrong: a plain
-- UNIQUE(tenant_id, dev_server_id, provider_type, is_default) would also
-- reject a SECOND non-default account in the same group (two rows with
-- is_default=false collide on the same key), which the Postgres partial
-- index never did.
--
-- Standard MySQL workaround: a generated column that evaluates to the
-- uniqueness key ONLY when the partial index's WHERE condition holds, and
-- to NULL otherwise — then a plain UNIQUE index on that generated column.
-- MySQL treats multiple NULLs in a UNIQUE index as distinct (same as
-- Postgres), so non-default/deleted rows never collide with each other,
-- while at most one row per (tenant_id, dev_server_id, provider_type) can
-- ever hold the non-NULL "is the default" value at once — the exact
-- semantics of the Postgres partial index, translated rather than weakened.
ALTER TABLE accounts
  ADD COLUMN default_slot_key VARCHAR(600) GENERATED ALWAYS AS (
    CASE WHEN is_default AND deleted_at IS NULL
         THEN CONCAT(tenant_id, '|', dev_server_id, '|', provider_type)
         ELSE NULL END
  ) STORED;

CREATE UNIQUE INDEX uq_accounts_one_default_per_dev_server_provider
  ON accounts (default_slot_key);

-- quota_limit_day >= 1000 (BL-AIP-01's field rule) is enforced in the domain
-- constructor, not a CHECK constraint — 0 (unlimited) must stay legal, and
-- "no lower than 1000 unless 0" isn't cleanly expressible as one CHECK
-- clause without duplicating the domain's own validation decision. Same as
-- the Postgres variant's comment, unchanged by dialect.
