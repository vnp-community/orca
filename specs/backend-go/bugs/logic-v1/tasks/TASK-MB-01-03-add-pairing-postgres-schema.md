# TASK-MB-01-03: Add `pairing_sessions`/`paired_devices` Postgres tables + repositories

**From Solution:** SOL-MB-01
**Priority:** P0
**Service:** `auth-service`
**File:** `backend-go/services/auth-service/migrations/0003_device_pairing.up.sql`, `backend-go/services/auth-service/internal/adapter/postgres/pairing_session_repository.go`, `backend-go/services/auth-service/internal/adapter/postgres/paired_device_repository.go`
**Depends on:** TASK-MB-01-02
**Status:** [x] DONE — migration 0003 + `PairingSessionStore`/`PairedDeviceStore` repos + ports added; `go build`/`go vet` clean (integration-tagged repository tests match existing postgres-adapter convention — no DB in this environment, `go test ./internal/adapter/postgres/...` shows "no test files" same as sibling repos).

---

## Context

Auth-service's existing migrations are `0001_init`/`0002_access_policies`
(`backend-go/services/auth-service/migrations/`). This task adds `0003`,
plus the two repository adapters implementing the ports TASK-MB-01-05/06
need.

## Changes to make

`backend-go/services/auth-service/migrations/0003_device_pairing.up.sql`:

```sql
CREATE TABLE auth.pairing_sessions (
    id                              TEXT PRIMARY KEY,   -- hash of the pairing token
    tenant_id                       UUID NOT NULL,
    user_id                         UUID NOT NULL REFERENCES auth.users(id),
    desktop_public_key              BYTEA NOT NULL,
    desktop_private_key_ciphertext  BYTEA NOT NULL,
    vault_key_ref                   TEXT NOT NULL,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at                      TIMESTAMPTZ NOT NULL,  -- BR-MB-01
    consumed_at                     TIMESTAMPTZ             -- BR-MB-02
);
CREATE INDEX idx_pairing_sessions_expires_at ON auth.pairing_sessions(expires_at); -- reaper job, mirrors sessions/refresh_tokens

CREATE TABLE auth.paired_devices (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                 UUID NOT NULL,
    user_id                   UUID NOT NULL REFERENCES auth.users(id),
    device_label              TEXT,
    shared_secret_ciphertext  BYTEA NOT NULL,
    vault_key_ref             TEXT NOT NULL,
    status                    TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','revoked')),
    paired_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at              TIMESTAMPTZ,
    revoked_at                TIMESTAMPTZ
);
CREATE INDEX idx_paired_devices_user_active ON auth.paired_devices(tenant_id, user_id) WHERE status = 'active'; -- backs BR-MB-03's count check
```

`backend-go/services/auth-service/migrations/0003_device_pairing.down.sql`:

```sql
DROP TABLE IF EXISTS auth.paired_devices;
DROP TABLE IF EXISTS auth.pairing_sessions;
```

`backend-go/services/auth-service/internal/adapter/postgres/pairing_session_repository.go` — implement a `PairingSessionRepository` with:

```go
package postgres

// PairingSessionStore implements usecase.PairingSessionRepository against
// auth.pairing_sessions.
type PairingSessionStore struct {
	pool *pgxpool.Pool
}

func NewPairingSessionStore(pool *pgxpool.Pool) *PairingSessionStore { return &PairingSessionStore{pool: pool} }

func (s *PairingSessionStore) Save(ctx context.Context, session domain.PairingSession) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth.pairing_sessions
			(id, tenant_id, user_id, desktop_public_key, desktop_private_key_ciphertext, vault_key_ref, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		session.ID, session.TenantID, session.UserID, session.DesktopPublicKey,
		session.DesktopPrivateKeyCiphertext, session.VaultKeyRef, session.CreatedAt, session.ExpiresAt)
	return err
}

// GetAndConsume atomically marks the row consumed and returns it — the
// single SQL statement that closes BR-MB-02's one-time-use race between
// two concurrent CompleteDevicePairing calls on the same token.
func (s *PairingSessionStore) GetAndConsume(ctx context.Context, id string) (domain.PairingSession, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE auth.pairing_sessions
		SET consumed_at = now()
		WHERE id = $1 AND consumed_at IS NULL
		RETURNING id, tenant_id, user_id, desktop_public_key, desktop_private_key_ciphertext, vault_key_ref, created_at, expires_at, consumed_at`,
		id)
	var session domain.PairingSession
	err := row.Scan(&session.ID, &session.TenantID, &session.UserID, &session.DesktopPublicKey,
		&session.DesktopPrivateKeyCiphertext, &session.VaultKeyRef, &session.CreatedAt, &session.ExpiresAt, &session.ConsumedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PairingSession{}, fmt.Errorf("postgres: %w", domain.ErrPairingTokenNotFound)
	}
	return session, err
}
```

`backend-go/services/auth-service/internal/adapter/postgres/paired_device_repository.go` — implement `Save`, `CountActive(ctx, tenantID, userID)`, `Get(ctx, id)`, `List(ctx, tenantID, userID)`, `RevokeAndWipeSecret(ctx, deviceID)` (`UPDATE ... SET status='revoked', shared_secret_ciphertext=NULL, vault_key_ref=NULL, revoked_at=now()`), and `Touch(ctx, id, now)` (updates `last_used_at`) following the same `pgxpool.Pool`-wrapping style as `backend-go/services/auth-service/internal/adapter/postgres/`'s existing repositories (e.g. its `UserRepository`/`SessionRepository` implementations).

Add both to `usecase/ports.go`:

```go
type PairingSessionRepository interface {
	Save(ctx context.Context, session domain.PairingSession) error
	GetAndConsume(ctx context.Context, id string) (domain.PairingSession, error)
}

type PairedDeviceRepository interface {
	Save(ctx context.Context, device domain.PairedDevice) error
	CountActive(ctx context.Context, tenantID, userID string) (int, error)
	Get(ctx context.Context, id string) (domain.PairedDevice, error)
	List(ctx context.Context, tenantID, userID string) ([]domain.PairedDevice, error)
	RevokeAndWipeSecret(ctx context.Context, id string) error
	Touch(ctx context.Context, id string, now time.Time) error
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/... && go vet ./services/auth-service/...
go test ./services/auth-service/internal/adapter/postgres/... -run PairedDevice -run PairingSession
```
