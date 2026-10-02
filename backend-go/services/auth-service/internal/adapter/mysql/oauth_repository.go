package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

// OAuth authorization-server persistence (BE-MCP-SOL-005), MySQL/TiDB mirror
// of internal/adapter/postgres/oauth_repository.go. Every "UPDATE ... WHERE
// x IS NULL" below changes the column from NULL to a value, so RowsAffected()
// is reliable even under MySQL's changed-rows-only default.

type txBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

func (r *Repository) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	b, ok := r.db.(txBeginner)
	if !ok {
		return errors.New("mysql: repository handle cannot begin transactions")
	}
	tx, err := b.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit: %w", err)
	}
	return nil
}

const oauthClientColumns = `client_id, client_name, COALESCE(client_uri, ''), redirect_uris, registered_via, created_at, last_used_at`

func scanOAuthClient(row rowScanner) (domain.OAuthClient, error) {
	var c domain.OAuthClient
	var uris []byte
	var last sql.NullTime
	if err := row.Scan(&c.ClientID, &c.ClientName, &c.ClientURI, &uris, &c.RegisteredVia, &c.CreatedAt, &last); err != nil {
		return domain.OAuthClient{}, err
	}
	if err := json.Unmarshal(uris, &c.RedirectURIs); err != nil {
		return domain.OAuthClient{}, fmt.Errorf("decode redirect_uris: %w", err)
	}
	if last.Valid {
		t := last.Time
		c.LastUsedAt = &t
	}
	return c, nil
}

func (r *Repository) CountOAuthClients(ctx context.Context) (int, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM oauth_clients`).Scan(&n); err != nil {
		return 0, fmt.Errorf("mysql: count oauth clients: %w", err)
	}
	return n, nil
}

func (r *Repository) CreateOAuthClient(ctx context.Context, c domain.OAuthClient) error {
	uris, err := json.Marshal(c.RedirectURIs)
	if err != nil {
		return fmt.Errorf("mysql: encode redirect_uris: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO oauth_clients (client_id, client_name, client_uri, redirect_uris, registered_via, created_at)
		VALUES (?, ?, NULLIF(?, ''), ?, ?, ?)
	`, c.ClientID, c.ClientName, c.ClientURI, uris, c.RegisteredVia, c.CreatedAt)
	if err != nil {
		return fmt.Errorf("mysql: insert oauth client: %w", err)
	}
	return nil
}

func (r *Repository) GetOAuthClient(ctx context.Context, clientID string) (domain.OAuthClient, error) {
	c, err := scanOAuthClient(r.db.QueryRowContext(ctx, `SELECT `+oauthClientColumns+` FROM oauth_clients WHERE client_id = ?`, clientID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OAuthClient{}, usecase.ErrOAuthClientNotFound
	}
	if err != nil {
		return domain.OAuthClient{}, fmt.Errorf("mysql: get oauth client: %w", err)
	}
	return c, nil
}

func (r *Repository) TouchOAuthClientUsed(ctx context.Context, clientID string, at time.Time) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE oauth_clients SET last_used_at = ? WHERE client_id = ?`, at, clientID); err != nil {
		return fmt.Errorf("mysql: touch oauth client: %w", err)
	}
	return nil
}

const oauthStatusColumns = `tenant_id, client_id, status, COALESCE(updated_by, ''), updated_at`

func scanOAuthStatus(row rowScanner) (domain.OAuthClientTenantStatus, error) {
	var s domain.OAuthClientTenantStatus
	var status string
	err := row.Scan(&s.TenantID, &s.ClientID, &status, &s.UpdatedBy, &s.UpdatedAt)
	s.Status = domain.OAuthClientStatus(status)
	return s, err
}

func (r *Repository) EnsureOAuthClientTenantStatus(ctx context.Context, s domain.OAuthClientTenantStatus) (domain.OAuthClientTenantStatus, error) {
	// The no-op ON DUPLICATE KEY keeps an existing row (possibly 'blocked') untouched.
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO oauth_client_tenant_status (tenant_id, client_id, status, updated_by, updated_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?)
		ON DUPLICATE KEY UPDATE tenant_id = tenant_id
	`, s.TenantID, s.ClientID, string(s.Status), s.UpdatedBy, s.UpdatedAt); err != nil {
		return domain.OAuthClientTenantStatus{}, fmt.Errorf("mysql: ensure oauth client status: %w", err)
	}
	return r.GetOAuthClientTenantStatus(ctx, s.TenantID, s.ClientID)
}

