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

// OAuth authorization-server persistence (BE-MCP-SOL-005). Only hashes of
// codes and refresh tokens are ever passed in. Lookups by hash carry no
// tenant (the /token endpoint is anonymous); the tenant is read from the row.
// Every tenant-scoped statement binds tenant_id explicitly.

const oauthClientColumns = `client_id, client_name, COALESCE(client_uri, ''), redirect_uris, registered_via, created_at, last_used_at`

func scanOAuthClient(row pgx.Row) (domain.OAuthClient, error) {
	var c domain.OAuthClient
	err := row.Scan(&c.ClientID, &c.ClientName, &c.ClientURI, &c.RedirectURIs, &c.RegisteredVia, &c.CreatedAt, &c.LastUsedAt)
	return c, err
}

func (r *Repository) CountOAuthClients(ctx context.Context) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM auth.oauth_clients`).Scan(&n); err != nil {
		return 0, fmt.Errorf("postgres: count oauth clients: %w", err)
	}
	return n, nil
}

func (r *Repository) CreateOAuthClient(ctx context.Context, c domain.OAuthClient) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO auth.oauth_clients (client_id, client_name, client_uri, redirect_uris, registered_via, created_at)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6)
	`, c.ClientID, c.ClientName, c.ClientURI, c.RedirectURIs, c.RegisteredVia, c.CreatedAt)
	if err != nil {
		return fmt.Errorf("postgres: insert oauth client: %w", err)
	}
	return nil
}

func (r *Repository) GetOAuthClient(ctx context.Context, clientID string) (domain.OAuthClient, error) {
	c, err := scanOAuthClient(r.pool.QueryRow(ctx, `SELECT `+oauthClientColumns+` FROM auth.oauth_clients WHERE client_id = $1`, clientID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OAuthClient{}, usecase.ErrOAuthClientNotFound
	}
	if err != nil {
		return domain.OAuthClient{}, fmt.Errorf("postgres: get oauth client: %w", err)
	}
	return c, nil
}

func (r *Repository) TouchOAuthClientUsed(ctx context.Context, clientID string, at time.Time) error {
	if _, err := r.pool.Exec(ctx, `UPDATE auth.oauth_clients SET last_used_at = $2 WHERE client_id = $1`, clientID, at); err != nil {
		return fmt.Errorf("postgres: touch oauth client: %w", err)
	}
	return nil
}

const oauthStatusColumns = `tenant_id::text, client_id, status, COALESCE(updated_by::text, ''), updated_at`

func scanOAuthStatus(row pgx.Row) (domain.OAuthClientTenantStatus, error) {
	var s domain.OAuthClientTenantStatus
	var status string
	err := row.Scan(&s.TenantID, &s.ClientID, &status, &s.UpdatedBy, &s.UpdatedAt)
	s.Status = domain.OAuthClientStatus(status)
	return s, err
}

func (r *Repository) EnsureOAuthClientTenantStatus(ctx context.Context, s domain.OAuthClientTenantStatus) (domain.OAuthClientTenantStatus, error) {
	// DO NOTHING keeps an existing row (possibly 'blocked') untouched.
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO auth.oauth_client_tenant_status (tenant_id, client_id, status, updated_by, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5)
		ON CONFLICT (tenant_id, client_id) DO NOTHING
	`, s.TenantID, s.ClientID, string(s.Status), s.UpdatedBy, s.UpdatedAt); err != nil {
		return domain.OAuthClientTenantStatus{}, fmt.Errorf("postgres: ensure oauth client status: %w", err)
	}
	return r.GetOAuthClientTenantStatus(ctx, s.TenantID, s.ClientID)
}

func (r *Repository) GetOAuthClientTenantStatus(ctx context.Context, tenantID, clientID string) (domain.OAuthClientTenantStatus, error) {
	s, err := scanOAuthStatus(r.pool.QueryRow(ctx,
		`SELECT `+oauthStatusColumns+` FROM auth.oauth_client_tenant_status WHERE tenant_id = $1 AND client_id = $2`, tenantID, clientID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OAuthClientTenantStatus{}, usecase.ErrOAuthClientStatusNotFound
	}
	if err != nil {
		return domain.OAuthClientTenantStatus{}, fmt.Errorf("postgres: get oauth client status: %w", err)
	}
	return s, nil
}

func (r *Repository) SetOAuthClientTenantStatus(ctx context.Context, s domain.OAuthClientTenantStatus) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE auth.oauth_client_tenant_status
		SET status = $3, updated_by = NULLIF($4, '')::uuid, updated_at = $5
		WHERE tenant_id = $1 AND client_id = $2
	`, s.TenantID, s.ClientID, string(s.Status), s.UpdatedBy, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: set oauth client status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return usecase.ErrOAuthClientStatusNotFound
	}
	return nil
}

