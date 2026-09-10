-- CR-CLI-002/TASK-BE-CLI-005: revocation-list entry for every CLI/service
-- JWT IssueServiceToken mints. No "get by jti" read path is needed for JWT
-- validation itself (signature/exp via JWKS covers that) — this table only
-- backs IsServiceTokenRevoked's jti -> bool check and the self-service
-- ListCliTokens/RevokeCliToken surface.
CREATE TABLE auth.issued_service_tokens (
    jti         TEXT PRIMARY KEY,
    user_id     UUID NOT NULL,
    audience    TEXT NOT NULL,
    issued_at   TIMESTAMPTZ NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ
);

CREATE INDEX idx_issued_service_tokens_user_id ON auth.issued_service_tokens (user_id);
