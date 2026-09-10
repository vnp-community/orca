package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CompanyEmailDomainRepository implements usecase.CompanyEmailDomainRepository
// against tenant.company_email_domains.
type CompanyEmailDomainRepository struct {
	pool *pgxpool.Pool
}

func NewCompanyEmailDomainRepository(pool *pgxpool.Pool) *CompanyEmailDomainRepository {
	return &CompanyEmailDomainRepository{pool: pool}
}

// Add is an upsert on email_domain's own PRIMARY KEY — re-registering the
// same (companyID, emailDomain) pair is a no-op; re-registering emailDomain
// under a DIFFERENT companyID overwrites it (the usecase layer's
// ResolveCompanyID pre-check is what actually prevents that in practice —
// see AddCompanyEmailDomain's doc comment on why this method itself stays
// permissive).
func (r *CompanyEmailDomainRepository) Add(ctx context.Context, companyID, emailDomain string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tenant.company_email_domains (email_domain, company_id)
		VALUES ($1, $2)
		ON CONFLICT (email_domain) DO UPDATE SET company_id = EXCLUDED.company_id
	`, emailDomain, companyID)
	if err != nil {
		return fmt.Errorf("postgres: add company email domain: %w", err)
	}
	return nil
}

func (r *CompanyEmailDomainRepository) Remove(ctx context.Context, emailDomain string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM tenant.company_email_domains WHERE email_domain = $1`, emailDomain)
	if err != nil {
		return fmt.Errorf("postgres: remove company email domain: %w", err)
	}
	return nil
}

func (r *CompanyEmailDomainRepository) ListForCompany(ctx context.Context, companyID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT email_domain FROM tenant.company_email_domains WHERE company_id = $1 ORDER BY email_domain
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list company email domains: %w", err)
	}
	defer rows.Close()

	domains := make([]string, 0)
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("postgres: scan company email domain row: %w", err)
		}
		domains = append(domains, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate company email domain rows: %w", err)
	}
	return domains, nil
}

func (r *CompanyEmailDomainRepository) ResolveCompanyID(ctx context.Context, emailDomain string) (string, bool, error) {
	var companyID string
	err := r.pool.QueryRow(ctx, `
		SELECT company_id FROM tenant.company_email_domains WHERE email_domain = $1
	`, emailDomain).Scan(&companyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("postgres: resolve company by email domain: %w", err)
	}
	return companyID, true, nil
}