func (r *Repository) GetOAuthClientTenantStatus(ctx context.Context, tenantID, clientID string) (domain.OAuthClientTenantStatus, error) {
	s, err := scanOAuthStatus(r.db.QueryRowContext(ctx,
		`SELECT `+oauthStatusColumns+` FROM oauth_client_tenant_status WHERE tenant_id = ? AND client_id = ?`, tenantID, clientID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OAuthClientTenantStatus{}, usecase.ErrOAuthClientStatusNotFound
	}
	if err != nil {
		return domain.OAuthClientTenantStatus{}, fmt.Errorf("mysql: get oauth client status: %w", err)
	}
	return s, nil
}

func (r *Repository) SetOAuthClientTenantStatus(ctx context.Context, s domain.OAuthClientTenantStatus) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE oauth_client_tenant_status SET status = ?, updated_by = NULLIF(?, ''), updated_at = ?
		WHERE tenant_id = ? AND client_id = ?
	`, string(s.Status), s.UpdatedBy, s.UpdatedAt, s.TenantID, s.ClientID); err != nil {
		return fmt.Errorf("mysql: set oauth client status: %w", err)
	}
	// RowsAffected() can be 0 for an existing row whose values did not
	// change, so existence is confirmed separately.
	_, err := r.GetOAuthClientTenantStatus(ctx, s.TenantID, s.ClientID)
	return err
}

func (r *Repository) ListOAuthClientViews(ctx context.Context, tenantID string) ([]domain.OAuthClientView, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.client_id, c.client_name, COALESCE(c.client_uri, ''), c.redirect_uris, c.registered_via, c.created_at, c.last_used_at,
		       s.status, COALESCE(s.updated_by, ''), s.updated_at
		FROM oauth_client_tenant_status s
		JOIN oauth_clients c ON c.client_id = s.client_id
		WHERE s.tenant_id = ?
		ORDER BY c.created_at DESC, c.client_id
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: list oauth clients for tenant: %w", err)
	}
	defer rows.Close()
	var out []domain.OAuthClientView
	for rows.Next() {
		var v domain.OAuthClientView
		var uris []byte
		var last sql.NullTime
		var status string
		c := &v.Client
		if err := rows.Scan(&c.ClientID, &c.ClientName, &c.ClientURI, &uris, &c.RegisteredVia, &c.CreatedAt, &last,
			&status, &v.Status.UpdatedBy, &v.Status.UpdatedAt); err != nil {
			return nil, fmt.Errorf("mysql: scan oauth client view: %w", err)
		}
		if err := json.Unmarshal(uris, &c.RedirectURIs); err != nil {
			return nil, fmt.Errorf("mysql: decode redirect_uris: %w", err)
		}
		if last.Valid {
			t := last.Time
			c.LastUsedAt = &t
		}
		v.Status.TenantID, v.Status.ClientID, v.Status.Status = tenantID, c.ClientID, domain.OAuthClientStatus(status)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) CreateOAuthFamilyWithCode(ctx context.Context, f domain.OAuthTokenFamily, c domain.OAuthAuthCode) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO oauth_token_families (family_id, tenant_id, user_id, client_id, grant_id, scope, resource, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, f.FamilyID, f.TenantID, f.UserID, f.ClientID, f.GrantID, f.Scope, f.Resource, f.CreatedAt); err != nil {
			return fmt.Errorf("mysql: insert oauth family: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO oauth_auth_codes (code_hash, family_id, tenant_id, redirect_uri, code_challenge, expires_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, c.CodeHash, c.FamilyID, c.TenantID, c.RedirectURI, c.CodeChallenge, c.ExpiresAt); err != nil {
			return fmt.Errorf("mysql: insert oauth auth code: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetOAuthAuthCode(ctx context.Context, codeHash string) (domain.OAuthAuthCode, error) {
	var c domain.OAuthAuthCode
	var used sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT code_hash, family_id, tenant_id, redirect_uri, code_challenge, expires_at, used_at
		FROM oauth_auth_codes WHERE code_hash = ?
	`, codeHash).Scan(&c.CodeHash, &c.FamilyID, &c.TenantID, &c.RedirectURI, &c.CodeChallenge, &c.ExpiresAt, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OAuthAuthCode{}, usecase.ErrOAuthCodeNotFound
	}
	if err != nil {
		return domain.OAuthAuthCode{}, fmt.Errorf("mysql: get oauth auth code: %w", err)
	}
	if used.Valid {
		t := used.Time
		c.UsedAt = &t
	}
	return c, nil
}

