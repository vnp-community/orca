package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const hostSetupColumns = `id, tenant_id, dev_server_id, folder_path, display_name, status, project_id, created_by`

// HostSetupRepository implements usecase.HostSetupRepository against
// `project_host_setups` — mirrors postgres.HostSetupRepository.
type HostSetupRepository struct {
	db *sql.DB
}

func NewHostSetupRepository(db *sql.DB) *HostSetupRepository {
	return &HostSetupRepository{db: db}
}

func (r *HostSetupRepository) Create(ctx context.Context, s domain.HostSetup) (domain.HostSetup, error) {
	now := time.Now().UTC()
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO project_host_setups (id, tenant_id, dev_server_id, folder_path, display_name, status, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.TenantID, s.DevServerID, s.FolderPath, nullableString(s.DisplayName), string(s.Status), s.CreatedBy, now, now); err != nil {
		return domain.HostSetup{}, fmt.Errorf("mysql: insert host setup: %w", err)
	}
	return s, nil
}

func (r *HostSetupRepository) Get(ctx context.Context, tenantID, id string) (domain.HostSetup, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+hostSetupColumns+`
		FROM project_host_setups
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id)
	out, err := scanHostSetup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.HostSetup{}, domain.ErrHostSetupNotFound
	}
	if err != nil {
		return domain.HostSetup{}, fmt.Errorf("mysql: query host setup: %w", err)
	}
	return out, nil
}

func (r *HostSetupRepository) List(ctx context.Context, tenantID string) ([]domain.HostSetup, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+hostSetupColumns+`
		FROM project_host_setups
		WHERE tenant_id = ?
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query host setups: %w", err)
	}
	defer rows.Close()

	var out []domain.HostSetup
	for rows.Next() {
		s, err := scanHostSetup(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan host setup row: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Update checks existence separately rather than trusting RowsAffected —
// see repository.go's UpdateDevServerID doc comment.
func (r *HostSetupRepository) Update(ctx context.Context, tenantID, id string, patch domain.HostSetupPatch) (domain.HostSetup, error) {
	if _, err := r.Get(ctx, tenantID, id); err != nil {
		return domain.HostSetup{}, err
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE project_host_setups
		SET folder_path  = COALESCE(NULLIF(?, ''), folder_path),
		    display_name = COALESCE(NULLIF(?, ''), display_name),
		    updated_at   = ?
		WHERE tenant_id = ? AND id = ?
	`, patch.FolderPath, patch.DisplayName, time.Now().UTC(), tenantID, id); err != nil {
		return domain.HostSetup{}, fmt.Errorf("mysql: update host setup: %w", err)
	}
	return r.Get(ctx, tenantID, id)
}

func (r *HostSetupRepository) Delete(ctx context.Context, tenantID, id string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM project_host_setups WHERE tenant_id = ? AND id = ?
	`, tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: delete host setup: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrHostSetupNotFound
	}
	return nil
}

func (r *HostSetupRepository) SetStatus(ctx context.Context, tenantID, id string, status domain.HostSetupStatus) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE project_host_setups SET status = ?, updated_at = ?
		WHERE tenant_id = ? AND id = ?
	`, string(status), time.Now().UTC(), tenantID, id); err != nil {
		return fmt.Errorf("mysql: set host setup status: %w", err)
	}
	return nil
}

// Complete checks existence separately rather than trusting RowsAffected —
// see repository.go's UpdateDevServerID doc comment.
func (r *HostSetupRepository) Complete(ctx context.Context, tenantID, id, projectID string) (domain.HostSetup, error) {
	if _, err := r.Get(ctx, tenantID, id); err != nil {
		return domain.HostSetup{}, err
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE project_host_setups
		SET status = ?, project_id = ?, updated_at = ?
		WHERE tenant_id = ? AND id = ?
	`, string(domain.HostSetupCompleted), projectID, time.Now().UTC(), tenantID, id); err != nil {
		return domain.HostSetup{}, fmt.Errorf("mysql: complete host setup: %w", err)
	}
	return r.Get(ctx, tenantID, id)
}

func scanHostSetup(row rowScanner) (domain.HostSetup, error) {
	var s domain.HostSetup
	var status string
	var displayName, projectID sql.NullString
	if err := row.Scan(&s.ID, &s.TenantID, &s.DevServerID, &s.FolderPath, &displayName, &status, &projectID, &s.CreatedBy); err != nil {
		return domain.HostSetup{}, err
	}
	s.DisplayName = displayName.String
	s.ProjectID = projectID.String
	s.Status = domain.HostSetupStatus(status)
	return s, nil
}
