-- Append-only, versioned access-policy documents (admin console RBAC/rate
-- tiers) — see auth-service.md:150/172 and internal/usecase/ports.go's
-- AccessPolicyRepository doc comment: UpdateAccessPolicy never mutates a
-- row in place, it always inserts a new (id, version) row, so this table
-- has no UPDATE in its normal write path, only INSERT/SELECT/DELETE.
--
-- document is JSON (not JSONB — MySQL has no separate binary-JSON type,
-- "JSON" already stores an optimized binary form internally); no query in
-- internal/adapter/postgres/access_policy_repository.go uses a JSONB
-- operator (->, @>, etc.) on this column, so there's no lost capability
-- to translate here, only the column-type keyword.
CREATE TABLE access_policies (
    id              CHAR(36) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    kind            VARCHAR(64) NOT NULL,
    document        JSON NOT NULL,
    version         INT NOT NULL,
    updated_by      CHAR(36),
    updated_at      TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id, version),
    UNIQUE (name, version)
);

-- Fast "give me the latest version of every policy" lookups
-- (ListAccessPolicies, GetAdminStats's total_policies count).
CREATE INDEX idx_auth_access_policies_id_version ON access_policies (id, version DESC);
