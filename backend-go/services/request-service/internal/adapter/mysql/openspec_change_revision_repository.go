package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

var _ usecase.OpenSpecChangeRevisionRepository = (*OpenSpecChangeRevisionRepository)(nil)

// maxRevisionPage bounds a listing whatever the caller asks for: a request has as many versions as it had Plan runs.
const maxRevisionPage = 500

type OpenSpecChangeRevisionRepository struct {
	*Repository
}

func NewOpenSpecChangeRevisionRepository(r *Repository) *OpenSpecChangeRevisionRepository {
	return &OpenSpecChangeRevisionRepository{Repository: r}
}

// revisionIndexColumns is a version without its text; revisionFullColumns adds the text.
const revisionIndexColumns = `id, request_id, revision, run_id, commit_sha, tasks_md_digest, bundle_digest, files_json,
	(bundle_json IS NOT NULL), created_at`
const revisionFullColumns = revisionIndexColumns + `, bundle_json`

func scanPlanRevision(row rowScanner, withText bool) (domain.OpenSpecChangeRevision, error) {
	var rev domain.OpenSpecChangeRevision
	var files, bundle []byte
	dest := []any{&rev.ID, &rev.RequestID, &rev.Revision, &rev.RunID, &rev.Commit, &rev.TasksMDDigest, &rev.BundleDigest, &files, &rev.TextKept, &rev.CreatedAt}
	if withText {
		dest = append(dest, &bundle)
	}
	if err := row.Scan(dest...); err != nil {
		return domain.OpenSpecChangeRevision{}, err
	}
	rev.CreatedAt = rev.CreatedAt.UTC()
	if err := json.Unmarshal(files, &rev.Files); err != nil {
		return domain.OpenSpecChangeRevision{}, fmt.Errorf("mysql: openspec change revision files: %w", err)
	}
	if withText && len(bundle) > 0 {
		if err := json.Unmarshal(bundle, &rev.Bundle); err != nil {
			return domain.OpenSpecChangeRevision{}, fmt.Errorf("mysql: openspec change revision bundle: %w", err)
		}
	}
	return rev, nil
}

// Append joins the transaction of ctx when there is one, so a version is stored or lost together with the change it
// describes.
func (r *OpenSpecChangeRevisionRepository) Append(ctx context.Context, rev domain.OpenSpecChangeRevision, keepText int) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		files, err := json.Marshal(nonNilFiles(rev.Files))
		if err != nil {
			return err
		}
		var bundle any
		if rev.Bundle != nil {
			b, err := json.Marshal(rev.Bundle)
			if err != nil {
				return err
			}
			bundle = string(b)
		}
		var created any
		if !rev.CreatedAt.IsZero() {
			created = rev.CreatedAt.UTC()
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO openspec_change_revisions (id, tenant_id, request_id, revision, run_id, commit_sha, tasks_md_digest, bundle_digest,
				files_json, bundle_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP(6)))`,
			rev.ID, tenantID, rev.RequestID, rev.Revision, rev.RunID, rev.Commit, rev.TasksMDDigest, rev.BundleDigest, string(files), bundle,
			created); err != nil {
			return fmt.Errorf("mysql: append openspec change revision: %w", err)
		}
		if keepText <= 0 {
			return nil
		}
		if _, err := db.ExecContext(ctx, `
			UPDATE openspec_change_revisions SET bundle_json = NULL
			WHERE tenant_id = ? AND request_id = ? AND revision <= ? AND bundle_json IS NOT NULL`,
			tenantID, rev.RequestID, rev.Revision-keepText); err != nil {
			return fmt.Errorf("mysql: prune openspec change revision text: %w", err)
		}
		return nil
	})
}

func nonNilFiles(in []domain.RevisionFile) []domain.RevisionFile {
	if in == nil {
		return []domain.RevisionFile{}
	}
	return in
}

func (r *OpenSpecChangeRevisionRepository) List(ctx context.Context, requestID string, beforeRevision, limit int) ([]domain.OpenSpecChangeRevision, error) {
	var out []domain.OpenSpecChangeRevision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if limit <= 0 || limit > maxRevisionPage {
			limit = maxRevisionPage
		}
		q := strings.Builder{}
		q.WriteString(`SELECT ` + revisionIndexColumns + ` FROM openspec_change_revisions WHERE tenant_id = ? AND request_id = ?`)
		args := []any{tenantID, requestID}
		if beforeRevision > 0 {
			q.WriteString(` AND revision < ?`)
			args = append(args, beforeRevision)
		}
		q.WriteString(` ORDER BY revision DESC LIMIT ?`)
		args = append(args, limit)
		rows, err := db.QueryContext(ctx, q.String(), args...)
		if err != nil {
			return fmt.Errorf("mysql: list openspec change revisions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			rev, err := scanPlanRevision(rows, false)
			if err != nil {
				return err
			}
			out = append(out, rev)
		}
		return rows.Err()
	})
	return out, err
}

func (r *OpenSpecChangeRevisionRepository) Get(ctx context.Context, requestID string, revision int) (domain.OpenSpecChangeRevision, bool, error) {
	return r.getOne(ctx, `request_id = ? AND revision = ?`, requestID, revision)
}

func (r *OpenSpecChangeRevisionRepository) GetByRun(ctx context.Context, runID string) (domain.OpenSpecChangeRevision, bool, error) {
	return r.getOne(ctx, `run_id = ?`, runID)
}

// getOne reads one version with its text. The tenant comes from the scoped context: MySQL has no RLS, so without it
// another tenant's version would be readable.
func (r *OpenSpecChangeRevisionRepository) getOne(ctx context.Context, predicate string, args ...any) (domain.OpenSpecChangeRevision, bool, error) {
	var rev domain.OpenSpecChangeRevision
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		var err error
		rev, err = scanPlanRevision(db.QueryRowContext(ctx, `SELECT `+revisionFullColumns+` FROM openspec_change_revisions WHERE tenant_id = ? AND `+predicate,
			append([]any{tenantID}, args...)...), true)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("mysql: get openspec change revision: %w", err)
		}
		found = true
		return nil
	})
	return rev, found, err
}
