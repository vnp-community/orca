package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// DepartmentRepository implements usecase.DepartmentRepository against
// departments, always scoped by company_id — same not-found-not-
// wrong-company posture as internal/adapter/postgres.DepartmentRepository
// (tenant-service.md §9): a department_id from another company must resolve
// as not-found, so Get filters by (company_id, id) in the same query rather
// than filtering after the fact.
type DepartmentRepository struct {
	db *sql.DB
}

func NewDepartmentRepository(db *sql.DB) *DepartmentRepository {
	return &DepartmentRepository{db: db}
}

func (r *DepartmentRepository) Create(ctx context.Context, d domain.Department) (domain.Department, error) {
	settingsJSON, err := marshalSettings(d.Settings)
	if err != nil {
		return domain.Department{}, fmt.Errorf("mysql: marshal department settings: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO departments (id, company_id, name, settings_json) VALUES (?, ?, ?, ?)
	`, d.ID, d.CompanyID, d.Name, settingsJSON)
	if err != nil {
		return domain.Department{}, fmt.Errorf("mysql: insert department: %w", err)
	}
	return d, nil
}

func (r *DepartmentRepository) Get(ctx context.Context, companyID, id string) (domain.Department, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, company_id, name, settings_json
		FROM departments
		WHERE company_id = ? AND id = ?
	`, companyID, id)

	var d domain.Department
	var settingsJSON string
	if err := row.Scan(&d.ID, &d.CompanyID, &d.Name, &settingsJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Department{}, false, nil
		}
		return domain.Department{}, false, fmt.Errorf("mysql: query department: %w", err)
	}

	settings, err := unmarshalSettings(settingsJSON)
	if err != nil {
		return domain.Department{}, false, fmt.Errorf("mysql: unmarshal department settings: %w", err)
	}
	d.Settings = settings
	return d, true, nil
}

// ExistsByName backs CreateDepartment's name-uniqueness check.
func (r *DepartmentRepository) ExistsByName(ctx context.Context, companyID, name string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM departments WHERE company_id = ? AND name = ?)
	`, companyID, name).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("mysql: check department name exists: %w", err)
	}
	return exists, nil
}

func (r *DepartmentRepository) List(ctx context.Context, companyID string) ([]domain.Department, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, company_id, name, settings_json
		FROM departments
		WHERE company_id = ?
		ORDER BY id
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query departments: %w", err)
	}
	defer rows.Close()

	var out []domain.Department
	for rows.Next() {
		var d domain.Department
		var settingsJSON string
		if err := rows.Scan(&d.ID, &d.CompanyID, &d.Name, &settingsJSON); err != nil {
			return nil, fmt.Errorf("mysql: scan department row: %w", err)
		}
		settings, err := unmarshalSettings(settingsJSON)
		if err != nil {
			return nil, fmt.Errorf("mysql: unmarshal department settings: %w", err)
		}
		d.Settings = settings
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate department rows: %w", err)
	}
	return out, nil
}

// Update applies patch's non-empty fields, scoped by (companyID, id). No
// RETURNING on MySQL — always issues the UPDATE, then re-SELECTs by
// (companyID, id); found=false means "no matching row", never derived from
// RowsAffected() (see CompanyRepository.Update's doc comment on why).
func (r *DepartmentRepository) Update(ctx context.Context, companyID, id string, patch domain.DepartmentSettingsPatch) (domain.Department, bool, error) {
	var settingsArg any
	if patch.SettingsJSON != "" {
		settings, err := unmarshalSettings(patch.SettingsJSON)
		if err != nil {
			return domain.Department{}, false, fmt.Errorf("mysql: unmarshal department settings patch: %w", err)
		}
		marshaled, err := marshalSettings(settings)
		if err != nil {
			return domain.Department{}, false, fmt.Errorf("mysql: marshal department settings patch: %w", err)
		}
		settingsArg = marshaled
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE departments
		SET name          = COALESCE(NULLIF(?, ''), name),
		    settings_json = COALESCE(?, settings_json)
		WHERE company_id = ? AND id = ?
	`, patch.Name, settingsArg, companyID, id)
	if err != nil {
		return domain.Department{}, false, fmt.Errorf("mysql: update department: %w", err)
	}

	return r.Get(ctx, companyID, id)
}
