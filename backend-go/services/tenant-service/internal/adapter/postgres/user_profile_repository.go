package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// UserProfileRepository implements usecase.UserProfileRepository against
// tenant.user_profiles — 1:1 with a user, logical FK to auth-service.
type UserProfileRepository struct {
	pool *pgxpool.Pool
}

func NewUserProfileRepository(pool *pgxpool.Pool) *UserProfileRepository {
	return &UserProfileRepository{pool: pool}
}

// Upsert creates or updates a user's profile row, keyed on user_id.
func (r *UserProfileRepository) Upsert(ctx context.Context, p domain.UserProfile) error {
	settingsJSON, err := marshalSettings(p.Settings)
	if err != nil {
		return fmt.Errorf("postgres: marshal user profile settings: %w", err)
	}
	departmentID := nullableString(p.DepartmentID)

	_, err = r.pool.Exec(ctx, `
		INSERT INTO tenant.user_profiles (user_id, company_id, department_id, settings_json)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			company_id     = EXCLUDED.company_id,
			department_id  = EXCLUDED.department_id,
			settings_json  = EXCLUDED.settings_json,
			updated_at     = now()
	`, p.UserID, p.CompanyID, departmentID, settingsJSON)
	if err != nil {
		return fmt.Errorf("postgres: upsert user profile: %w", err)
	}
	return nil
}

// Get looks up userID's profile, scoped by companyID — a profile row that
// exists but belongs to a different company resolves as not-found, same
// isolation rule as DepartmentRepository/TeamRepository (tenant-service.md §9).
func (r *UserProfileRepository) Get(ctx context.Context, companyID, userID string) (domain.UserProfile, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT user_id, company_id, department_id, settings_json
		FROM tenant.user_profiles
		WHERE user_id = $1 AND company_id = $2
	`, userID, companyID)

	var p domain.UserProfile
	var departmentID *string
	var settingsJSON string
	if err := row.Scan(&p.UserID, &p.CompanyID, &departmentID, &settingsJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.UserProfile{}, false, nil
		}
		return domain.UserProfile{}, false, fmt.Errorf("postgres: query user profile: %w", err)
	}
	if departmentID != nil {
		p.DepartmentID = *departmentID
	}

	settings, err := unmarshalSettings(settingsJSON)
	if err != nil {
		return domain.UserProfile{}, false, fmt.Errorf("postgres: unmarshal user profile settings: %w", err)
	}
	p.Settings = settings
	return p, true, nil
}

func (r *UserProfileRepository) ListUserIDsByDepartment(ctx context.Context, companyID, departmentID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT user_id FROM tenant.user_profiles
		WHERE company_id = $1 AND department_id = $2
	`, companyID, departmentID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query user ids by department: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("postgres: scan user id row: %w", err)
		}
		out = append(out, uid)
	}
	return out, rows.Err()
}

func (r *UserProfileRepository) ListUserIDsByCompany(ctx context.Context, companyID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT user_id FROM tenant.user_profiles WHERE company_id = $1
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query user ids by company: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("postgres: scan user id row: %w", err)
		}
		out = append(out, uid)
	}
	return out, rows.Err()
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
