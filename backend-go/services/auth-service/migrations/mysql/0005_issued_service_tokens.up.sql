-- jti is base64url(32 random bytes) (see internal/usecase/issue_service_token.go's
-- serviceTokenJTIBytes) — 43 chars, well within VARCHAR(128)'s bound.
-- VARCHAR, not TEXT, for the same PRIMARY KEY length-bound reason as
-- sessions.token_hash.
CREATE TABLE issued_service_tokens (
    jti         VARCHAR(128) PRIMARY KEY,
    user_id     CHAR(36) NOT NULL,
    audience    VARCHAR(255) NOT NULL,
    issued_at   TIMESTAMP(6) NOT NULL,
    expires_at  TIMESTAMP(6) NOT NULL,
    revoked_at  TIMESTAMP(6) NULL,

    CONSTRAINT fk_issued_service_tokens_user FOREIGN KEY (user_id) REFERENCES users(id)
);
CREATE INDEX idx_issued_service_tokens_user_id ON issued_service_tokens(user_id);