func (r *Repository) ListOAuthClientViews(ctx context.Context, tenantID string) ([]domain.OAuthClientView, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.client_id, c.client_name, COALESCE(c.client_uri, ''), c.redirect_uris, c.registered_via, c.created_at, c.last_used_at,
		       s.status, COALESCE(s.updated_by::text, ''), s.updated_at
		FROM auth.oauth_client_tenant_status s
		JOIN auth.oauth_clients c ON c.client_id = s.client_id
		WHERE s.tenant_id = $1
		ORDER BY c.created_at DESC, c.client_id
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list oauth clients for tenant: %w", err)
	}
	defer rows.Close()
	var out []domain.OAuthClientView
	for rows.Next() {
		var v domain.OAuthClientView
		var status string
		c := &v.Client
		if err := rows.Scan(&c.ClientID, &c.ClientName, &c.ClientURI, &c.RedirectURIs, &c.RegisteredVia, &c.CreatedAt, &c.LastUsedAt,
			&status, &v.Status.UpdatedBy, &v.Status.UpdatedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan oauth client view: %w", err)
		}
		v.Status.TenantID, v.Status.ClientID, v.Status.Status = tenantID, c.ClientID, domain.OAuthClientStatus(status)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	return nil
}

func (r *Repository) CreateOAuthFamilyWithCode(ctx context.Context, f domain.OAuthTokenFamily, c domain.OAuthAuthCode) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO auth.oauth_token_families (family_id, tenant_id, user_id, client_id, grant_id, scope, resource, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, f.FamilyID, f.TenantID, f.UserID, f.ClientID, f.GrantID, f.Scope, f.Resource, f.CreatedAt); err != nil {
			return fmt.Errorf("postgres: insert oauth family: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO auth.oauth_auth_codes (code_hash, family_id, tenant_id, redirect_uri, code_challenge, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, c.CodeHash, c.FamilyID, c.TenantID, c.RedirectURI, c.CodeChallenge, c.ExpiresAt); err != nil {
			return fmt.Errorf("postgres: insert oauth auth code: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetOAuthAuthCode(ctx context.Context, codeHash string) (domain.OAuthAuthCode, error) {
	var c domain.OAuthAuthCode
	err := r.pool.QueryRow(ctx, `
		SELECT code_hash, family_id::text, tenant_id::text, redirect_uri, code_challenge, expires_at, used_at
		FROM auth.oauth_auth_codes WHERE code_hash = $1
	`, codeHash).Scan(&c.CodeHash, &c.FamilyID, &c.TenantID, &c.RedirectURI, &c.CodeChallenge, &c.ExpiresAt, &c.UsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OAuthAuthCode{}, usecase.ErrOAuthCodeNotFound
	}
	if err != nil {
		return domain.OAuthAuthCode{}, fmt.Errorf("postgres: get oauth auth code: %w", err)
	}
	return c, nil
}

func (r *Repository) GetOAuthFamily(ctx context.Context, familyID string) (domain.OAuthTokenFamily, error) {
	var f domain.OAuthTokenFamily
	err := r.pool.QueryRow(ctx, `
		SELECT family_id::text, tenant_id::text, user_id::text, client_id, grant_id::text, scope, resource,
		       created_at, revoked_at, COALESCE(revoke_reason, '')
		FROM auth.oauth_token_families WHERE family_id = $1
	`, familyID).Scan(&f.FamilyID, &f.TenantID, &f.UserID, &f.ClientID, &f.GrantID, &f.Scope, &f.Resource,
		&f.CreatedAt, &f.RevokedAt, &f.RevokeReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OAuthTokenFamily{}, usecase.ErrOAuthFamilyNotFound
	}
	if err != nil {
		return domain.OAuthTokenFamily{}, fmt.Errorf("postgres: get oauth family: %w", err)
	}
	return f, nil
}

func insertOAuthRefresh(ctx context.Context, tx pgx.Tx, t domain.OAuthRefreshToken) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO auth.oauth_refresh_tokens (token_hash, family_id, tenant_id, expires_at)
		VALUES ($1, $2, $3, $4)
	`, t.TokenHash, t.FamilyID, t.TenantID, t.ExpiresAt); err != nil {
		return fmt.Errorf("postgres: insert oauth refresh token: %w", err)
	}
	return nil
}

func (r *Repository) ClaimOAuthAuthCode(ctx context.Context, codeHash string, at time.Time, first domain.OAuthRefreshToken) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		// The single-statement conditional update is the only thing that
		// decides which of two concurrent exchanges wins the code.
		tag, err := tx.Exec(ctx, `UPDATE auth.oauth_auth_codes SET used_at = $2 WHERE code_hash = $1 AND used_at IS NULL`, codeHash, at)
		if err != nil {
			return fmt.Errorf("postgres: claim oauth auth code: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return usecase.ErrOAuthCodeAlreadyUsed
		}
		return insertOAuthRefresh(ctx, tx, first)
	})
}

func (r *Repository) GetOAuthRefreshToken(ctx context.Context, tokenHash string) (domain.OAuthRefreshToken, error) {
	var t domain.OAuthRefreshToken
	err := r.pool.QueryRow(ctx, `
		SELECT token_hash, family_id::text, tenant_id::text, expires_at, used_at, COALESCE(replaced_by_hash, '')
		FROM auth.oauth_refresh_tokens WHERE token_hash = $1
	`, tokenHash).Scan(&t.TokenHash, &t.FamilyID, &t.TenantID, &t.ExpiresAt, &t.UsedAt, &t.ReplacedByHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OAuthRefreshToken{}, usecase.ErrOAuthRefreshNotFound
	}
	if err != nil {
		return domain.OAuthRefreshToken{}, fmt.Errorf("postgres: get oauth refresh token: %w", err)
	}
	return t, nil
}

func (r *Repository) RotateOAuthRefreshToken(ctx context.Context, oldHash string, at time.Time, next domain.OAuthRefreshToken) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE auth.oauth_refresh_tokens SET used_at = $2, replaced_by_hash = $3
			WHERE token_hash = $1 AND used_at IS NULL
		`, oldHash, at, next.TokenHash)
		if err != nil {
			return fmt.Errorf("postgres: rotate oauth refresh token: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return usecase.ErrOAuthRefreshAlreadyUsed
		}
		return insertOAuthRefresh(ctx, tx, next)
	})
}

