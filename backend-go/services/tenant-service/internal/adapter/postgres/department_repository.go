package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// DepartmentRepository implements usecase.DepartmentRepository against
// tenant.departments, always scoped by company_id — see tenant-service.md
// §9: a department_id from another company must resolve as not-found, not
// "wrong company", so Get filters by (company_id, id) in the same query
// rather than filtering after the fact.
type DepartmentRepository struct {
	pool *pgxpool.Pool
}

func NewDepartmentRepository(pool *pgxpool.Pool) *DepartmentRepository {
	return &DepartmentRepository{pool: pool}
}

func (r *DepartmentRepository) Create(ctx context.Context, d domain.Department) (domain.Department, error) {
	settingsJSON, err := marshalSettings(d.Settings)
	if err != nil {
		return domain.Department{}, fmt.Errorf("postgres: marshal department settings: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO tenant.departments (id, company_id, name, settings_json) VALUES ($1, $2, $3, $4)
	`, d.ID, d.CompanyID, d.Name, settingsJSON)
	if err != nil {
		return domain.Department{}, fmt.Errorf("postgres: insert department: %w", err)
	}
	return d, nil
}

func (r *DepartmentRepository) Get(ctx context.Context, companyID, id string) (domain.Department, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, company_id, name, settings_json
		FROM tenant.departments
		WHERE company_id = $1 AND id = $2
	`, companyID, id)

	var d domain.Department
	var settingsJSON string
	if err := row.Scan(&d.ID, &d.CompanyID, &d.Name, &settingsJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Department{}, false, nil
		}
		return domain.Department{}, false, fmt.Errorf("postgres: query department: %w", err)
	}

	settings, err := unmarshalSettings(settingsJSON)
	if err != nil {
		return domain.Department{}, false, fmt.Errorf("postgres: unmarshal department settings: %w", err)
	}
	d.Settings = settings
	return d, true, nil
}

func (r *DepartmentRepository) List(ctx context.Context, companyID string) ([]domain.Department, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, company_id, name, settings_json
		FROM tenant.departments
		WHERE company_id = $1
		ORDER BY id
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query departments: %w", err)
	}
	defer rows.Close()

	var out []domain.Department
	for rows.Next() {
		var d domain.Department
		var settingsJSON string
		if err := rows.Scan(&d.ID, &d.CompanyID, &d.Name, &settingsJSON); err != nil {
			return nil, fmt.Errorf("postgres: scan department row: %w", err)
		}
		settings, err := unmarshalSettings(settingsJSON)
		if err != nil {
			return nil, fmt.Errorf("postgres: unmarshal department settings: %w", err)
		}
		d.Settings = settings
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate department rows: %w", err)
	}
	return out, nil
}

// Update applies patch's non-empty fields, scoped by (companyID, id) — a
// department belonging to a different company resolves as not-found, same
// isolation rule as Get.
func (r *DepartmentRepository) Update(ctx context.Context, companyID, id string, patch domain.DepartmentSettingsPatch) (domain.Department, bool, error) {
	var settingsArg any
	if patch.SettingsJSON != "" {
		settings, err := unmarshalSettings(patch.SettingsJSON)
		if err != nil {
			return domain.Department{}, false, fmt.Errorf("postgres: unmarshal department settings patch: %w", err)
		}
		marshaled, err := marshalSettings(settings)
		if err != nil {
			return domain.Department{}, false, fmt.Errorf("postgres: marshal department settings patch: %w", err)
		}
		settingsArg = marshaled
	}

	row := r.pool.QueryRow(ctx, `
		UPDATE tenant.departments
		SET name          = COALESCE(NULLIF($3, ''), name),
		    settings_json = COALESCE($4, settings_json)
		WHERE company_id = $1 AND id = $2
		RETURNING id, company_id, name, settings_json
	`, companyID, id, patch.Name, settingsArg)

	var d domain.Department
	var settingsJSON string
	if err := row.Scan(&d.ID, &d.CompanyID, &d.Name, &settingsJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Department{}, false, nil
		}
		return domain.Department{}, false, fmt.Errorf("postgres: update department: %w", err)
	}
	settings, err := unmarshalSettings(settingsJSON)
	if err != nil {
		return domain.Department{}, false, fmt.Errorf("postgres: unmarshal department settings: %w", err)
	}
	d.Settings = settings
	return d, true, nil
}
