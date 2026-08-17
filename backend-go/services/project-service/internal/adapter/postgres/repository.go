// Package postgres implements project-service's ProjectRepository port
// (defined in internal/usecase) against this service's own PostgreSQL
// database — see specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in project-service
// that knows SQL exists.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// Repository implements usecase.ProjectRepository against Postgres via pgx
// — hand-written SQL (see architecture/04-tech-stack.md: sqlc codegen is the
// eventual target, this scaffold hand-writes the equivalent queries
// directly to avoid a build-time dependency on the sqlc binary, matching
// usage-service's reference scaffold).
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, p domain.Project) (domain.Project, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO project.projects (id, tenant_id, name, dev_server_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, tenant_id, name, COALESCE(dev_server_id, '')
	`, p.ID, p.TenantID, p.Name, nullableString(p.DevServerID))

	var out domain.Project
	if err := row.Scan(&out.ID, &out.TenantID, &out.Name, &out.DevServerID); err != nil {
		return domain.Project{}, fmt.Errorf("postgres: insert project: %w", err)
	}
	return out, nil
}

func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.Project, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, COALESCE(dev_server_id, '')
		FROM project.projects
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var out domain.Project
	err := row.Scan(&out.ID, &out.TenantID, &out.Name, &out.DevServerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Project{}, domain.ErrProjectNotFound
	}
	if err != nil {
		return domain.Project{}, fmt.Errorf("postgres: query project: %w", err)
	}
	return out, nil
}

func (r *Repository) List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Project, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, name, COALESCE(dev_server_id, '')
		FROM project.projects
		WHERE tenant_id = $1 AND id > $2
		ORDER BY id
		LIMIT $3
	`, tenantID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query projects: %w", err)
	}
	defer rows.Close()

	var out []domain.Project
	for rows.Next() {
		var p domain.Project
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.DevServerID); err != nil {
			return nil, "", fmt.Errorf("postgres: scan project row: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate project rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func (r *Repository) AddMember(ctx context.Context, m domain.ProjectMember) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO project.project_members (project_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`, m.ProjectID, m.UserID, string(m.Role))
	if err != nil {
		return fmt.Errorf("postgres: insert project member: %w", err)
	}
	return nil
}

// UpdateDevServerID is the ONLY write path for dev_server_id — called after
// usecase.RebindDevServer's active-execution guard has already passed. See
// project-service.md §3: "UpdateProject's field mask rejects dev_server_id
// so there is exactly one code path for rebinding, not two that can drift."
func (r *Repository) UpdateDevServerID(ctx context.Context, tenantID, projectID, devServerID string) (domain.Project, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE project.projects
		SET dev_server_id = $3
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, name, COALESCE(dev_server_id, '')
	`, tenantID, projectID, nullableString(devServerID))

	var out domain.Project
	err := row.Scan(&out.ID, &out.TenantID, &out.Name, &out.DevServerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Project{}, domain.ErrProjectNotFound
	}
	if err != nil {
		return domain.Project{}, fmt.Errorf("postgres: update dev_server_id: %w", err)
	}
	return out, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
