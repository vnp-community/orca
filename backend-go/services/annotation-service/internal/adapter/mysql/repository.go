// Package mysql implements annotation-service's Repository port (defined
// in internal/usecase) against MySQL/TiDB via database/sql +
// github.com/go-sql-driver/mysql — mirroring internal/adapter/postgres's
// behavior (idempotent create, author lookup, tenant scoping, bulk
// mark-sent) 1:1 against the dialect-safe schema in
// migrations/mysql/{0001_init,0002_annotation_request_id,
// 0003_annotation_side_range_sent}.up.sql (table `annotations`, no
// schema/database prefix). See
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-005.md
// (this service's adapter), following the pattern set by
// BE-DB-SOL-002 (usage-service, the pilot).
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
)

// Repository implements usecase.Repository against MySQL/TiDB via
// database/sql. No RLS equivalent exists in MySQL — every query below
// filters by tenant_id explicitly, which is the ONLY tenant-isolation
// enforcement for this adapter (see TASK-BE-DB-003's finding, true here
// for the same reason it was true for usage-service: no Go code anywhere
// calls SET LOCAL app.tenant_id, so the Postgres adapter's RLS policy
// never actually activated either — this doesn't lower the bar, it just
// doesn't add a backstop that was never real).
type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateAnnotation mirrors internal/adapter/postgres.Repository.CreateAnnotation
// exactly: a plain INSERT with no ON CONFLICT/INSERT IGNORE. Idempotency on
// (tenant_id, request_id) is a usecase-layer concern
// (usecase.CreateAnnotation.Execute calls FindByRequestID first, and
// retries it once more if this INSERT fails on the UNIQUE constraint) —
// this adapter doesn't special-case a duplicate-key error, matching the
// Postgres adapter's own behavior 1:1.
func (r *Repository) CreateAnnotation(ctx context.Context, a domain.Annotation) (domain.Annotation, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO annotations (
			id, tenant_id, author_id, repo_id, worktree_id, file_path, line, end_line, side, ref,
			content, original_code, resolved, sent_to_agent, sent_at, request_id, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		a.ID, a.TenantID, a.AuthorID, a.Anchor.RepoID, nullString(a.Anchor.WorktreeID), a.Anchor.FilePath, a.Anchor.Line, a.Anchor.EndLine, int32(a.Anchor.Side), a.Anchor.Ref,
		a.Content, a.OriginalCode, a.Resolved, a.SentToAgent, a.SentAt, a.RequestID, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return domain.Annotation{}, fmt.Errorf("mysql: insert annotation: %w", err)
	}
	return a, nil
}

// nullString converts "" to a NULL-capable bind for the nullable
// worktree_id column — an empty WorktreeID means "not worktree-scoped",
// which must round-trip as SQL NULL, not the string "" (mirrors
// internal/adapter/postgres's nullString exactly).
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// FindByRequestID looks up an existing annotation for (tenantID,
// requestID) — CreateAnnotation's idempotency check, mirroring
// internal/adapter/postgres.Repository.FindByRequestID.
func (r *Repository) FindByRequestID(ctx context.Context, tenantID, requestID string) (domain.Annotation, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, author_id, repo_id, worktree_id, file_path, line, end_line, side, ref,
		       content, original_code, resolved, sent_to_agent, sent_at, request_id, created_at, updated_at
		FROM annotations
		WHERE tenant_id = ? AND request_id = ?
	`, tenantID, requestID)

	a, err := scanAnnotation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Annotation{}, false, nil
	}
	if err != nil {
		return domain.Annotation{}, false, fmt.Errorf("mysql: find annotation by request id: %w", err)
	}
	return a, true, nil
}

func (r *Repository) ListAnnotations(ctx context.Context, tenantID, repoID, filePath, pageToken string, pageSize int32) ([]domain.Annotation, string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, author_id, repo_id, worktree_id, file_path, line, end_line, side, ref,
		       content, original_code, resolved, sent_to_agent, sent_at, request_id, created_at, updated_at
		FROM annotations
		WHERE tenant_id = ? AND repo_id = ? AND (? = '' OR file_path = ?) AND id > ?
		ORDER BY id
		LIMIT ?
	`, tenantID, repoID, filePath, filePath, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query annotations: %w", err)
	}
	defer rows.Close()

	var out []domain.Annotation
	for rows.Next() {
		a, err := scanAnnotation(rows)
		if err != nil {
			return nil, "", fmt.Errorf("mysql: scan annotation row: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate annotation rows: %w", err)
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
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, author_id, repo_id, worktree_id, file_path, line, end_line, side, ref,
		       content, original_code, resolved, sent_to_agent, sent_at, request_id, created_at, updated_at
		FROM annotations
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id)

	a, err := scanAnnotation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Annotation{}, domain.ErrAnnotationNotFound
	}
	if err != nil {
		return domain.Annotation{}, fmt.Errorf("mysql: get annotation: %w", err)
	}
	return a, nil
}

