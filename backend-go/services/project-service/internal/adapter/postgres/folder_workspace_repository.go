package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// pgUniqueViolationCode is Postgres's SQLSTATE for a unique_violation —
// used below to map folder_workspaces' UNIQUE(tenant_id, dev_server_id,
// path) constraint to domain.ErrPathAlreadyRegistered rather than a raw
// pgconn error.
const pgUniqueViolationCode = "23505"

// pgForeignKeyViolationCode is Postgres's SQLSTATE for a
// foreign_key_violation — used below to map an invalid project_group_id to
// domain.ErrProjectGroupNotFound (no app-level pre-check of group
// existence; the FK constraint itself is the enforcement).
const pgForeignKeyViolationCode = "23503"

// COALESCE(project_group_id::text, ”) mirrors project_group_repository.go's
// own parent_group_id/project_id convention — nullableString below is the
// INSERT-direction inverse.
const folderWorkspaceColumns = `id, tenant_id, dev_server_id, path, name, added_by, created_at, COALESCE(project_group_id::text, '')`

// FolderWorkspaceRepository implements usecase.FolderWorkspaceRepository
// against project.folder_workspaces. Kept as its own struct (not folded
// into Repository) — one struct per entity/port, matching RepoRepository/
// WorktreeRepository/ProjectGroupRepository's layout in this package.
type FolderWorkspaceRepository struct {
	pool *pgxpool.Pool
}

func NewFolderWorkspaceRepository(pool *pgxpool.Pool) *FolderWorkspaceRepository {
	return &FolderWorkspaceRepository{pool: pool}
}

func (r *FolderWorkspaceRepository) Create(ctx context.Context, fw domain.FolderWorkspace) (domain.FolderWorkspace, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO project.folder_workspaces (id, tenant_id, dev_server_id, path, name, added_by, project_group_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+folderWorkspaceColumns,
		fw.ID, fw.TenantID, fw.DevServerID, fw.Path, fw.Name, fw.AddedBy, nullableString(fw.ProjectGroupID),
	)

	out, err := scanFolderWorkspace(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case pgUniqueViolationCode:
				return domain.FolderWorkspace{}, domain.ErrPathAlreadyRegistered
			case pgForeignKeyViolationCode:
				return domain.FolderWorkspace{}, domain.ErrProjectGroupNotFound
			}
		}
		return domain.FolderWorkspace{}, fmt.Errorf("postgres: insert folder workspace: %w", err)
	}
	return out, nil
}

func (r *FolderWorkspaceRepository) Update(ctx context.Context, id, name string) (domain.FolderWorkspace, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE project.folder_workspaces SET name = $2
		WHERE id = $1
		RETURNING `+folderWorkspaceColumns,
		id, name,
	)

	out, err := scanFolderWorkspace(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.FolderWorkspace{}, domain.ErrFolderWorkspaceNotFound
	}
	if err != nil {
		return domain.FolderWorkspace{}, fmt.Errorf("postgres: update folder workspace: %w", err)
	}
	return out, nil
}

func (r *FolderWorkspaceRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM project.folder_workspaces WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres: delete folder workspace: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFolderWorkspaceNotFound
	}
	return nil
}

func (r *FolderWorkspaceRepository) ListByTenant(ctx context.Context, tenantID string) ([]domain.FolderWorkspace, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+folderWorkspaceColumns+`
		FROM project.folder_workspaces
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list folder workspaces: %w", err)
	}
	defer rows.Close()

	var out []domain.FolderWorkspace
	for rows.Next() {
		fw, err := scanFolderWorkspace(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan folder workspace row: %w", err)
		}
		out = append(out, fw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate folder workspace rows: %w", err)
	}
	return out, nil
}

func (r *FolderWorkspaceRepository) FindByPath(ctx context.Context, tenantID, devServerID, path string) (*domain.FolderWorkspace, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+folderWorkspaceColumns+`
		FROM project.folder_workspaces
		WHERE tenant_id = $1 AND dev_server_id = $2 AND path = $3
	`, tenantID, devServerID, path)

	fw, err := scanFolderWorkspace(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: find folder workspace by path: %w", err)
	}
	return &fw, nil
}

// RepoPathExists cross-checks against project.worktrees, joined through
// project.projects for its dev_server_id. project.repos itself has no
// filesystem-path column (its `url` is the git remote, per
// migrations/0003_repos.up.sql's comment) — project.worktrees.path is this
// service's only column holding a real on-disk path for a git-backed
// checkout, so that (not repos) is what a folder-workspace path can
// actually collide with.
func (r *FolderWorkspaceRepository) RepoPathExists(ctx context.Context, tenantID, devServerID, path string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM project.worktrees wt
			JOIN project.projects p ON p.id = wt.project_id
			WHERE p.tenant_id = $1 AND p.dev_server_id = $2 AND wt.path = $3
		)
	`, tenantID, devServerID, path).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres: check repo path exists: %w", err)
	}
	return exists, nil
}

func (r *FolderWorkspaceRepository) Get(ctx context.Context, id string) (*domain.FolderWorkspace, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+folderWorkspaceColumns+`
		FROM project.folder_workspaces
		WHERE id = $1
	`, id)

	fw, err := scanFolderWorkspace(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get folder workspace: %w", err)
	}
	return &fw, nil
}

// scanFolderWorkspace does NOT convert pgx.ErrNoRows itself — see
// scanProjectGroup's identical doc comment (every caller here already
// checks errors.Is(err, pgx.ErrNoRows) against the raw scan error).
func scanFolderWorkspace(row rowScanner) (domain.FolderWorkspace, error) {
	var fw domain.FolderWorkspace
	if err := row.Scan(&fw.ID, &fw.TenantID, &fw.DevServerID, &fw.Path, &fw.Name, &fw.AddedBy, &fw.CreatedAt, &fw.ProjectGroupID); err != nil {
		return domain.FolderWorkspace{}, err
	}
	return fw, nil
}
