-- Append-only, versioned access-policy documents (admin console RBAC/rate
-- tiers) — see auth-service.md:150/172 and internal/usecase/ports.go's
-- AccessPolicyRepository doc comment: UpdateAccessPolicy never mutates a
-- row in place, it always inserts a new (id, version) row, so this table
-- has no UPDATE in its normal write path, only INSERT/SELECT/DELETE.
CREATE TABLE auth.access_policies (
    id              UUID NOT NULL,
    name            TEXT NOT NULL,
    kind            TEXT NOT NULL,
    document        JSONB NOT NULL,
    version         INT NOT NULL,
    updated_by      UUID,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (id, version),
    UNIQUE (name, version)
);

-- Fast "give me the latest version of every policy" lookups
-- (ListAccessPolicies, GetAdminStats's total_policies count).
CREATE INDEX idx_auth_access_policies_id_version ON auth.access_policies (id, version DESC);
