package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

func (r *Repository) FindByProviderSubject(ctx context.Context, provider domain.SsoProvider, externalSubject string) (domain.SsoIdentity, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, tenant_id, provider, external_subject, email_at_link, created_at, last_login_at
		FROM auth.sso_identities
		WHERE provider = $1 AND external_subject = $2
	`, string(provider), externalSubject)

	var id domain.SsoIdentity
	var providerStr string
	err := row.Scan(&id.ID, &id.UserID, &id.TenantID, &providerStr, &id.ExternalSubject, &id.EmailAtLink, &id.CreatedAt, &id.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SsoIdentity{}, fmt.Errorf("postgres: query sso identity: %w", usecase.ErrSsoIdentityNotFound)
	}
	if err != nil {
		return domain.SsoIdentity{}, fmt.Errorf("postgres: scan sso identity row: %w", err)
	}
	id.Provider = domain.SsoProvider(providerStr)
	return id, nil
}

func (r *Repository) Link(ctx context.Context, identity domain.SsoIdentity) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO auth.sso_identities (id, user_id, tenant_id, provider, external_subject, email_at_link, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, identity.ID, identity.UserID, identity.TenantID, string(identity.Provider), identity.ExternalSubject, identity.EmailAtLink, identity.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("postgres: link sso identity: identity already linked: %w", err)
		}
		return fmt.Errorf("postgres: link sso identity: %w", err)
	}
	return nil
}

func (r *Repository) TouchLastLogin(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE auth.sso_identities SET last_login_at = $2 WHERE id = $1`, id, at)
	if err != nil {
		return fmt.Errorf("postgres: touch sso identity last_login_at: %w", err)
	}
	return nil
}
