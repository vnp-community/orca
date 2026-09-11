package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

// InsertPolicyVersion appends a new (id, version) row — never an UPDATE,
// per auth-service.md:150's append-only versioning contract.
func (r *Repository) InsertPolicyVersion(ctx context.Context, p domain.AccessPolicy) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO access_policies (id, name, kind, document, version, updated_by, updated_at)
		VALUES (?,?,?,?,?,?,?)
	`, p.ID, p.Name, p.Kind, p.DocumentJSON, p.Version, p.UpdatedBy, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("mysql: insert access policy version: %w", err)
	}
	return nil
}

// GetLatestPolicy returns the highest-version row for id.
func (r *Repository) GetLatestPolicy(ctx context.Context, id string) (domain.AccessPolicy, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, kind, document, version, updated_by, updated_at
		FROM access_policies
		WHERE id = ?
		ORDER BY version DESC
		LIMIT 1
	`, id)

	var p domain.AccessPolicy
	err := row.Scan(&p.ID, &p.Name, &p.Kind, &p.DocumentJSON, &p.Version, &p.UpdatedBy, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AccessPolicy{}, fmt.Errorf("mysql: get latest access policy: %w", usecase.ErrPolicyNotFound)
	}
	if err != nil {
		return domain.AccessPolicy{}, fmt.Errorf("mysql: get latest access policy: %w", err)
	}
	return p, nil
}

// ListLatestPolicies returns one row per policy id — its latest version
// only. Postgres's DISTINCT ON (id) ... ORDER BY id, version DESC has no
// MySQL equivalent — translated to ROW_NUMBER() OVER (PARTITION BY id
// ORDER BY version DESC), a MySQL 8.0+ window function (available on the
// mysql:8 test image and TiDB, which both support the 8.0 window-function
// syntax).
func (r *Repository) ListLatestPolicies(ctx context.Context, pageToken string, pageSize int32) ([]domain.AccessPolicy, string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, kind, document, version, updated_by, updated_at FROM (
			SELECT id, name, kind, document, version, updated_by, updated_at,
			       ROW_NUMBER() OVER (PARTITION BY id ORDER BY version DESC) AS rn
			FROM access_policies
		) latest
		WHERE rn = 1 AND id > ?
		ORDER BY id
		LIMIT ?
	`, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: list access policies: %w", err)
	}
	defer rows.Close()

	var out []domain.AccessPolicy
	for rows.Next() {
		var p domain.AccessPolicy
		if err := rows.Scan(&p.ID, &p.Name, &p.Kind, &p.DocumentJSON, &p.Version, &p.UpdatedBy, &p.UpdatedAt); err != nil {
			return nil, "", fmt.Errorf("mysql: scan access policy row: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate access policy rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// DeletePolicy removes every version row for id — hard-delete, mirrors the
// Postgres variant (see that file's TASK-003 comment).
func (r *Repository) DeletePolicy(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM access_policies WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("mysql: delete access policy: %w", err)
	}
	return nil
}

// CountDistinctIDs returns the number of distinct policy ids, not the
// number of version rows.
func (r *Repository) CountDistinctIDs(ctx context.Context) (int32, error) {
	var n int32
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT id) FROM access_policies`).Scan(&n); err != nil {
		return 0, fmt.Errorf("mysql: count distinct access policies: %w", err)
	}
	return n, nil
}
