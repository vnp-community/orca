package mysql

import (
	"context"
	"errors"
	"fmt"

	sqldriver "github.com/go-sql-driver/mysql"

	"database/sql"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

// mysqlDuplicateEntry is go-sql-driver/mysql's error number for a
// unique-constraint violation (ER_DUP_ENTRY) — MySQL's equivalent of
// Postgres's SQLSTATE 23505, used by internal/adapter/postgres's
// pgUniqueViolation.
const mysqlDuplicateEntry = 1062

func (r *Repository) CreateUser(ctx context.Context, user domain.User, passwordHash string) (domain.User, error) {
	// sso_provider is never set here — see postgres/user_repository.go's
	// identical comment (this INSERT shape is shared by both CreateUser
	// and LoginOrProvisionSsoUser's reuse of it).
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, tenant_id, email, name, password_hash, role, is_active, created_at)
		VALUES (?,?,?,?,?,?,?,?)
	`, user.ID, user.TenantID, user.Email, user.Name, passwordHash, string(user.Role), user.IsActive, user.CreatedAt)
	if err != nil {
		var mysqlErr *sqldriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlDuplicateEntry {
			return domain.User{}, fmt.Errorf("mysql: insert user: %w", usecase.ErrUserAlreadyExists)
		}
		return domain.User{}, fmt.Errorf("mysql: insert user: %w", err)
	}
	return user, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (domain.User, string, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, email, name, password_hash, role, is_active, created_at, COALESCE(sso_provider, '')
		FROM users
		WHERE email = ?
	`, email)
	return scanUserWithHash(row)
}

func (r *Repository) GetUserByID(ctx context.Context, userID string) (domain.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, email, name, password_hash, role, is_active, created_at, COALESCE(sso_provider, '')
		FROM users
		WHERE id = ?
	`, userID)
	user, _, err := scanUserWithHash(row)
	return user, err
}

func (r *Repository) ListUsers(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.User, string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, email, name, role, is_active, created_at, COALESCE(sso_provider, '')
		FROM users
		WHERE tenant_id = ? AND id > ?
		ORDER BY id
		LIMIT ?
	`, tenantID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query users: %w", err)
	}
	defer rows.Close()

	var out []domain.User
	for rows.Next() {
		var u domain.User
		var role, ssoProvider string
		if err := rows.Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &role, &u.IsActive, &u.CreatedAt, &ssoProvider); err != nil {
			return nil, "", fmt.Errorf("mysql: scan user row: %w", err)
		}
		u.Role = domain.Role(role)
		u.SsoProvider = domain.SsoProvider(ssoProvider)
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate user rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// UpdateUserRole does NOT branch on UPDATE's RowsAffected()==0 to detect
// "not found" (unlike a naive $N->? translation of the Postgres variant's
// RETURNING-based read): MySQL's default RowsAffected() semantics count
// only rows whose VALUES actually changed, not rows matched by WHERE
// (Postgres/pgx's RETURNING has no such ambiguity — it returns a row
// whenever WHERE matches, changed or not). Setting a user's role to the
// role it already has — a legitimate idempotent admin-console retry —
// would incorrectly report ErrUserNotFound under a RowsAffected-based
// translation. Instead: UPDATE unconditionally, then re-SELECT by id —
// mirrors annotation-service's BE-DB-SOL-005 §3.1 fix for the identical
// class of bug, applied here because auth-service's UpdateUserRole/
// UpdateUser is a security-relevant admin operation (role changes), not
// just data hygiene.
func (r *Repository) UpdateUserRole(ctx context.Context, userID string, role domain.Role) (domain.User, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, string(role), userID); err != nil {
		return domain.User{}, fmt.Errorf("mysql: update user role: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, email, name, role, is_active, created_at, COALESCE(sso_provider, '')
		FROM users WHERE id = ?
	`, userID)
	var u domain.User
	var roleStr, ssoProvider string
	err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &roleStr, &u.IsActive, &u.CreatedAt, &ssoProvider)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, fmt.Errorf("mysql: update user role: %w", usecase.ErrUserNotFound)
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("mysql: update user role: %w", err)
	}
	u.Role = domain.Role(roleStr)
	u.SsoProvider = domain.SsoProvider(ssoProvider)
	return u, nil
}

// SetSsoProvider — 0 rows affected (a userID that doesn't exist) is not
// treated as an error, matching SetActive's own idempotent-update
// contract (see usecase.UserRepository's doc comment) — no RowsAffected
// branch exists here in either dialect, so there is no ambiguity to
// translate.
func (r *Repository) SetSsoProvider(ctx context.Context, userID string, provider domain.SsoProvider) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET sso_provider = ? WHERE id = ?`, string(provider), userID)
	if err != nil {
		return fmt.Errorf("mysql: set user sso_provider: %w", err)
	}
	return nil
}

func (r *Repository) HasAnyUsers(ctx context.Context) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users LIMIT 1)`).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("mysql: checking for any users: %w", err)
	}
	return exists, nil
}

// SetActive flips a user's is_active flag — idempotent, matching
// usecase.UserRepository's doc comment; no not-found detection here in
// either dialect (the caller re-reads afterward), so no RowsAffected
// ambiguity to translate.
func (r *Repository) SetActive(ctx context.Context, userID string, active bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET is_active = ? WHERE id = ?`, active, userID)
	if err != nil {
		return fmt.Errorf("mysql: set user active: %w", err)
	}
	return nil
}

// UpdateUser applies a partial update via COALESCE, same as the Postgres
// variant. See UpdateUserRole's doc comment above for why this UPDATEs
// unconditionally then re-SELECTs by id, rather than branching on
// RowsAffected()==0.
func (r *Repository) UpdateUser(ctx context.Context, userID string, email, name *string, role *domain.Role) (domain.User, error) {
	var roleStr *string
	if role != nil {
		s := string(*role)
		roleStr = &s
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET email = COALESCE(?, email), name = COALESCE(?, name), role = COALESCE(?, role)
		WHERE id = ?
	`, email, name, roleStr, userID); err != nil {
		return domain.User{}, fmt.Errorf("mysql: update user: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, email, name, role, is_active, created_at
		FROM users WHERE id = ?
	`, userID)
	var u domain.User
	var scannedRole string
	err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &scannedRole, &u.IsActive, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, fmt.Errorf("mysql: update user: %w", usecase.ErrUserNotFound)
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("mysql: scan updated user row: %w", err)
	}
	u.Role = domain.Role(scannedRole)
	return u, nil
}

// Count returns the total number of users across every tenant.
func (r *Repository) Count(ctx context.Context) (int32, error) {
	var n int32
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("mysql: count users: %w", err)
	}
	return n, nil
}

func scanUserWithHash(row rowScanner) (domain.User, string, error) {
	var u domain.User
	var role, passwordHash, ssoProvider string
	err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &passwordHash, &role, &u.IsActive, &u.CreatedAt, &ssoProvider)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, "", fmt.Errorf("mysql: query user: %w", usecase.ErrUserNotFound)
	}
	if err != nil {
		return domain.User{}, "", fmt.Errorf("mysql: scan user row: %w", err)
	}
	u.Role = domain.Role(role)
	u.SsoProvider = domain.SsoProvider(ssoProvider)
	return u, passwordHash, nil
}
