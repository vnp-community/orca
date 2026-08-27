# TASK-AWS-03-04: Implement `AgentTokenRepository` over Postgres

**From Solution:** SOL-AWS-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/postgres/agent_token_repository.go` (new)
**Depends on:** TASK-AWS-03-01, TASK-AWS-03-03
**Status:** [x] DONE — `AgentTokenStore` created verbatim per spec, `var _ usecase.AgentTokenRepository = (*AgentTokenStore)(nil)` compiles; wired into main.go (`agentTokenStore`, consumed by TASK-AWS-03-05/06/07); `go build`/`go vet` clean. No live Postgres available in this worktree for the RLS/exclusion integration test — left for a follow-up with a test DB.

---

## Context

Implements `usecase.AgentTokenRepository` against `infra.agent_tokens`,
following `BrowserProfileStore`'s existing shape (own type over the shared
`pgxpool.Pool`, not a method on the umbrella `Repository`, per that
adapter's package doc comment on method-name collisions).

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/adapter/postgres/agent_token_repository.go`:

```go
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

var _ usecase.AgentTokenRepository = (*AgentTokenStore)(nil)

// AgentTokenStore implements usecase.AgentTokenRepository against
// infra.agent_tokens (migrations/0007_agent_tokens) — see
// BrowserProfileStore's doc comment for why this is its own type over the
// shared pool rather than a method on Repository.
type AgentTokenStore struct {
	pool *pgxpool.Pool
}

// NewAgentTokenStore builds an AgentTokenStore over the same pool
// Repository/SshTargetStore/BrowserProfileStore use.
func NewAgentTokenStore(pool *pgxpool.Pool) *AgentTokenStore {
	return &AgentTokenStore{pool: pool}
}

const agentTokenColumns = `id, tenant_id, dev_server_id, name, COALESCE(token_hash, ''), COALESCE(credential_ref_id::text, ''), created_at, last_used_at, revoked_at`

func scanAgentToken(row pgx.Row) (domain.AgentToken, error) {
	var t domain.AgentToken
	if err := row.Scan(&t.ID, &t.TenantID, &t.DevServerID, &t.Name, &t.TokenHash, &t.CredentialRefID, &t.CreatedAt, &t.LastUsedAt, &t.RevokedAt); err != nil {
		return domain.AgentToken{}, err
	}
	return t, nil
}

// CountActive counts non-revoked tokens for devServerID.
func (s *AgentTokenStore) CountActive(ctx context.Context, tenantID, devServerID string) (int, error) {
	const q = `SELECT count(*) FROM infra.agent_tokens WHERE tenant_id = $1 AND dev_server_id = $2 AND revoked_at IS NULL`
	var n int
	if err := s.pool.QueryRow(ctx, q, tenantID, devServerID).Scan(&n); err != nil {
		return 0, fmt.Errorf("postgres: counting active agent tokens: %w", err)
	}
	return n, nil
}

// Insert persists a new token row. Exactly one of TokenHash/CredentialRefID
// must be set — enforced again by the table's exactly_one_secret_ref CHECK.
func (s *AgentTokenStore) Insert(ctx context.Context, t domain.AgentToken) error {
	const q = `
		INSERT INTO infra.agent_tokens (id, tenant_id, dev_server_id, name, token_hash, credential_ref_id, created_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, '')::uuid, $7)`
	_, err := s.pool.Exec(ctx, q, t.ID, t.TenantID, t.DevServerID, t.Name, t.TokenHash, t.CredentialRefID, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("postgres: inserting agent token: %w", err)
	}
	return nil
}

// ListActive returns every non-revoked token for devServerID, newest first.
func (s *AgentTokenStore) ListActive(ctx context.Context, tenantID, devServerID string) ([]domain.AgentToken, error) {
	const q = `SELECT ` + agentTokenColumns + ` FROM infra.agent_tokens
		WHERE tenant_id = $1 AND dev_server_id = $2 AND revoked_at IS NULL ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, q, tenantID, devServerID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing active agent tokens: %w", err)
	}
	defer rows.Close()

	var out []domain.AgentToken
	for rows.Next() {
		t, err := scanAgentToken(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scanning agent token: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FindActiveByHash looks up a non-revoked direct-websocket token by hash.
func (s *AgentTokenStore) FindActiveByHash(ctx context.Context, hash string) (domain.AgentToken, bool, error) {
	const q = `SELECT ` + agentTokenColumns + ` FROM infra.agent_tokens WHERE token_hash = $1 AND revoked_at IS NULL`
	t, err := scanAgentToken(s.pool.QueryRow(ctx, q, hash))
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.AgentToken{}, false, nil
		}
		return domain.AgentToken{}, false, fmt.Errorf("postgres: finding agent token by hash: %w", err)
	}
	return t, true, nil
}

// ActiveForDevServer returns the most-recently-created non-revoked token
// for a relay-websocket DevServer — SOL-AWS-01's per-dial resolution read.
func (s *AgentTokenStore) ActiveForDevServer(ctx context.Context, tenantID, devServerID string) (domain.AgentToken, bool, error) {
	const q = `SELECT ` + agentTokenColumns + ` FROM infra.agent_tokens
		WHERE tenant_id = $1 AND dev_server_id = $2 AND revoked_at IS NULL
		ORDER BY created_at DESC LIMIT 1`
	t, err := scanAgentToken(s.pool.QueryRow(ctx, q, tenantID, devServerID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.AgentToken{}, false, nil
		}
		return domain.AgentToken{}, false, fmt.Errorf("postgres: finding active agent token for dev server: %w", err)
	}
	return t, true, nil
}

// TouchLastUsed bumps last_used_at to now.
func (s *AgentTokenStore) TouchLastUsed(ctx context.Context, id string) error {
	const q = `UPDATE infra.agent_tokens SET last_used_at = $2 WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id, time.Now())
	if err != nil {
		return fmt.Errorf("postgres: touching agent token last_used_at: %w", err)
	}
	return nil
}

// Revoke sets revoked_at and returns the updated row.
func (s *AgentTokenStore) Revoke(ctx context.Context, tenantID, id string) (domain.AgentToken, error) {
	const q = `UPDATE infra.agent_tokens SET revoked_at = now()
		WHERE tenant_id = $1 AND id = $2 AND revoked_at IS NULL
		RETURNING ` + agentTokenColumns
	t, err := scanAgentToken(s.pool.QueryRow(ctx, q, tenantID, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.AgentToken{}, fmt.Errorf("postgres: agent token %s not found or already revoked", id)
		}
		return domain.AgentToken{}, fmt.Errorf("postgres: revoking agent token: %w", err)
	}
	return t, nil
}
```

Wire `agentTokenStore := infrapostgres.NewAgentTokenStore(pool)` into
`services/infra-fleet-service/cmd/server/main.go` alongside the other
`infrapostgres.New*Store(pool)` calls (TASK-AWS-03-05/06/07 consume it).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go vet ./services/infra-fleet-service/internal/adapter/postgres/...
```

Expected: clean build; `var _ usecase.AgentTokenRepository = (*AgentTokenStore)(nil)`
compiles, confirming full interface satisfaction.

Once a test Postgres is available, add
`adapter/postgres/agent_token_repository_test.go` per SOL-AWS-03's test
plan: `FindActiveByHash`/`CountActive`/`ActiveForDevServer` exclude revoked
rows; RLS smoke test (tenant B cannot see tenant A's rows).