// UpdateAnnotation has no RETURNING equivalent in MySQL, so it runs the
// UPDATE then re-reads the row via GetAnnotation — deliberately NOT
// branching on UPDATE's RowsAffected()==0 to detect "not found": MySQL's
// default RowsAffected() semantics count only rows whose VALUES actually
// changed, not rows matched by WHERE (Postgres/pgx's RowsAffected counts
// every row the UPDATE touched, changed or not). A caller re-submitting
// the same content/resolved value it already set — an entirely legitimate
// idempotent retry — would hit RowsAffected()==0 under MySQL's semantics
// even though the row exists and the tenant/id matched, which would
// wrongly surface as domain.ErrAnnotationNotFound. Re-reading by
// (tenant_id, id) instead sidesteps this dialect difference entirely: a
// missing row is indistinguishable from a no-op update in MySQL's
// RowsAffected count, but GetAnnotation's own NOT FOUND path already
// handles "missing" correctly regardless of what the UPDATE changed.
func (r *Repository) UpdateAnnotation(ctx context.Context, tenantID, id, content string, resolved bool) (domain.Annotation, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE annotations
		SET content = ?, resolved = ?, updated_at = NOW(6)
		WHERE tenant_id = ? AND id = ?
	`, content, resolved, tenantID, id)
	if err != nil {
		return domain.Annotation{}, fmt.Errorf("mysql: update annotation: %w", err)
	}
	return r.GetAnnotation(ctx, tenantID, id)
}

func (r *Repository) DeleteAnnotation(ctx context.Context, tenantID, id string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM annotations WHERE tenant_id = ? AND id = ?
	`, tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: delete annotation: %w", err)
	}
	// Unlike UPDATE, MySQL's DELETE RowsAffected() always counts rows
	// matched by WHERE (there's no "changed vs matched" ambiguity for a
	// DELETE — a row is either removed or it isn't), so this check is
	// dialect-safe as-is, no re-read needed.
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: delete annotation rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrAnnotationNotFound
	}
	return nil
}

// MarkSent transitions every annotation in ids (scoped to tenantID) to
// SentToAgent=true/SentAt=sentAt in one statement, then re-reads exactly
// that (tenantID, ids) set to return the updated rows — MySQL has no
// RETURNING, and (per UpdateAnnotation's comment) re-reading also avoids
// MySQL's matched-vs-changed RowsAffected ambiguity for the ids in the
// list that were already sent_to_agent=true. Any id not found for the
// tenant is silently absent from the result, not a hard failure — SOL-CR-03
// calls this after PTY injection already succeeded, so a partial id
// mismatch (e.g. a concurrently-deleted annotation) must not turn a
// successful send into an error response, mirroring the Postgres adapter.
func (r *Repository) MarkSent(ctx context.Context, tenantID string, ids []string, sentAt time.Time) ([]domain.Annotation, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	idArgs := make([]any, 0, len(ids))
	for _, id := range ids {
		idArgs = append(idArgs, id)
	}

	updateArgs := append([]any{sentAt, tenantID}, idArgs...)
	updateQuery := fmt.Sprintf(`
		UPDATE annotations SET sent_to_agent = true, sent_at = ?
		WHERE tenant_id = ? AND id IN (%s)
	`, placeholders)
	if _, err := r.db.ExecContext(ctx, updateQuery, updateArgs...); err != nil {
		return nil, fmt.Errorf("mysql: mark annotations sent: %w", err)
	}

	selectArgs := append([]any{tenantID}, idArgs...)
	selectQuery := fmt.Sprintf(`
		SELECT id, tenant_id, author_id, repo_id, worktree_id, file_path, line, end_line, side, ref,
		       content, original_code, resolved, sent_to_agent, sent_at, request_id, created_at, updated_at
		FROM annotations
		WHERE tenant_id = ? AND id IN (%s)
	`, placeholders)
	rows, err := r.db.QueryContext(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, fmt.Errorf("mysql: query marked annotations: %w", err)
	}
	defer rows.Close()

	var out []domain.Annotation
	for rows.Next() {
		a, err := scanAnnotation(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan marked annotation: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate marked annotation rows: %w", err)
	}
	return out, nil
}

// rowScanner is satisfied by both *sql.Rows and *sql.Row, letting
// scanAnnotation serve every read path above — mirrors
// internal/adapter/postgres's rowScanner.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanAnnotation(row rowScanner) (domain.Annotation, error) {
	var a domain.Annotation
	var worktreeID sql.NullString
	var side int32
	var sentAt sql.NullTime
	if err := row.Scan(
		&a.ID, &a.TenantID, &a.AuthorID, &a.Anchor.RepoID, &worktreeID, &a.Anchor.FilePath, &a.Anchor.Line, &a.Anchor.EndLine, &side, &a.Anchor.Ref,
		&a.Content, &a.OriginalCode, &a.Resolved, &a.SentToAgent, &sentAt, &a.RequestID, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return domain.Annotation{}, err
	}
	if worktreeID.Valid {
		a.Anchor.WorktreeID = worktreeID.String
	}
	a.Anchor.Side = domain.Side(side)
	if sentAt.Valid {
		t := sentAt.Time
		a.SentAt = &t
	}
	return a, nil
}
