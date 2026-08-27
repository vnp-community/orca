package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

// InsertPolicyVersion appends a new (id, version) row — never an UPDATE,
// per auth-service.md:150's append-only versioning contract (see
// usecase.AccessPolicyRepository's doc comment).
func (r *Repository) InsertPolicyVersion(ctx context.Context, p domain.AccessPolicy) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO auth.access_policies (id, name, kind, document, version, updated_by, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, p.ID, p.Name, p.Kind, p.DocumentJSON, p.Version, p.UpdatedBy, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: insert access policy version: %w", err)
	}
	return nil
}

// GetLatestPolicy returns the highest-version row for id.
func (r *Repository) GetLatestPolicy(ctx context.Context, id string) (domain.AccessPolicy, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, kind, document, version, updated_by, updated_at
		FROM auth.access_policies
		WHERE id = $1
		ORDER BY version DESC
		LIMIT 1
	`, id)

	var p domain.AccessPolicy
	err := row.Scan(&p.ID, &p.Name, &p.Kind, &p.DocumentJSON, &p.Version, &p.UpdatedBy, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AccessPolicy{}, fmt.Errorf("postgres: get latest access policy: %w", usecase.ErrPolicyNotFound)
	}
	if err != nil {
		return domain.AccessPolicy{}, fmt.Errorf("postgres: get latest access policy: %w", err)
	}
	return p, nil
}

// ListLatestPolicies returns one row per policy id — its latest version
// only — using DISTINCT ON (id) ordered by version DESC.
func (r *Repository) ListLatestPolicies(ctx context.Context, pageToken string, pageSize int32) ([]domain.AccessPolicy, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, kind, document, version, updated_by, updated_at
		FROM (
			SELECT DISTINCT ON (id) id, name, kind, document, version, updated_by, updated_at
			FROM auth.access_policies
			ORDER BY id, version DESC
		) latest
		WHERE latest.id::text > $1
		ORDER BY latest.id
		LIMIT $2
	`, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: list access policies: %w", err)
	}
	defer rows.Close()

	var out []domain.AccessPolicy
	for rows.Next() {
		var p domain.AccessPolicy
		if err := rows.Scan(&p.ID, &p.Name, &p.Kind, &p.DocumentJSON, &p.Version, &p.UpdatedBy, &p.UpdatedAt); err != nil {
			return nil, "", fmt.Errorf("postgres: scan access policy row: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate access policy rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// DeletePolicy removes every version row for id — hard-delete. See
// TASK-003's Context note: no audit-retention requirement was found for
// policy documents specifically (distinct from the append-only audit log
// itself, which is never deleted), so this scaffold hard-deletes rather
// than soft-deletes; revisit if a compliance requirement surfaces later.
func (r *Repository) DeletePolicy(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM auth.access_policies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres: delete access policy: %w", err)
	}
	return nil
}

// CountDistinctIDs returns the number of distinct policy ids, not the
// number of version rows.
func (r *Repository) CountDistinctIDs(ctx context.Context) (int32, error) {
	var n int32
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(DISTINCT id) FROM auth.access_policies`).Scan(&n); err != nil {
		return 0, fmt.Errorf("postgres: count distinct access policies: %w", err)
	}
	return n, nil
}
