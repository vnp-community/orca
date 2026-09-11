package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

var _ usecase.AgentTokenRepository = (*AgentTokenStore)(nil)

// AgentTokenStore implements usecase.AgentTokenRepository against
// agent_tokens (migrations/mysql/0020).
type AgentTokenStore struct {
	db *sql.DB
}

func NewAgentTokenStore(db *sql.DB) *AgentTokenStore {
	return &AgentTokenStore{db: db}
}

// credential_ref_id needs no ::text cast unlike Postgres — CHAR(36) already
// coerces to a string inside COALESCE.
const agentTokenColumns = `id, tenant_id, dev_server_id, name, COALESCE(token_hash, ''), COALESCE(credential_ref_id, ''), created_at, last_used_at, revoked_at`

func scanAgentToken(row rowScanner) (domain.AgentToken, error) {
	var t domain.AgentToken
	if err := row.Scan(&t.ID, &t.TenantID, &t.DevServerID, &t.Name, &t.TokenHash, &t.CredentialRefID, &t.CreatedAt, &t.LastUsedAt, &t.RevokedAt); err != nil {
		return domain.AgentToken{}, err
	}
	return t, nil
}

// CountActive counts non-revoked tokens for devServerID.
func (s *AgentTokenStore) CountActive(ctx context.Context, tenantID, devServerID string) (int, error) {
	const q = `SELECT count(*) FROM agent_tokens WHERE tenant_id = ? AND dev_server_id = ? AND revoked_at IS NULL`
	var n int
	if err := s.db.QueryRowContext(ctx, q, tenantID, devServerID).Scan(&n); err != nil {
		return 0, fmt.Errorf("mysql: counting active agent tokens: %w", err)
	}
	return n, nil
}

// Insert persists a new token row. Exactly one of TokenHash/CredentialRefID
// must be set — enforced again by the table's exactly_one_secret_ref CHECK.
func (s *AgentTokenStore) Insert(ctx context.Context, t domain.AgentToken) error {
	const q = `
		INSERT INTO agent_tokens (id, tenant_id, dev_server_id, name, token_hash, credential_ref_id, created_at)
		VALUES (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)`
	_, err := s.db.ExecContext(ctx, q, t.ID, t.TenantID, t.DevServerID, t.Name, t.TokenHash, t.CredentialRefID, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("mysql: inserting agent token: %w", err)
	}
	return nil
}

// ListActive returns every non-revoked token for devServerID, newest first.
func (s *AgentTokenStore) ListActive(ctx context.Context, tenantID, devServerID string) ([]domain.AgentToken, error) {
	const q = `SELECT ` + agentTokenColumns + ` FROM agent_tokens
		WHERE tenant_id = ? AND dev_server_id = ? AND revoked_at IS NULL ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, tenantID, devServerID)
	if err != nil {
		return nil, fmt.Errorf("mysql: listing active agent tokens: %w", err)
	}
	defer rows.Close()

	var out []domain.AgentToken
	for rows.Next() {
		t, err := scanAgentToken(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scanning agent token: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FindActiveByHash looks up a non-revoked direct-websocket token by hash.
func (s *AgentTokenStore) FindActiveByHash(ctx context.Context, hash string) (domain.AgentToken, bool, error) {
	const q = `SELECT ` + agentTokenColumns + ` FROM agent_tokens WHERE token_hash = ? AND revoked_at IS NULL`
	t, err := scanAgentToken(s.db.QueryRowContext(ctx, q, hash))
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.AgentToken{}, false, nil
		}
		return domain.AgentToken{}, false, fmt.Errorf("mysql: finding agent token by hash: %w", err)
	}
	return t, true, nil
}

// ActiveForDevServer returns the most-recently-created non-revoked token
// for a relay-websocket DevServer — SOL-AWS-01's per-dial resolution read.
func (s *AgentTokenStore) ActiveForDevServer(ctx context.Context, tenantID, devServerID string) (domain.AgentToken, bool, error) {
	const q = `SELECT ` + agentTokenColumns + ` FROM agent_tokens
		WHERE tenant_id = ? AND dev_server_id = ? AND revoked_at IS NULL
		ORDER BY created_at DESC LIMIT 1`
	t, err := scanAgentToken(s.db.QueryRowContext(ctx, q, tenantID, devServerID))
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.AgentToken{}, false, nil
		}
		return domain.AgentToken{}, false, fmt.Errorf("mysql: finding active agent token for dev server: %w", err)
	}
	return t, true, nil
}

// TouchLastUsed bumps last_used_at to now.
func (s *AgentTokenStore) TouchLastUsed(ctx context.Context, id string) error {
	const q = `UPDATE agent_tokens SET last_used_at = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, time.Now(), id)
	if err != nil {
		return fmt.Errorf("mysql: touching agent token last_used_at: %w", err)
	}
	return nil
}

// Revoke sets revoked_at and returns the updated row. Unlike
// UpdateAnnotation's pitfall (BE-DB-SOL-005 §3.1), revoked_at always
// transitions NULL -> a concrete timestamp here, never a same-value retry,
// so RowsAffected() == 0 unambiguously means "not found or already
// revoked" on both dialects — safe to use directly, no SELECT-first needed.
func (s *AgentTokenStore) Revoke(ctx context.Context, tenantID, id string) (domain.AgentToken, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE agent_tokens SET revoked_at = ?
		WHERE tenant_id = ? AND id = ? AND revoked_at IS NULL`, time.Now(), tenantID, id)
	if err != nil {
		return domain.AgentToken{}, fmt.Errorf("mysql: revoking agent token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.AgentToken{}, fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return domain.AgentToken{}, fmt.Errorf("mysql: agent token %s not found or already revoked", id)
	}
	t, err := scanAgentToken(s.db.QueryRowContext(ctx, `SELECT `+agentTokenColumns+` FROM agent_tokens WHERE tenant_id = ? AND id = ?`, tenantID, id))
	if err != nil {
		return domain.AgentToken{}, fmt.Errorf("mysql: reading back revoked agent token: %w", err)
	}
	return t, nil
}
