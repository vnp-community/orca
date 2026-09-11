package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// CompanyRepository implements usecase.CompanyRepository against
// companies (the tenant root, no tenant_id column of its own — mirrors
// internal/adapter/postgres.CompanyRepository's doc comment).
type CompanyRepository struct {
	db *sql.DB
}

func NewCompanyRepository(db *sql.DB) *CompanyRepository {
	return &CompanyRepository{db: db}
}

func (r *CompanyRepository) Create(ctx context.Context, c domain.Company) (domain.Company, error) {
	settingsJSON, err := marshalSettings(c.Settings)
	if err != nil {
		return domain.Company{}, fmt.Errorf("mysql: marshal company settings: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO companies (id, name, settings_json) VALUES (?, ?, ?)
	`, c.ID, c.Name, settingsJSON)
	if err != nil {
		return domain.Company{}, fmt.Errorf("mysql: insert company: %w", err)
	}
	return c, nil
}

func (r *CompanyRepository) Get(ctx context.Context, id string) (domain.Company, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, settings_json FROM companies WHERE id = ?
	`, id)

	var c domain.Company
	var settingsJSON string
	if err := row.Scan(&c.ID, &c.Name, &settingsJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Company{}, false, nil
		}
		return domain.Company{}, false, fmt.Errorf("mysql: query company: %w", err)
	}

	settings, err := unmarshalSettings(settingsJSON)
	if err != nil {
		return domain.Company{}, false, fmt.Errorf("mysql: unmarshal company settings: %w", err)
	}
	c.Settings = settings
	return c, true, nil
}

// List returns every company row, ordered by name — mirrors
// internal/adapter/postgres.CompanyRepository.List's doc comment on the
// admin-gating requirement this pushes onto callers.
func (r *CompanyRepository) List(ctx context.Context) ([]domain.Company, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, settings_json FROM companies ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("mysql: list companies: %w", err)
	}
	defer rows.Close()

	companies := make([]domain.Company, 0)
	for rows.Next() {
		var c domain.Company
		var settingsJSON string
		if err := rows.Scan(&c.ID, &c.Name, &settingsJSON); err != nil {
			return nil, fmt.Errorf("mysql: scan company row: %w", err)
		}
		settings, err := unmarshalSettings(settingsJSON)
		if err != nil {
			return nil, fmt.Errorf("mysql: unmarshal company settings: %w", err)
		}
		c.Settings = settings
		companies = append(companies, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate company rows: %w", err)
	}
	return companies, nil
}

func (r *CompanyRepository) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM companies WHERE id = ?)`, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("mysql: check company existence: %w", err)
	}
	return exists, nil
}

// Update applies patch's non-empty fields via
// COALESCE(NULLIF(?, ”), col) — same convention as the Postgres adapter.
// MySQL has no RETURNING, so this always issues the UPDATE (a no-op patch
// still executes cleanly) and then re-SELECTs by id — found=false means "no
// row with this id", never derived from RowsAffected()==0 (which on MySQL
// counts CHANGED rows, not MATCHED rows, and would misreport a no-op-value
// retry as not-found — see BE-DB-SOL-005/TASK-BE-DB-010's
// UpdateAnnotation finding, the same pitfall this method deliberately
// avoids).
func (r *CompanyRepository) Update(ctx context.Context, id string, patch domain.CompanySettingsPatch) (domain.Company, bool, error) {
	var settingsArg any
	if patch.SettingsJSON != "" {
		settings, err := unmarshalSettings(patch.SettingsJSON)
		if err != nil {
			return domain.Company{}, false, fmt.Errorf("mysql: unmarshal company settings patch: %w", err)
		}
		marshaled, err := marshalSettings(settings)
		if err != nil {
			return domain.Company{}, false, fmt.Errorf("mysql: marshal company settings patch: %w", err)
		}
		settingsArg = marshaled
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE companies
		SET name          = COALESCE(NULLIF(?, ''), name),
		    settings_json = COALESCE(?, settings_json)
		WHERE id = ?
	`, patch.Name, settingsArg, id)
	if err != nil {
		return domain.Company{}, false, fmt.Errorf("mysql: update company: %w", err)
	}

	return r.Get(ctx, id)
}
