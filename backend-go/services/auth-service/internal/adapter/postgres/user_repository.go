package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

// pgUniqueViolation is Postgres's SQLSTATE for a unique-constraint
// violation — see https://www.postgresql.org/docs/current/errcodes-appendix.html.
const pgUniqueViolation = "23505"

func (r *Repository) CreateUser(ctx context.Context, user domain.User, passwordHash string) (domain.User, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO auth.users (id, tenant_id, email, name, password_hash, role, is_active, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, user.ID, user.TenantID, user.Email, user.Name, passwordHash, string(user.Role), user.IsActive, user.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.User{}, fmt.Errorf("postgres: insert user: %w", usecase.ErrUserAlreadyExists)
		}
		return domain.User{}, fmt.Errorf("postgres: insert user: %w", err)
	}
	return user, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (domain.User, string, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, email, name, password_hash, role, is_active, created_at
		FROM auth.users
		WHERE email = $1
	`, email)
	return scanUserWithHash(row)
}

func (r *Repository) GetUserByID(ctx context.Context, userID string) (domain.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, email, name, password_hash, role, is_active, created_at
		FROM auth.users
		WHERE id = $1
	`, userID)
	user, _, err := scanUserWithHash(row)
	return user, err
}

func (r *Repository) ListUsers(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.User, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, email, name, role, is_active, created_at
		FROM auth.users
		WHERE tenant_id = $1 AND id::text > $2
		ORDER BY id
		LIMIT $3
	`, tenantID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query users: %w", err)
	}
	defer rows.Close()

	var out []domain.User
	for rows.Next() {
		var u domain.User
		var role string
		if err := rows.Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &role, &u.IsActive, &u.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("postgres: scan user row: %w", err)
		}
		u.Role = domain.Role(role)
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate user rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func (r *Repository) UpdateUserRole(ctx context.Context, userID string, role domain.Role) (domain.User, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE auth.users SET role = $2
		WHERE id = $1
		RETURNING id, tenant_id, email, name, role, is_active, created_at
	`, userID, string(role))

	var u domain.User
	var roleStr string
	err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &roleStr, &u.IsActive, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, fmt.Errorf("postgres: update user role: %w", usecase.ErrUserNotFound)
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("postgres: update user role: %w", err)
	}
	u.Role = domain.Role(roleStr)
	return u, nil
}

func (r *Repository) HasAnyUsers(ctx context.Context) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth.users LIMIT 1)`).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres: checking for any users: %w", err)
	}
	return exists, nil
}

// SetActive flips a user's is_active flag — idempotent, matching
// usecase.UserRepository's doc comment; a userID that doesn't exist yet
// affects 0 rows, which is not treated as an error here (the caller,
// DeactivateUser/ReactivateUser, re-reads the user afterward and surfaces
// ErrUserNotFound from that read instead).
func (r *Repository) SetActive(ctx context.Context, userID string, active bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE auth.users SET is_active = $2 WHERE id = $1`, userID, active)
	if err != nil {
		return fmt.Errorf("postgres: set user active: %w", err)
	}
	return nil
}

// UpdateUser applies a partial update — nil fields are left unchanged via
// COALESCE. Distinct from UpdateUserRole above (kept as-is).
func (r *Repository) UpdateUser(ctx context.Context, userID string, email, name *string, role *domain.Role) (domain.User, error) {
	var roleStr *string
	if role != nil {
		s := string(*role)
		roleStr = &s
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE auth.users
		SET email = COALESCE($2, email), name = COALESCE($3, name), role = COALESCE($4, role)
		WHERE id = $1
		RETURNING id, tenant_id, email, name, role, is_active, created_at
	`, userID, email, name, roleStr)

	var u domain.User
	var scannedRole string
	err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &scannedRole, &u.IsActive, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, fmt.Errorf("postgres: update user: %w", usecase.ErrUserNotFound)
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("postgres: scan updated user row: %w", err)
	}
	u.Role = domain.Role(scannedRole)
	return u, nil
}

// Count returns the total number of users across every tenant.
func (r *Repository) Count(ctx context.Context) (int32, error) {
	var n int32
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM auth.users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("postgres: count users: %w", err)
	}
	return n, nil
}

// rowScanner is satisfied by both pgx.Row and pgx.Rows, letting
// scanUserWithHash serve both QueryRow and Query call sites.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanUserWithHash(row rowScanner) (domain.User, string, error) {
	var u domain.User
	var role, passwordHash string
	err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &passwordHash, &role, &u.IsActive, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, "", fmt.Errorf("postgres: query user: %w", usecase.ErrUserNotFound)
	}
	if err != nil {
		return domain.User{}, "", fmt.Errorf("postgres: scan user row: %w", err)
	}
	u.Role = domain.Role(role)
	return u, passwordHash, nil
}
