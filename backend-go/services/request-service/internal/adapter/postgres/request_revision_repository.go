package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && (constraint == "" || pgErr.ConstraintName == constraint)
}

const revisionColumns = `id, tenant_id, request_id, revision, cause, snapshot, digest, actor_id, actor_kind, clarification_id, created_at`

func scanRevision(row pgx.Row) (domain.RequestRevision, error) {
	var (
		rev           domain.RequestRevision
		cause, kind   string
		actor, clarif *string
		created       time.Time
	)
	if err := row.Scan(&rev.ID, &rev.TenantID, &rev.RequestID, &rev.Revision, &cause, &rev.Snapshot, &rev.Digest, &actor, &kind, &clarif, &created); err != nil {
		return domain.RequestRevision{}, err
	}
	// jsonb prints with spaces and its own key order; hand back the canonical bytes the digest was taken over.
	canon, err := domain.CanonicalJSON(rev.Snapshot)
	if err != nil {
		return domain.RequestRevision{}, fmt.Errorf("postgres: snapshot of revision %d is not canonicalisable: %w", rev.Revision, err)
	}
	rev.Snapshot = canon
	rev.Cause, rev.ActorKind = domain.RevisionCause(cause), domain.RevisionActorKind(kind)
	rev.ActorID, rev.ClarificationID, rev.CreatedAt = derefString(actor), derefString(clarif), created.UTC()
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
		_, err := db.Exec(ctx, `
			INSERT INTO request.request_revisions (id, tenant_id, request_id, revision, cause, snapshot, digest, actor_id, actor_kind, clarification_id, created_at)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10::uuid, $11)`,
			id, tenantID, rev.RequestID, rev.Revision, string(rev.Cause), string(rev.Snapshot), rev.Digest,
			nullIfEmpty(rev.ActorID), string(rev.ActorKind), nullIfEmpty(rev.ClarificationID), at)
		if isUniqueViolation(err, "request_revisions_unique_revision") {
			return domain.ErrRequestVersionConflict(rev.RequestID, int64(rev.Revision))
		}
		if err != nil {
			return fmt.Errorf("postgres: append request revision: %w", err)
		}
		return nil
	})
}

func (r *RequestRevisionRepository) Get(ctx context.Context, requestID string, revision int) (domain.RequestRevision, error) {
	var out domain.RequestRevision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(requestID); perr != nil {
			return domain.ErrRequestRevisionNotFound(requestID, revision)
		}
		got, err := scanRevision(db.QueryRow(ctx, `SELECT `+revisionColumns+` FROM request.request_revisions
			WHERE tenant_id = $1 AND request_id = $2 AND revision = $3`, tenantID, requestID, revision))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrRequestRevisionNotFound(requestID, revision)
		}
		if err != nil {
			return fmt.Errorf("postgres: get request revision: %w", err)
		}
		out = got
		return nil
	})
	return out, err
}

func (r *RequestRevisionRepository) List(ctx context.Context, requestID string, afterRevision, limit int) ([]domain.RequestRevision, error) {
	var out []domain.RequestRevision
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(requestID); perr != nil {
			return nil
		}
		rows, err := db.Query(ctx, `SELECT `+revisionColumns+` FROM request.request_revisions
			WHERE tenant_id = $1 AND request_id = $2 AND revision > $3 ORDER BY revision LIMIT $4`, tenantID, requestID, afterRevision, limit)
		if err != nil {
			return fmt.Errorf("postgres: list request revisions: %w", err)
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
