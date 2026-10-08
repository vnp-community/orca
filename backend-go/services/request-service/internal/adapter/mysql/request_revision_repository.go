package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type RequestRevisionRepository struct {
	*Repository
}

func NewRequestRevisionRepository(r *Repository) *RequestRevisionRepository {
	return &RequestRevisionRepository{Repository: r}
}

var _ usecase.RequestRevisionRepository = (*RequestRevisionRepository)(nil)

const revisionColumns = `id, tenant_id, request_id, revision, cause, snapshot, digest, actor_id, actor_kind, clarification_id, created_at`

func scanRevision(row rowScanner) (domain.RequestRevision, error) {
	var (
		rev           domain.RequestRevision
		cause, kind   string
		actor, clarif sql.NullString
	)
	if err := row.Scan(&rev.ID, &rev.TenantID, &rev.RequestID, &rev.Revision, &cause, &rev.Snapshot, &rev.Digest, &actor, &kind, &clarif, &rev.CreatedAt); err != nil {
		return domain.RequestRevision{}, err
	}
	// MySQL prints JSON with its own spacing and key order; hand back the canonical bytes the digest was taken over.
	canon, err := domain.CanonicalJSON(rev.Snapshot)
	if err != nil {
		return domain.RequestRevision{}, fmt.Errorf("mysql: snapshot of revision %d is not canonicalisable: %w", rev.Revision, err)
	}
	rev.Snapshot = canon
	rev.Cause, rev.ActorKind = domain.RevisionCause(cause), domain.RevisionActorKind(kind)
	rev.ActorID, rev.ClarificationID, rev.CreatedAt = actor.String, clarif.String, rev.CreatedAt.UTC()
	return rev, nil
}

func (r *RequestRevisionRepository) Append(ctx context.Context, rev domain.RequestRevision) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		id := rev.ID
		if id == "" {
			id = uuid.NewString()
		}
		at := rev.CreatedAt
		if at.IsZero() {
			at = time.Now().UTC()
		}
		_, err := db.ExecContext(ctx, `
			INSERT INTO request_revisions (id, tenant_id, request_id, revision, cause, snapshot, digest, actor_id, actor_kind, clarification_id, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			id, tenantID, rev.RequestID, rev.Revision, string(rev.Cause), string(rev.Snapshot), rev.Digest,
			nullIfEmpty(rev.ActorID), string(rev.ActorKind), nullIfEmpty(rev.ClarificationID), at)
		if isDuplicateKey(err, "request_revisions_unique_revision") {
			return domain.ErrRequestVersionConflict(rev.RequestID, int64(rev.Revision))
		}
		if err != nil {
			return fmt.Errorf("mysql: append request revision: %w", err)
		}
		return nil
	})
}

func (r *RequestRevisionRepository) Get(ctx context.Context, requestID string, revision int) (domain.RequestRevision, error) {
	var out domain.RequestRevision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		got, err := scanRevision(db.QueryRowContext(ctx, `SELECT `+revisionColumns+` FROM request_revisions
			WHERE tenant_id = ? AND request_id = ? AND revision = ?`, tenantID, requestID, revision))
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrRequestRevisionNotFound(requestID, revision)
		}
		if err != nil {
			return fmt.Errorf("mysql: get request revision: %w", err)
		}
		out = got
		return nil
	})
	return out, err
}

func (r *RequestRevisionRepository) List(ctx context.Context, requestID string, afterRevision, limit int) ([]domain.RequestRevision, error) {
	var out []domain.RequestRevision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT `+revisionColumns+` FROM request_revisions
			WHERE tenant_id = ? AND request_id = ? AND revision > ? ORDER BY revision LIMIT ?`, tenantID, requestID, afterRevision, limit)
		if err != nil {
			return fmt.Errorf("mysql: list request revisions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			rev, err := scanRevision(rows)
			if err != nil {
				return err
			}
			out = append(out, rev)
		}
		return rows.Err()
	})
	return out, err
}
