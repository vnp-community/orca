package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// UserProfileRepository implements usecase.UserProfileRepository against
// user_profiles — 1:1 with a user, logical FK to auth-service. Also
// implements usecase.ClientStateRepository (5 opaque per-user JSON columns,
// see GetClientStateColumn/SetClientStateColumn below) — same
// dual-interface struct shape as internal/adapter/postgres.UserProfileRepository.
type UserProfileRepository struct {
	db *sql.DB
}

func NewUserProfileRepository(db *sql.DB) *UserProfileRepository {
	return &UserProfileRepository{db: db}
}

// Upsert creates or updates a user's profile row, keyed on user_id (the
// PRIMARY KEY). ON DUPLICATE KEY UPDATE is MySQL's equivalent of Postgres's
// ON CONFLICT (user_id) DO UPDATE.
func (r *UserProfileRepository) Upsert(ctx context.Context, p domain.UserProfile) error {
	settingsJSON, err := marshalSettings(p.Settings)
	if err != nil {
		return fmt.Errorf("mysql: marshal user profile settings: %w", err)
	}
	departmentID := nullableString(p.DepartmentID)

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO user_profiles (user_id, company_id, department_id, settings_json)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			company_id     = VALUES(company_id),
			department_id  = VALUES(department_id),
			settings_json  = VALUES(settings_json),
			updated_at     = NOW(6)
	`, p.UserID, p.CompanyID, departmentID, settingsJSON)
	if err != nil {
		return fmt.Errorf("mysql: upsert user profile: %w", err)
	}
	return nil
}

// Get looks up userID's profile, scoped by companyID — a profile row that
// exists but belongs to a different company resolves as not-found, same
// isolation rule as DepartmentRepository/TeamRepository (tenant-service.md §9).
func (r *UserProfileRepository) Get(ctx context.Context, companyID, userID string) (domain.UserProfile, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT user_id, company_id, department_id, settings_json
		FROM user_profiles
		WHERE user_id = ? AND company_id = ?
	`, userID, companyID)

	var p domain.UserProfile
	var departmentID *string
	var settingsJSON string
	if err := row.Scan(&p.UserID, &p.CompanyID, &departmentID, &settingsJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.UserProfile{}, false, nil
		}
		return domain.UserProfile{}, false, fmt.Errorf("mysql: query user profile: %w", err)
	}
	if departmentID != nil {
		p.DepartmentID = *departmentID
	}

	settings, err := unmarshalSettings(settingsJSON)
	if err != nil {
		return domain.UserProfile{}, false, fmt.Errorf("mysql: unmarshal user profile settings: %w", err)
	}
	p.Settings = settings
	return p, true, nil
}

func (r *UserProfileRepository) ListUserIDsByDepartment(ctx context.Context, companyID, departmentID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_id FROM user_profiles
		WHERE company_id = ? AND department_id = ?
	`, companyID, departmentID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query user ids by department: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("mysql: scan user id row: %w", err)
		}
		out = append(out, uid)
	}
	return out, rows.Err()
}

func (r *UserProfileRepository) ListUserIDsByCompany(ctx context.Context, companyID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_id FROM user_profiles WHERE company_id = ?
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query user ids by company: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("mysql: scan user id row: %w", err)
		}
		out = append(out, uid)
	}
	return out, rows.Err()
}

// GetOnboardingState reads userID's stored onboarding progress. Returns
// found=false when no profile row exists at all OR the row exists but
// onboarding_state_json is NULL (never saved) — both mean "wizard not
// started", the caller's existing default.
func (r *UserProfileRepository) GetOnboardingState(ctx context.Context, companyID, userID string) (string, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT onboarding_state_json FROM user_profiles
		WHERE user_id = ? AND company_id = ?
	`, userID, companyID)

	var stateJSON *string
	if err := row.Scan(&stateJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("mysql: query onboarding state: %w", err)
	}
	if stateJSON == nil {
		return "", false, nil
	}
	return *stateJSON, true, nil
}

