ALTER TABLE ai_provider.accounts
  ADD COLUMN quota_limit_day      INTEGER NOT NULL DEFAULT 0,  -- 0 = unlimited, per ai-provider-service.md §5
  ADD COLUMN last_health_check_at TIMESTAMPTZ,
  ADD COLUMN created_by           UUID,
  -- Not in §5's original sketch — added per SOL-AIP-01's rationale
  -- (BUG-AIP-02's Models-list dependency, BL-AIP-02's server-default tier).
  ADD COLUMN models               TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN is_default           BOOLEAN NOT NULL DEFAULT false;

-- At most one default per (tenant, dev_server, provider_type) — enforced at
-- the DB level, mirroring credential-broker-service's unique_vault_path
-- posture (defense in depth, not "trust the usecase layer alone").
CREATE UNIQUE INDEX uq_accounts_one_default_per_dev_server_provider
  ON ai_provider.accounts (tenant_id, dev_server_id, provider_type)
  WHERE is_default AND deleted_at IS NULL;

-- quota_limit_day >= 1000 (BL-AIP-01's field rule) is enforced in the domain
-- constructor, not a CHECK constraint — 0 (unlimited) must stay legal, and
-- "no lower than 1000 unless 0" isn't cleanly expressible as one CHECK
-- clause without duplicating the domain's own validation decision.
