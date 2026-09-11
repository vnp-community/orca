package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const folderWorkspaceColumns = `id, tenant_id, dev_server_id, path, name, added_by, created_at, project_group_id`

// FolderWorkspaceRepository implements usecase.FolderWorkspaceRepository
// against `folder_workspaces` — mirrors postgres.FolderWorkspaceRepository.
type FolderWorkspaceRepository struct {
	db *sql.DB
}

func NewFolderWorkspaceRepository(db *sql.DB) *FolderWorkspaceRepository {
	return &FolderWorkspaceRepository{db: db}
}

// Create maps MySQL error codes to the same domain sentinels
// postgres.FolderWorkspaceRepository.Create maps its SQLSTATE codes to:
// 1062 (ER_DUP_ENTRY, the UNIQUE(tenant_id, dev_server_id, path) violation)
// -> ErrPathAlreadyRegistered, 1452 (ER_NO_REFERENCED_ROW_2, an invalid
// project_group_id FK) -> ErrProjectGroupNotFound.
func (r *FolderWorkspaceRepository) Create(ctx context.Context, fw domain.FolderWorkspace) (domain.FolderWorkspace, error) {
	now := time.Now().UTC()
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO folder_workspaces (id, tenant_id, dev_server_id, path, name, added_by, created_at, project_group_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, fw.ID, fw.TenantID, fw.DevServerID, fw.Path, fw.Name, fw.AddedBy, now, nullableString(fw.ProjectGroupID)); err != nil {
		if isMySQLDuplicateEntry(err) {
			return domain.FolderWorkspace{}, domain.ErrPathAlreadyRegistered
		}
		if isMySQLForeignKeyViolation(err) {
			return domain.FolderWorkspace{}, domain.ErrProjectGroupNotFound
		}
		return domain.FolderWorkspace{}, fmt.Errorf("mysql: insert folder workspace: %w", err)
	}
	fw.CreatedAt = now
	return fw, nil
}

// Update checks existence separately rather than trusting RowsAffected —
// see repository.go's UpdateDevServerID doc comment.
func (r *FolderWorkspaceRepository) Update(ctx context.Context, id, name string) (domain.FolderWorkspace, error) {
	existing, err := r.Get(ctx, id)
	if err != nil {
		return domain.FolderWorkspace{}, err
	}
	if existing == nil {
		return domain.FolderWorkspace{}, domain.ErrFolderWorkspaceNotFound
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE folder_workspaces SET name = ? WHERE id = ?`, name, id); err != nil {
		return domain.FolderWorkspace{}, fmt.Errorf("mysql: update folder workspace: %w", err)
	}
	updated, err := r.Get(ctx, id)
	if err != nil {
		return domain.FolderWorkspace{}, err
	}
	if updated == nil {
		// Row existed at the existence check above but is gone by the time
		// this re-reads it — a concurrent Delete raced this Update. Report
		// not-found rather than dereferencing a nil pointer.
		return domain.FolderWorkspace{}, domain.ErrFolderWorkspaceNotFound
	}
	return *updated, nil
}

func (r *FolderWorkspaceRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM folder_workspaces WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("mysql: delete folder workspace: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrFolderWorkspaceNotFound
	}
	return nil
}

func (r *FolderWorkspaceRepository) ListByTenant(ctx context.Context, tenantID string) ([]domain.FolderWorkspace, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+folderWorkspaceColumns+`
		FROM folder_workspaces
		WHERE tenant_id = ?
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: list folder workspaces: %w", err)
	}
	defer rows.Close()

	var out []domain.FolderWorkspace
	for rows.Next() {
		fw, err := scanFolderWorkspace(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan folder workspace row: %w", err)
		}
		out = append(out, fw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate folder workspace rows: %w", err)
	}
	return out, nil
}

func (r *FolderWorkspaceRepository) FindByPath(ctx context.Context, tenantID, devServerID, path string) (*domain.FolderWorkspace, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+folderWorkspaceColumns+`
		FROM folder_workspaces
		WHERE tenant_id = ? AND dev_server_id = ? AND path = ?
	`, tenantID, devServerID, path)

	fw, err := scanFolderWorkspace(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mysql: find folder workspace by path: %w", err)
	}
	return &fw, nil
}

// RepoPathExists cross-checks against `worktrees`, joined through
// `projects` for its dev_server_id — mirrors
// postgres.FolderWorkspaceRepository.RepoPathExists's identical join.
func (r *FolderWorkspaceRepository) RepoPathExists(ctx context.Context, tenantID, devServerID, path string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM worktrees wt
			JOIN projects p ON p.id = wt.project_id
			WHERE p.tenant_id = ? AND p.dev_server_id = ? AND wt.path = ?
		)
	`, tenantID, devServerID, path).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("mysql: check repo path exists: %w", err)
	}
	return exists, nil
}

func (r *FolderWorkspaceRepository) Get(ctx context.Context, id string) (*domain.FolderWorkspace, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+folderWorkspaceColumns+`
		FROM folder_workspaces
		WHERE id = ?
	`, id)

	fw, err := scanFolderWorkspace(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mysql: get folder workspace: %w", err)
	}
	return &fw, nil
}

func scanFolderWorkspace(row rowScanner) (domain.FolderWorkspace, error) {
	var fw domain.FolderWorkspace
	var projectGroupID sql.NullString
	if err := row.Scan(&fw.ID, &fw.TenantID, &fw.DevServerID, &fw.Path, &fw.Name, &fw.AddedBy, &fw.CreatedAt, &projectGroupID); err != nil {
		return domain.FolderWorkspace{}, err
	}
	fw.ProjectGroupID = projectGroupID.String
	return fw, nil
}