// SetOnboardingState upserts ONLY onboarding_state_json — see
// usecase.UserProfileRepository's doc comment on why this isn't routed
// through Upsert. A brand-new row gets company_id from companyID and leaves
// department_id/settings_json at their column defaults (NULL/JSON_OBJECT(),
// see migrations/mysql/0001_init.up.sql); an existing row's
// department_id/settings_json are left untouched (ON DUPLICATE KEY UPDATE
// only sets onboarding_state_json/updated_at).
func (r *UserProfileRepository) SetOnboardingState(ctx context.Context, companyID, userID, stateJSON string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_profiles (user_id, company_id, onboarding_state_json)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
			onboarding_state_json = VALUES(onboarding_state_json),
			updated_at            = NOW(6)
	`, userID, companyID, stateJSON)
	if err != nil {
		return fmt.Errorf("mysql: set onboarding state: %w", err)
	}
	return nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// clientStateColumn mirrors internal/adapter/postgres's own whitelist type
// 1:1 — never built from unchecked input, only ever returned by
// columnNameFor's explicit switch below. See that file's doc comment for
// why GetClientStateColumn/SetClientStateColumn's public signatures stay
// plain strings.
type clientStateColumn string

const (
	columnKeybindings              clientStateColumn = "keybindings_json"
	columnUILocalState             clientStateColumn = "ui_local_state_json"
	columnSavedRuntimeEnvironments clientStateColumn = "saved_runtime_environments_json"
	columnClientSettings           clientStateColumn = "client_settings_json"
	columnAccountsDevServerMap     clientStateColumn = "accounts_dev_server_json"
)

// columnNameFor is the ONLY place allowed to turn an external "which column"
// string into a SQL identifier — column names can't be bound as ? query
// parameters, so this explicit switch (not string interpolation of the
// input directly) is what stands between a caller and SQL injection via
// column name, same non-negotiable requirement as the Postgres adapter.
func columnNameFor(column string) (clientStateColumn, bool) {
	switch clientStateColumn(column) {
	case columnKeybindings, columnUILocalState, columnSavedRuntimeEnvironments,
		columnClientSettings, columnAccountsDevServerMap:
		return clientStateColumn(column), true
	default:
		return "", false
	}
}

// GetClientStateColumn reads one of the 5 opaque per-user JSON columns —
// same found=false semantics as GetOnboardingState (no row OR NULL column
// both mean "never saved").
func (r *UserProfileRepository) GetClientStateColumn(ctx context.Context, companyID, userID, column string) (string, bool, error) {
	col, ok := columnNameFor(column)
	if !ok {
		return "", false, fmt.Errorf("mysql: unknown client state column %q", column)
	}

	query := fmt.Sprintf(`SELECT %s FROM user_profiles WHERE user_id = ? AND company_id = ?`, col)
	row := r.db.QueryRowContext(ctx, query, userID, companyID)

	var valueJSON *string
	if err := row.Scan(&valueJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("mysql: query client state column %s: %w", col, err)
	}
	if valueJSON == nil {
		return "", false, nil
	}
	return *valueJSON, true, nil
}

// SetClientStateColumn upserts ONLY the named column, same partial-update
// shape as SetOnboardingState.
func (r *UserProfileRepository) SetClientStateColumn(ctx context.Context, companyID, userID, column, valueJSON string) error {
	col, ok := columnNameFor(column)
	if !ok {
		return fmt.Errorf("mysql: unknown client state column %q", column)
	}

	query := fmt.Sprintf(`
		INSERT INTO user_profiles (user_id, company_id, %s)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
			%s         = VALUES(%s),
			updated_at = NOW(6)
	`, col, col, col)
	if _, err := r.db.ExecContext(ctx, query, userID, companyID, valueJSON); err != nil {
		return fmt.Errorf("mysql: set client state column %s: %w", col, err)
	}
	return nil
}
