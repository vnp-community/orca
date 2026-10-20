package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// OutcomeReviewContextRepository stores the locked design and the checked results of context runs.
type OutcomeReviewContextRepository struct {
	*Repository
}

func NewOutcomeReviewContextRepository(r *Repository) *OutcomeReviewContextRepository {
	return &OutcomeReviewContextRepository{Repository: r}
}

var _ usecase.ReviewContextStore = (*OutcomeReviewContextRepository)(nil)

const contextColumns = `id, request_id, kind, digest, prompt_version, payload, created_at, expires_at`

func scanContext(row interface{ Scan(dest ...any) error }, tenantID string) (usecase.ReviewContextRecord, error) {
	var r usecase.ReviewContextRecord
	var kind string
	if err := row.Scan(&r.ID, &r.RequestID, &kind, &r.Digest, &r.PromptVersion, &r.Payload, &r.CreatedAt, &r.ExpiresAt); err != nil {
		return usecase.ReviewContextRecord{}, err
	}
	r.TenantID, r.Kind = tenantID, usecase.ReviewContextKind(kind)
	r.CreatedAt, r.ExpiresAt = r.CreatedAt.UTC(), r.ExpiresAt.UTC()
	return r, nil
}

func (r *OutcomeReviewContextRepository) Put(ctx context.Context, in usecase.ReviewContextRecord) (usecase.ReviewContextRecord, error) {
	var out usecase.ReviewContextRecord
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, err := db.ExecContext(ctx, `INSERT INTO outcome_review_contexts (id, tenant_id, request_id, kind, digest, prompt_version, payload, expires_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE payload = VALUES(payload), prompt_version = VALUES(prompt_version), expires_at = VALUES(expires_at), created_at = CURRENT_TIMESTAMP(6)`,
			in.ID, tenantID, in.RequestID, string(in.Kind), in.Digest, in.PromptVersion, string(in.Payload), in.ExpiresAt.UTC()); err != nil {
			return fmt.Errorf("mysql: put outcome review context: %w", err)
		}
		var err error
		out, err = scanContext(db.QueryRowContext(ctx, `SELECT `+contextColumns+` FROM outcome_review_contexts
			WHERE tenant_id = ? AND request_id = ? AND kind = ? AND digest = ?`, tenantID, in.RequestID, string(in.Kind), in.Digest), tenantID)
		return err
	})
	return out, err
}

func (r *OutcomeReviewContextRepository) Get(ctx context.Context, requestID string, kind usecase.ReviewContextKind, digest string, now time.Time) (usecase.ReviewContextRecord, bool, error) {
	var out usecase.ReviewContextRecord
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		var err error
		out, err = scanContext(db.QueryRowContext(ctx, `SELECT `+contextColumns+` FROM outcome_review_contexts
			WHERE tenant_id = ? AND request_id = ? AND kind = ? AND digest = ? AND expires_at > ?`, tenantID, requestID, string(kind), digest, now.UTC()), tenantID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return out, found, err
}

func (r *OutcomeReviewContextRepository) Latest(ctx context.Context, requestID string, kind usecase.ReviewContextKind, now time.Time) (usecase.ReviewContextRecord, bool, error) {
	var out usecase.ReviewContextRecord
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		var err error
		out, err = scanContext(db.QueryRowContext(ctx, `SELECT `+contextColumns+` FROM outcome_review_contexts
			WHERE tenant_id = ? AND request_id = ? AND kind = ? AND expires_at > ? ORDER BY created_at DESC, id DESC LIMIT 1`, tenantID, requestID, string(kind), now.UTC()), tenantID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return out, found, err
}