func (r *Repository) RevokeOAuthFamily(ctx context.Context, familyID, reason string, at time.Time) error {
	if _, err := r.pool.Exec(ctx, `
		UPDATE auth.oauth_token_families SET revoked_at = $3, revoke_reason = $2
		WHERE family_id = $1 AND revoked_at IS NULL
	`, familyID, reason, at); err != nil {
		return fmt.Errorf("postgres: revoke oauth family: %w", err)
	}
	return nil
}

func (r *Repository) RevokeOAuthGrant(ctx context.Context, tenantID, grantID, reason string, at time.Time) (int, error) {
	n := 0
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO auth.oauth_grant_revocations (tenant_id, grant_id, revoked_at) VALUES ($1, $2, $3)
			ON CONFLICT (tenant_id, grant_id) DO NOTHING
		`, tenantID, grantID, at); err != nil {
			return fmt.Errorf("postgres: insert oauth grant revocation: %w", err)
		}
		tag, err := tx.Exec(ctx, `
			UPDATE auth.oauth_token_families SET revoked_at = $3, revoke_reason = $4
			WHERE tenant_id = $1 AND grant_id = $2 AND revoked_at IS NULL
		`, tenantID, grantID, at, reason)
		if err != nil {
			return fmt.Errorf("postgres: revoke oauth families for grant: %w", err)
		}
		n = int(tag.RowsAffected())
		return nil
	})
	return n, err
}

func (r *Repository) RevokeOAuthFamiliesForClient(ctx context.Context, tenantID, clientID, reason string, at time.Time) (int, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE auth.oauth_token_families SET revoked_at = $3, revoke_reason = $4
		WHERE tenant_id = $1 AND client_id = $2 AND revoked_at IS NULL
	`, tenantID, clientID, at, reason)
	if err != nil {
		return 0, fmt.Errorf("postgres: revoke oauth families for client: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (r *Repository) IsOAuthGrantRevoked(ctx context.Context, tenantID, grantID string) (bool, error) {
	var revoked bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM auth.oauth_grant_revocations WHERE tenant_id = $1 AND grant_id = $2)
	`, tenantID, grantID).Scan(&revoked)
	if err != nil {
		return false, fmt.Errorf("postgres: check oauth grant revocation: %w", err)
	}
	return revoked, nil
}

var _ usecase.OAuthRepository = (*Repository)(nil)
