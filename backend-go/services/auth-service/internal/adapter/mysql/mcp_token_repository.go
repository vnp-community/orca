package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

// MCP personal access token persistence (BE-MCP-SOL-006), MySQL/TiDB mirror of
// internal/adapter/postgres/mcp_token_repository.go. Each conditional UPDATE
// below changes a NULL column to a value, so RowsAffected() is reliable.

const mcpTokenColumns = `jti, tenant_id, user_id, name, scope, token_sha256, created_at, expires_at,
	first_used_at, last_used_at, revoked_at, COALESCE(revoked_by, '')`

func scanMcpToken(row rowScanner) (domain.McpToken, error) {
	var t domain.McpToken
	var first, last, revoked sql.NullTime
	if err := row.Scan(&t.JTI, &t.TenantID, &t.UserID, &t.Name, &t.Scope, &t.TokenSHA256, &t.CreatedAt, &t.ExpiresAt,
		&first, &last, &revoked, &t.RevokedBy); err != nil {
		return domain.McpToken{}, err
	}
	for _, p := range []struct {
		n sql.NullTime
		d **time.Time
	}{{first, &t.FirstUsedAt}, {last, &t.LastUsedAt}, {revoked, &t.RevokedAt}} {
		if p.n.Valid {
			v := p.n.Time
			*p.d = &v
		}
	}
	return t, nil
}

func (r *Repository) CreateMcpToken(ctx context.Context, t domain.McpToken) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO mcp_tokens (jti, tenant_id, user_id, name, scope, token_sha256, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, t.JTI, t.TenantID, t.UserID, t.Name, t.Scope, t.TokenSHA256, t.CreatedAt, t.ExpiresAt)
	if err != nil {
		return fmt.Errorf("mysql: insert mcp token: %w", err)
	}
	return nil
}

func (r *Repository) ListMcpTokens(ctx context.Context, tenantID, userID string) ([]domain.McpToken, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+mcpTokenColumns+` FROM mcp_tokens
		WHERE tenant_id = ? AND user_id = ? ORDER BY created_at DESC`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("mysql: list mcp tokens: %w", err)
	}
	defer rows.Close()
	var out []domain.McpToken
	for rows.Next() {
		t, err := scanMcpToken(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan mcp token: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) GetMcpToken(ctx context.Context, tenantID, jti string) (domain.McpToken, error) {
	t, err := scanMcpToken(r.db.QueryRowContext(ctx, `SELECT `+mcpTokenColumns+` FROM mcp_tokens WHERE tenant_id = ? AND jti = ?`, tenantID, jti))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.McpToken{}, usecase.ErrMcpTokenNotFound
	}
	if err != nil {
		return domain.McpToken{}, fmt.Errorf("mysql: get mcp token: %w", err)
	}
	return t, nil
}

func (r *Repository) RevokeMcpToken(ctx context.Context, tenantID, userID, jti string, at time.Time) error {
	res, err := r.db.ExecContext(ctx, `UPDATE mcp_tokens SET revoked_at = ?, revoked_by = ?
		WHERE tenant_id = ? AND user_id = ? AND jti = ? AND revoked_at IS NULL`, at, userID, tenantID, userID, jti)
	if err != nil {
		return fmt.Errorf("mysql: revoke mcp token: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return usecase.ErrMcpTokenNotFound
	}
	return nil
}

func (r *Repository) MarkMcpTokenFirstUsed(ctx context.Context, tenantID, jti string, at time.Time) (bool, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE mcp_tokens SET first_used_at = ?
		WHERE tenant_id = ? AND jti = ? AND first_used_at IS NULL`, at, tenantID, jti)
	if err != nil {
		return false, fmt.Errorf("mysql: mark mcp token first used: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (r *Repository) TouchMcpTokenLastUsed(ctx context.Context, tenantID, jti string, at, staleBefore time.Time) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE mcp_tokens SET last_used_at = ?
		WHERE tenant_id = ? AND jti = ? AND (last_used_at IS NULL OR last_used_at < ?)`, at, tenantID, jti, staleBefore); err != nil {
		return fmt.Errorf("mysql: touch mcp token: %w", err)
	}
	return nil
}

var _ usecase.McpTokenRepository = (*Repository)(nil)

func (r *Repository) SetMcpPatSuspension(ctx context.Context, tenantID string, suspended bool, reason string, at time.Time) error {
	var err error
	if suspended {
		_, err = r.db.ExecContext(ctx, `INSERT INTO mcp_pat_suspensions (tenant_id, suspended_at, reason) VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE suspended_at = VALUES(suspended_at), reason = VALUES(reason)`, tenantID, at, reason)
	} else {
		_, err = r.db.ExecContext(ctx, `DELETE FROM mcp_pat_suspensions WHERE tenant_id = ?`, tenantID)
	}
	if err != nil {
		return fmt.Errorf("mysql: set mcp pat suspension: %w", err)
	}
	return nil
}

func (r *Repository) IsMcpPatSuspended(ctx context.Context, tenantID string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM mcp_pat_suspensions WHERE tenant_id = ?`, tenantID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("mysql: read mcp pat suspension: %w", err)
	}
	return true, nil
}

var _ usecase.McpPatSuspensionRepository = (*Repository)(nil)
