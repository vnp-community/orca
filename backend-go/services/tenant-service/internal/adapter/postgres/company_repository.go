package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// CompanyRepository implements usecase.CompanyRepository against
// tenant.companies — the tenant root, with no tenant_id column of its own
// (tenant-service.md §5).
type CompanyRepository struct {
	pool *pgxpool.Pool
}

func NewCompanyRepository(pool *pgxpool.Pool) *CompanyRepository {
	return &CompanyRepository{pool: pool}
}

func (r *CompanyRepository) Create(ctx context.Context, c domain.Company) (domain.Company, error) {
	settingsJSON, err := marshalSettings(c.Settings)
	if err != nil {
		return domain.Company{}, fmt.Errorf("postgres: marshal company settings: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO tenant.companies (id, name, settings_json) VALUES ($1, $2, $3)
	`, c.ID, c.Name, settingsJSON)
	if err != nil {
		return domain.Company{}, fmt.Errorf("postgres: insert company: %w", err)
	}
	return c, nil
}

func (r *CompanyRepository) Get(ctx context.Context, id string) (domain.Company, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, settings_json FROM tenant.companies WHERE id = $1
	`, id)

	var c domain.Company
	var settingsJSON string
	if err := row.Scan(&c.ID, &c.Name, &settingsJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Company{}, false, nil
		}
		return domain.Company{}, false, fmt.Errorf("postgres: query company: %w", err)
	}

	settings, err := unmarshalSettings(settingsJSON)
	if err != nil {
		return domain.Company{}, false, fmt.Errorf("postgres: unmarshal company settings: %w", err)
	}
	c.Settings = settings
	return c, true, nil
}

func (r *CompanyRepository) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenant.companies WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres: check company existence: %w", err)
	}
	return exists, nil
}

// Update applies patch's non-empty fields via COALESCE(NULLIF($n, ”), col)
// — mirrors project-service.Repository.UpdateProject's convention.
// Returns found=false (no error) if id doesn't match any row.
func (r *CompanyRepository) Update(ctx context.Context, id string, patch domain.CompanySettingsPatch) (domain.Company, bool, error) {
	var settingsArg any
	if patch.SettingsJSON != "" {
		settings, err := unmarshalSettings(patch.SettingsJSON)
		if err != nil {
			return domain.Company{}, false, fmt.Errorf("postgres: unmarshal company settings patch: %w", err)
		}
		marshaled, err := marshalSettings(settings)
		if err != nil {
			return domain.Company{}, false, fmt.Errorf("postgres: marshal company settings patch: %w", err)
		}
		settingsArg = marshaled
	}

	row := r.pool.QueryRow(ctx, `
		UPDATE tenant.companies
		SET name          = COALESCE(NULLIF($2, ''), name),
		    settings_json = COALESCE($3, settings_json)
		WHERE id = $1
		RETURNING id, name, settings_json
	`, id, patch.Name, settingsArg)

	var c domain.Company
	var settingsJSON string
	if err := row.Scan(&c.ID, &c.Name, &settingsJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Company{}, false, nil
		}
		return domain.Company{}, false, fmt.Errorf("postgres: update company: %w", err)
	}
	settings, err := unmarshalSettings(settingsJSON)
	if err != nil {
		return domain.Company{}, false, fmt.Errorf("postgres: unmarshal company settings: %w", err)
	}
	c.Settings = settings
	return c, true, nil
}
