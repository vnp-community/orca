// Package postgres implements annotation-service's Repository port (defined
// in internal/usecase) against this service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in annotation-service
// that knows SQL exists.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
)

// Repository implements usecase.Repository against Postgres via pgx —
// hand-written SQL (see architecture/04-tech-stack.md: sqlc codegen is the
// eventual target, this scaffold hand-writes the equivalent queries
// directly to avoid a build-time dependency on the sqlc binary).
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateAnnotation(ctx context.Context, a domain.Annotation) (domain.Annotation, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO annotation.annotations (
			id, tenant_id, author_id, repo_id, file_path, line, ref,
			content, resolved, request_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`,
		a.ID, a.TenantID, a.AuthorID, a.Anchor.RepoID, a.Anchor.FilePath, a.Anchor.Line, a.Anchor.Ref,
		a.Content, a.Resolved, a.RequestID, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return domain.Annotation{}, fmt.Errorf("postgres: insert annotation: %w", err)
	}
	return a, nil
}

// FindByRequestID looks up an existing annotation for (tenantID, requestID)
// — CreateAnnotation's idempotency check, mirroring automation-service's
// AutomationRunRepository.FindByRequestID.
func (r *Repository) FindByRequestID(ctx context.Context, tenantID, requestID string) (domain.Annotation, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, author_id, repo_id, file_path, line, ref,
		       content, resolved, request_id, created_at, updated_at
		FROM annotation.annotations
		WHERE tenant_id = $1 AND request_id = $2
	`, tenantID, requestID)

	a, err := scanAnnotation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Annotation{}, false, nil
	}
	if err != nil {
		return domain.Annotation{}, false, fmt.Errorf("postgres: find annotation by request id: %w", err)
	}
	return a, true, nil
}

func (r *Repository) ListAnnotations(ctx context.Context, tenantID, repoID, filePath, pageToken string, pageSize int32) ([]domain.Annotation, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, author_id, repo_id, file_path, line, ref,
		       content, resolved, request_id, created_at, updated_at
		FROM annotation.annotations
		WHERE tenant_id = $1 AND repo_id = $2 AND ($3 = '' OR file_path = $3) AND id::text > $4
		ORDER BY id
		LIMIT $5
	`, tenantID, repoID, filePath, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query annotations: %w", err)
	}
	defer rows.Close()

	var out []domain.Annotation
	for rows.Next() {
		a, err := scanAnnotation(rows)
		if err != nil {
			return nil, "", fmt.Errorf("postgres: scan annotation row: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate annotation rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// GetAnnotation fetches a single annotation by id, scoped to tenantID.
// UpdateAnnotation/DeleteAnnotation's usecases call this first to read
// author_id for the OPA author-only check before mutating.
func (r *Repository) GetAnnotation(ctx context.Context, tenantID, id string) (domain.Annotation, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, author_id, repo_id, file_path, line, ref,
		       content, resolved, request_id, created_at, updated_at
		FROM annotation.annotations
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	a, err := scanAnnotation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Annotation{}, domain.ErrAnnotationNotFound
	}
	if err != nil {
		return domain.Annotation{}, fmt.Errorf("postgres: get annotation: %w", err)
	}
	return a, nil
}

func (r *Repository) UpdateAnnotation(ctx context.Context, tenantID, id, content string, resolved bool) (domain.Annotation, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE annotation.annotations
		SET content = $1, resolved = $2, updated_at = now()
		WHERE tenant_id = $3 AND id = $4
		RETURNING id, tenant_id, author_id, repo_id, file_path, line, ref,
		          content, resolved, request_id, created_at, updated_at
	`, content, resolved, tenantID, id)

	a, err := scanAnnotation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Annotation{}, domain.ErrAnnotationNotFound
	}
	if err != nil {
		return domain.Annotation{}, fmt.Errorf("postgres: update annotation: %w", err)
	}
	return a, nil
}

func (r *Repository) DeleteAnnotation(ctx context.Context, tenantID, id string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM annotation.annotations WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)
	if err != nil {
		return fmt.Errorf("postgres: delete annotation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAnnotationNotFound
	}
	return nil
}

// rowScanner is satisfied by both pgx.Rows and pgx.Row, letting
// scanAnnotation serve both ListAnnotations and UpdateAnnotation.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanAnnotation(row rowScanner) (domain.Annotation, error) {
	var a domain.Annotation
	if err := row.Scan(
		&a.ID, &a.TenantID, &a.AuthorID, &a.Anchor.RepoID, &a.Anchor.FilePath, &a.Anchor.Line, &a.Anchor.Ref,
		&a.Content, &a.Resolved, &a.RequestID, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return domain.Annotation{}, err
	}
	return a, nil
}
