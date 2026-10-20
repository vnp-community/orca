package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
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
const revisionIndexColumns = `id::text, request_id::text, revision, run_id::text, commit_sha, tasks_md_digest, bundle_digest, files_json,
	(bundle_json IS NOT NULL), created_at`
const revisionFullColumns = revisionIndexColumns + `, bundle_json`

func scanPlanRevision(row pgx.Row, withText bool) (domain.OpenSpecChangeRevision, error) {
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
		return domain.OpenSpecChangeRevision{}, fmt.Errorf("postgres: openspec change revision files: %w", err)
	}
	if withText && len(bundle) > 0 {
		if err := json.Unmarshal(bundle, &rev.Bundle); err != nil {
			return domain.OpenSpecChangeRevision{}, fmt.Errorf("postgres: openspec change revision bundle: %w", err)
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
			created = rev.CreatedAt
		}
		if _, err := db.Exec(ctx, `
			INSERT INTO request.openspec_change_revisions (id, tenant_id, request_id, revision, run_id, commit_sha, tasks_md_digest, bundle_digest,
				files_json, bundle_json, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, COALESCE($11, now()))`,
			rev.ID, tenantID, rev.RequestID, rev.Revision, rev.RunID, rev.Commit, rev.TasksMDDigest, rev.BundleDigest, string(files), bundle,
			created); err != nil {
			return fmt.Errorf("postgres: append openspec change revision: %w", err)
		}
		if keepText <= 0 {
			return nil
		}
		if _, err := db.Exec(ctx, `
			UPDATE request.openspec_change_revisions SET bundle_json = NULL
			WHERE tenant_id = $1 AND request_id = $2 AND revision <= $3 AND bundle_json IS NOT NULL`,
			tenantID, rev.RequestID, rev.Revision-keepText); err != nil {
			return fmt.Errorf("postgres: prune openspec change revision text: %w", err)
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
		if notUUID(requestID) {
			return nil
		}
		if limit <= 0 || limit > maxRevisionPage {
			limit = maxRevisionPage
		}
		q := strings.Builder{}
		q.WriteString(`SELECT ` + revisionIndexColumns + ` FROM request.openspec_change_revisions WHERE tenant_id = $1 AND request_id = $2`)
		args := []any{tenantID, requestID}
		if beforeRevision > 0 {
			args = append(args, beforeRevision)
			q.WriteString(fmt.Sprintf(` AND revision < $%d`, len(args)))
		}
		args = append(args, limit)
		q.WriteString(fmt.Sprintf(` ORDER BY revision DESC LIMIT $%d`, len(args)))
		rows, err := db.Query(ctx, q.String(), args...)
		if err != nil {
			return fmt.Errorf("postgres: list openspec change revisions: %w", err)
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
	return r.getOne(ctx, requestID, `request_id = $2 AND revision = $3`, requestID, revision)
}

func (r *OpenSpecChangeRevisionRepository) GetByRun(ctx context.Context, runID string) (domain.OpenSpecChangeRevision, bool, error) {
	return r.getOne(ctx, runID, `run_id = $2`, runID)
}

// getOne reads one version with its text; uuidArg is the id the predicate compares, so a malformed one is "not found".
func (r *OpenSpecChangeRevisionRepository) getOne(ctx context.Context, uuidArg, predicate string, args ...any) (domain.OpenSpecChangeRevision, bool, error) {
	var rev domain.OpenSpecChangeRevision
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(uuidArg) {
			return nil
		}
		var err error
		rev, err = scanPlanRevision(db.QueryRow(ctx, `SELECT `+revisionFullColumns+` FROM request.openspec_change_revisions WHERE tenant_id = $1 AND `+predicate,
			append([]any{tenantID}, args...)...), true)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("postgres: get openspec change revision: %w", err)
		}
		found = true
		return nil
	})
	return rev, found, err
}
