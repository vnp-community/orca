package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// checkStorableBaseRef stops an overlong ref before the database sees it, so the outcome never depends on the server's
// length enforcement (the CHECK here, strict mode on MySQL).
func checkStorableBaseRef(ref string) error {
	if len(ref) > domain.MaxWorktreeBaseRefBytes {
		return domain.ErrRequestBaseRefInvalid("must be at most 255 bytes")
	}
	return nil
}

// SetBaseRef replaces the base ref under optimistic locking and bumps the version. It does not decide whether the ref may still
// change: the caller checks the worktree lock in the same transaction (after LockRequest) so no worktree can slip in between.
func (r *RequestRepository) SetBaseRef(ctx context.Context, id, baseRef string, expectedVersion int64) (domain.Request, error) {
	if err := checkStorableBaseRef(baseRef); err != nil {
		return domain.Request{}, err
	}
	var out domain.Request
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(id) {
			return domain.ErrRequestNotFound(id)
		}
		row := db.QueryRow(ctx, `UPDATE request.requests SET base_ref = $3, version = version + 1, updated_at = now()
			WHERE id = $1 AND tenant_id = $2 AND version = $4
			RETURNING `+requestColumns, id, tenantID, baseRef, expectedVersion)
		got, err := scanRequest(row)
		if err == nil {
			out = got
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: set base ref: %w", err)
		}
		return r.classifyMissing(ctx, db, "request", tenantID, id, domain.ErrRequestNotFound(id), domain.ErrRequestVersionConflict(id, expectedVersion))
	})
	return out, err
}
