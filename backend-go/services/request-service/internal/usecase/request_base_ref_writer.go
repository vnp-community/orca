package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RequestBaseRefWriter stores the base ref of a Request. It is a narrow port beside RequestRepository (like the execution-mode
// pin) so adding it does not force every Request store fake to change.
type RequestBaseRefWriter interface {
	// SetBaseRef replaces the ref, bumps the version and returns the new row. A stale expectedVersion is REQUEST_VERSION_CONFLICT,
	// an unknown or foreign id is REQUEST_NOT_FOUND, an overlong ref is REQUEST_BASE_REF_INVALID. It never checks the worktree
	// lock: the caller does that in the same transaction, after taking the row lock (RequestLocker).
	SetBaseRef(ctx context.Context, id, baseRef string, expectedVersion int64) (domain.Request, error)
}
