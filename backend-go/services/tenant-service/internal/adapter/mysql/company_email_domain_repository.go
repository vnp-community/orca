package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// CompanyEmailDomainRepository implements usecase.CompanyEmailDomainRepository
// against company_email_domains.
type CompanyEmailDomainRepository struct {
	db *sql.DB
}

func NewCompanyEmailDomainRepository(db *sql.DB) *CompanyEmailDomainRepository {
	return &CompanyEmailDomainRepository{db: db}
}

// Add is an upsert on email_domain's own PRIMARY KEY — ON DUPLICATE KEY
// UPDATE is MySQL's equivalent of Postgres's ON CONFLICT ... DO UPDATE, same
// permissive-at-this-layer posture as the Postgres adapter (see
// usecase.CompanyEmailDomainRepository.Add's doc comment).
func (r *CompanyEmailDomainRepository) Add(ctx context.Context, companyID, emailDomain string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO company_email_domains (email_domain, company_id)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE company_id = VALUES(company_id)
	`, emailDomain, companyID)
	if err != nil {
		return fmt.Errorf("mysql: add company email domain: %w", err)
	}
	return nil
}

func (r *CompanyEmailDomainRepository) Remove(ctx context.Context, emailDomain string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM company_email_domains WHERE email_domain = ?`, emailDomain)
	if err != nil {
		return fmt.Errorf("mysql: remove company email domain: %w", err)
	}
	return nil
}

func (r *CompanyEmailDomainRepository) ListForCompany(ctx context.Context, companyID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT email_domain FROM company_email_domains WHERE company_id = ? ORDER BY email_domain
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("mysql: list company email domains: %w", err)
	}
	defer rows.Close()

	domains := make([]string, 0)
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("mysql: scan company email domain row: %w", err)
		}
		domains = append(domains, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate company email domain rows: %w", err)
	}
	return domains, nil
}

func (r *CompanyEmailDomainRepository) ResolveCompanyID(ctx context.Context, emailDomain string) (string, bool, error) {
	var companyID string
	err := r.db.QueryRowContext(ctx, `
		SELECT company_id FROM company_email_domains WHERE email_domain = ?
	`, emailDomain).Scan(&companyID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("mysql: resolve company by email domain: %w", err)
	}
	return companyID, true, nil
}
