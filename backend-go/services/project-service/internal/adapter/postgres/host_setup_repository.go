package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const hostSetupColumns = `id, tenant_id, dev_server_id, folder_path, COALESCE(display_name, ''), status, COALESCE(project_id::text, ''), created_by`

type HostSetupRepository struct {
	pool *pgxpool.Pool
}

func NewHostSetupRepository(pool *pgxpool.Pool) *HostSetupRepository {
	return &HostSetupRepository{pool: pool}
}

func (r *HostSetupRepository) Create(ctx context.Context, s domain.HostSetup) (domain.HostSetup, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO project.project_host_setups (id, tenant_id, dev_server_id, folder_path, display_name, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+hostSetupColumns,
		s.ID, s.TenantID, s.DevServerID, s.FolderPath, nullableString(s.DisplayName), string(s.Status), s.CreatedBy,
	)
	out, err := scanHostSetup(row)
	if err != nil {
		return domain.HostSetup{}, fmt.Errorf("postgres: insert host setup: %w", err)
	}
	return out, nil
}

func (r *HostSetupRepository) Get(ctx context.Context, tenantID, id string) (domain.HostSetup, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+hostSetupColumns+`
		FROM project.project_host_setups
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)
	out, err := scanHostSetup(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.HostSetup{}, domain.ErrHostSetupNotFound
	}
	if err != nil {
		return domain.HostSetup{}, fmt.Errorf("postgres: query host setup: %w", err)
	}
	return out, nil
}

func (r *HostSetupRepository) List(ctx context.Context, tenantID string) ([]domain.HostSetup, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+hostSetupColumns+`
		FROM project.project_host_setups
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query host setups: %w", err)
	}
	defer rows.Close()

	var out []domain.HostSetup
	for rows.Next() {
		s, err := scanHostSetup(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan host setup row: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *HostSetupRepository) Update(ctx context.Context, tenantID, id string, patch domain.HostSetupPatch) (domain.HostSetup, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE project.project_host_setups
		SET folder_path  = COALESCE(NULLIF($3, ''), folder_path),
		    display_name = COALESCE(NULLIF($4, ''), display_name),
		    updated_at   = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING `+hostSetupColumns,
		tenantID, id, patch.FolderPath, patch.DisplayName,
	)
	out, err := scanHostSetup(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.HostSetup{}, domain.ErrHostSetupNotFound
	}
	if err != nil {
		return domain.HostSetup{}, fmt.Errorf("postgres: update host setup: %w", err)
	}
	return out, nil
}

func (r *HostSetupRepository) Delete(ctx context.Context, tenantID, id string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM project.project_host_setups WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)
	if err != nil {
		return fmt.Errorf("postgres: delete host setup: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrHostSetupNotFound
	}
	return nil
}

func (r *HostSetupRepository) SetStatus(ctx context.Context, tenantID, id string, status domain.HostSetupStatus) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE project.project_host_setups SET status = $3, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id, string(status))
	if err != nil {
		return fmt.Errorf("postgres: set host setup status: %w", err)
	}
	return nil
}

func (r *HostSetupRepository) Complete(ctx context.Context, tenantID, id, projectID string) (domain.HostSetup, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE project.project_host_setups
		SET status = $3, project_id = $4, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING `+hostSetupColumns,
		tenantID, id, string(domain.HostSetupCompleted), projectID,
	)
	out, err := scanHostSetup(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.HostSetup{}, domain.ErrHostSetupNotFound
	}
	if err != nil {
		return domain.HostSetup{}, fmt.Errorf("postgres: complete host setup: %w", err)
	}
	return out, nil
}

func scanHostSetup(row rowScanner) (domain.HostSetup, error) {
	var s domain.HostSetup
	var status string
	if err := row.Scan(&s.ID, &s.TenantID, &s.DevServerID, &s.FolderPath, &s.DisplayName, &status, &s.ProjectID, &s.CreatedBy); err != nil {
		return domain.HostSetup{}, err
	}
	s.Status = domain.HostSetupStatus(status)
	return s, nil
}
