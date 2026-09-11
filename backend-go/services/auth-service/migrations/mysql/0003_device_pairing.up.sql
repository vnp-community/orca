-- id is the hex-SHA-256 hash of the raw pairing token (domain.HashSessionToken,
-- same 64-char hex shape as sessions.token_hash) — VARCHAR(64), not TEXT,
-- for the same PRIMARY KEY length-bound reason as sessions.token_hash.
CREATE TABLE pairing_sessions (
    id                              VARCHAR(64) PRIMARY KEY,
    tenant_id                       CHAR(36) NOT NULL,
    user_id                         CHAR(36) NOT NULL,
    desktop_public_key              BLOB NOT NULL,
    desktop_private_key_ciphertext  BLOB NOT NULL,
    vault_key_ref                   VARCHAR(255) NOT NULL,
    created_at                      TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    expires_at                      TIMESTAMP(6) NOT NULL,  -- BR-MB-01
    consumed_at                     TIMESTAMP(6) NULL,      -- BR-MB-02

    CONSTRAINT fk_pairing_sessions_user FOREIGN KEY (user_id) REFERENCES users(id)
);
CREATE INDEX idx_pairing_sessions_expires_at ON pairing_sessions(expires_at); -- reaper job, mirrors sessions/refresh_tokens

-- id has no server-side default here (unlike Postgres's
-- `DEFAULT gen_random_uuid()`): internal/usecase/complete_device_pairing.go
-- always generates the id in Go (usecase.newUUID(), uuid.NewString() under
-- the hood) before calling PairedDeviceRepository.Save, so the Postgres
-- default was already dead code from the application's point of view —
-- confirmed the same way BE-DB-SOL-001 §1 confirmed it for
-- usage-service's sessions.id. No functional loss.
-- shared_secret_ciphertext/vault_key_ref are NULLABLE here — deliberately
-- NOT a 1:1 mirror of the Postgres variant's `BYTEA NOT NULL`/`TEXT NOT
-- NULL` (migrations/postgres/0003_device_pairing.up.sql). Discovered via
-- this rollout's own integration test
-- (TestPairedDeviceStore_Lifecycle, first run): PairedDeviceRepository.
-- RevokeAndWipeSecret (postgres/paired_device_repository.go AND this
-- package's paired_device_repository.go) sets
-- `shared_secret_ciphertext = NULL, vault_key_ref = NULL` as BR-MB-04's
-- enforcement mechanism ("the plaintext-recoverable material is gone, not
-- just flagged") — literally translating the Postgres column as NOT NULL
-- makes that UPDATE fail with a constraint violation on MySQL, which is
-- what this test caught. Re-reading the Postgres migration confirms the
-- SAME bug exists there too (NOT NULL columns the app's own revoke path
-- tries to null out) — it has simply never been exercised, because
-- auth-service's Postgres adapter has NO integration test for
-- PairedDeviceRepository at all (confirmed via `ls` before this rollout,
-- see BE-DB-SOL-014 §1). This is a real, pre-existing, security-relevant
-- bug (BR-MB-04's wipe-on-revoke would fail in production Postgres too) —
-- flagged in BE-DB-SOL-014/TASK-BE-DB-019 for a follow-up fix, NOT
-- silently fixed in the Postgres migration here (out of this task's
-- scope: MySQL adapter only). This MySQL migration encodes the column as
-- the application code's actual contract requires (nullable), rather than
-- reproducing a Postgres bug nothing currently exercises.
CREATE TABLE paired_devices (
    id                        CHAR(36) PRIMARY KEY,
    tenant_id                 CHAR(36) NOT NULL,
    user_id                   CHAR(36) NOT NULL,
    device_label              TEXT,
    shared_secret_ciphertext  BLOB,
    vault_key_ref             VARCHAR(255),
    status                    VARCHAR(16) NOT NULL DEFAULT 'active',
    paired_at                 TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    last_used_at              TIMESTAMP(6) NULL,
    revoked_at                TIMESTAMP(6) NULL,

    CONSTRAINT fk_paired_devices_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT paired_devices_status_check CHECK (status IN ('active','revoked'))
);
-- MySQL has no partial index (Postgres's `WHERE status = 'active'`) —
-- indexes the full (tenant_id, user_id) pair instead; BR-MB-03's count
-- check (CountActive) still filters `status = 'active'` explicitly in the
-- query itself, so correctness doesn't depend on the index being partial,
-- only its selectivity does (slightly less selective here than Postgres).
CREATE INDEX idx_paired_devices_user ON paired_devices(tenant_id, user_id);