func (r *Repository) GetOAuthFamily(ctx context.Context, familyID string) (domain.OAuthTokenFamily, error) {
	var f domain.OAuthTokenFamily
	var revoked sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT family_id, tenant_id, user_id, client_id, grant_id, scope, resource, created_at, revoked_at, COALESCE(revoke_reason, '')
		FROM oauth_token_families WHERE family_id = ?
	`, familyID).Scan(&f.FamilyID, &f.TenantID, &f.UserID, &f.ClientID, &f.GrantID, &f.Scope, &f.Resource, &f.CreatedAt, &revoked, &f.RevokeReason)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OAuthTokenFamily{}, usecase.ErrOAuthFamilyNotFound
	}
	if err != nil {
		return domain.OAuthTokenFamily{}, fmt.Errorf("mysql: get oauth family: %w", err)
	}
	if revoked.Valid {
		t := revoked.Time
		f.RevokedAt = &t
	}
	return f, nil
}

func insertOAuthRefresh(ctx context.Context, tx *sql.Tx, t domain.OAuthRefreshToken) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO oauth_refresh_tokens (token_hash, family_id, tenant_id, expires_at) VALUES (?, ?, ?, ?)
	`, t.TokenHash, t.FamilyID, t.TenantID, t.ExpiresAt); err != nil {
		return fmt.Errorf("mysql: insert oauth refresh token: %w", err)
	}
	return nil
}

func (r *Repository) ClaimOAuthAuthCode(ctx context.Context, codeHash string, at time.Time, first domain.OAuthRefreshToken) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE oauth_auth_codes SET used_at = ? WHERE code_hash = ? AND used_at IS NULL`, at, codeHash)
		if err != nil {
			return fmt.Errorf("mysql: claim oauth auth code: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return usecase.ErrOAuthCodeAlreadyUsed
		}
		return insertOAuthRefresh(ctx, tx, first)
	})
}

func (r *Repository) GetOAuthRefreshToken(ctx context.Context, tokenHash string) (domain.OAuthRefreshToken, error) {
	var t domain.OAuthRefreshToken
	var used sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT token_hash, family_id, tenant_id, expires_at, used_at, COALESCE(replaced_by_hash, '')
		FROM oauth_refresh_tokens WHERE token_hash = ?
	`, tokenHash).Scan(&t.TokenHash, &t.FamilyID, &t.TenantID, &t.ExpiresAt, &used, &t.ReplacedByHash)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OAuthRefreshToken{}, usecase.ErrOAuthRefreshNotFound
	}
	if err != nil {
		return domain.OAuthRefreshToken{}, fmt.Errorf("mysql: get oauth refresh token: %w", err)
	}
	if used.Valid {
		u := used.Time
		t.UsedAt = &u
	}
	return t, nil
}

func (r *Repository) RotateOAuthRefreshToken(ctx context.Context, oldHash string, at time.Time, next domain.OAuthRefreshToken) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE oauth_refresh_tokens SET used_at = ?, replaced_by_hash = ? WHERE token_hash = ? AND used_at IS NULL
		`, at, next.TokenHash, oldHash)
		if err != nil {
			return fmt.Errorf("mysql: rotate oauth refresh token: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return usecase.ErrOAuthRefreshAlreadyUsed
		}
		return insertOAuthRefresh(ctx, tx, next)
	})
}

func (r *Repository) RevokeOAuthFamily(ctx context.Context, familyID, reason string, at time.Time) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE oauth_token_families SET revoked_at = ?, revoke_reason = ? WHERE family_id = ? AND revoked_at IS NULL
	`, at, reason, familyID); err != nil {
		return fmt.Errorf("mysql: revoke oauth family: %w", err)
	}
	return nil
}

func (r *Repository) RevokeOAuthGrant(ctx context.Context, tenantID, grantID, reason string, at time.Time) (int, error) {
	n := 0
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO oauth_grant_revocations (tenant_id, grant_id, revoked_at) VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE tenant_id = tenant_id
		`, tenantID, grantID, at); err != nil {
			return fmt.Errorf("mysql: insert oauth grant revocation: %w", err)
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE oauth_token_families SET revoked_at = ?, revoke_reason = ?
			WHERE tenant_id = ? AND grant_id = ? AND revoked_at IS NULL
		`, at, reason, tenantID, grantID)
		if err != nil {
			return fmt.Errorf("mysql: revoke oauth families for grant: %w", err)
		}
		c, _ := res.RowsAffected()
		n = int(c)
		return nil
	})
	return n, err
}

func (r *Repository) RevokeOAuthFamiliesForClient(ctx context.Context, tenantID, clientID, reason string, at time.Time) (int, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE oauth_token_families SET revoked_at = ?, revoke_reason = ?
		WHERE tenant_id = ? AND client_id = ? AND revoked_at IS NULL
	`, at, reason, tenantID, clientID)
	if err != nil {
		return 0, fmt.Errorf("mysql: revoke oauth families for client: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *Repository) IsOAuthGrantRevoked(ctx context.Context, tenantID, grantID string) (bool, error) {
	var n int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM oauth_grant_revocations WHERE tenant_id = ? AND grant_id = ?`, tenantID, grantID).Scan(&n); err != nil {
		return false, fmt.Errorf("mysql: check oauth grant revocation: %w", err)
	}
	return n > 0, nil
}

var _ usecase.OAuthRepository = (*Repository)(nil)
