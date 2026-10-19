package mysql

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// checkStorableBaseRef stops an overlong ref before the database sees it: a MySQL server in non-strict mode would truncate a
// VARCHAR(255) value silently, which would store a different ref from the one the person chose.
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
		if _, perr := uuid.Parse(id); perr != nil {
			return domain.ErrRequestNotFound(id)
		}
		res, err := db.ExecContext(ctx, `UPDATE requests SET base_ref = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP(6)
			WHERE id = ? AND tenant_id = ? AND version = ?`, baseRef, id, tenantID, expectedVersion)
		if err != nil {
			return fmt.Errorf("mysql: set base ref: %w", err)
		}
		// version always changes, so affected rows is 1 on success even without clientFoundRows.
		if n, _ := res.RowsAffected(); n == 1 {
			got, err := scanRequest(db.QueryRowContext(ctx, `SELECT `+requestColumns+` FROM requests WHERE id = ? AND tenant_id = ?`, id, tenantID))
			if err != nil {
				return fmt.Errorf("mysql: reread request: %w", err)
			}
			out = got
			return nil
		}
		return r.classifyMissing(ctx, db, "request", tenantID, id, domain.ErrRequestNotFound(id), domain.ErrRequestVersionConflict(id, expectedVersion))
	})
	return out, err
}
