package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sqldriver "github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

func (r *Repository) FindByProviderSubject(ctx context.Context, provider domain.SsoProvider, externalSubject string) (domain.SsoIdentity, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, tenant_id, provider, external_subject, email_at_link, created_at, last_login_at
		FROM sso_identities
		WHERE provider = ? AND external_subject = ?
	`, string(provider), externalSubject)

	var id domain.SsoIdentity
	var providerStr string
	err := row.Scan(&id.ID, &id.UserID, &id.TenantID, &providerStr, &id.ExternalSubject, &id.EmailAtLink, &id.CreatedAt, &id.LastLoginAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SsoIdentity{}, fmt.Errorf("mysql: query sso identity: %w", usecase.ErrSsoIdentityNotFound)
	}
	if err != nil {
		return domain.SsoIdentity{}, fmt.Errorf("mysql: scan sso identity row: %w", err)
	}
	id.Provider = domain.SsoProvider(providerStr)
	return id, nil
}

func (r *Repository) Link(ctx context.Context, identity domain.SsoIdentity) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sso_identities (id, user_id, tenant_id, provider, external_subject, email_at_link, created_at)
		VALUES (?,?,?,?,?,?,?)
	`, identity.ID, identity.UserID, identity.TenantID, string(identity.Provider), identity.ExternalSubject, identity.EmailAtLink, identity.CreatedAt)
	if err != nil {
		var mysqlErr *sqldriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlDuplicateEntry {
			return fmt.Errorf("mysql: link sso identity: identity already linked: %w", err)
		}
		return fmt.Errorf("mysql: link sso identity: %w", err)
	}
	return nil
}

func (r *Repository) TouchLastLogin(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sso_identities SET last_login_at = ? WHERE id = ?`, at, id)
	if err != nil {
		return fmt.Errorf("mysql: touch sso identity last_login_at: %w", err)
	}
	return nil
}
