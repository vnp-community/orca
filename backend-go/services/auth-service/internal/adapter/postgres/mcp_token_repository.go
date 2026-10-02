package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

const mcpTokenColumns = `jti, tenant_id::text, user_id::text, name, scope, token_sha256, created_at, expires_at,
	first_used_at, last_used_at, revoked_at, COALESCE(revoked_by::text, '')`

func scanMcpToken(row pgx.Row) (domain.McpToken, error) {
	var t domain.McpToken
	err := row.Scan(&t.JTI, &t.TenantID, &t.UserID, &t.Name, &t.Scope, &t.TokenSHA256, &t.CreatedAt, &t.ExpiresAt,
		&t.FirstUsedAt, &t.LastUsedAt, &t.RevokedAt, &t.RevokedBy)
	return t, err
}

func (r *Repository) CreateMcpToken(ctx context.Context, t domain.McpToken) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO auth.mcp_tokens (jti, tenant_id, user_id, name, scope, token_sha256, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, t.JTI, t.TenantID, t.UserID, t.Name, t.Scope, t.TokenSHA256, t.CreatedAt, t.ExpiresAt)
	if err != nil {
		return fmt.Errorf("postgres: insert mcp token: %w", err)
	}
	return nil
}

func (r *Repository) ListMcpTokens(ctx context.Context, tenantID, userID string) ([]domain.McpToken, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+mcpTokenColumns+` FROM auth.mcp_tokens
		WHERE tenant_id = $1 AND user_id = $2 ORDER BY created_at DESC`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list mcp tokens: %w", err)
	}
	defer rows.Close()
	var out []domain.McpToken
	for rows.Next() {
		t, err := scanMcpToken(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan mcp token: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) GetMcpToken(ctx context.Context, tenantID, jti string) (domain.McpToken, error) {
	t, err := scanMcpToken(r.pool.QueryRow(ctx, `SELECT `+mcpTokenColumns+` FROM auth.mcp_tokens WHERE tenant_id = $1 AND jti = $2`, tenantID, jti))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.McpToken{}, usecase.ErrMcpTokenNotFound
	}
	if err != nil {
		return domain.McpToken{}, fmt.Errorf("postgres: get mcp token: %w", err)
	}
	return t, nil
}

func (r *Repository) RevokeMcpToken(ctx context.Context, tenantID, userID, jti string, at time.Time) error {
	tag, err := r.pool.Exec(ctx, `UPDATE auth.mcp_tokens SET revoked_at = $4, revoked_by = $2
		WHERE tenant_id = $1 AND user_id = $2 AND jti = $3 AND revoked_at IS NULL`, tenantID, userID, jti, at)
	if err != nil {
		return fmt.Errorf("postgres: revoke mcp token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return usecase.ErrMcpTokenNotFound
	}
	return nil
}

func (r *Repository) MarkMcpTokenFirstUsed(ctx context.Context, tenantID, jti string, at time.Time) (bool, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE auth.mcp_tokens SET first_used_at = $3
		WHERE tenant_id = $1 AND jti = $2 AND first_used_at IS NULL`, tenantID, jti, at)
	if err != nil {
		return false, fmt.Errorf("postgres: mark mcp token first used: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Repository) TouchMcpTokenLastUsed(ctx context.Context, tenantID, jti string, at, staleBefore time.Time) error {
	if _, err := r.pool.Exec(ctx, `UPDATE auth.mcp_tokens SET last_used_at = $3
		WHERE tenant_id = $1 AND jti = $2 AND (last_used_at IS NULL OR last_used_at < $4)`, tenantID, jti, at, staleBefore); err != nil {
		return fmt.Errorf("postgres: touch mcp token: %w", err)
	}
	return nil
}

var _ usecase.McpTokenRepository = (*Repository)(nil)
