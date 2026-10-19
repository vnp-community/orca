package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// WorktreePresence answers the part of "does this Request have a worktree?" that request-service stores itself: a finished
// Request, an OpenSpec proposal worktree (or the marker reserved before it is created) and an executing Request. Task worktrees
// live in task-service (usecase.TaskAwareWorktreePresence adds them for the stages after execution).
type WorktreePresence struct {
	*Repository
}

func NewWorktreePresence(r *Repository) *WorktreePresence { return &WorktreePresence{Repository: r} }

var _ usecase.WorktreePresence = (*WorktreePresence)(nil)

// An abandoned change keeps its row but its worktree is gone, so it no longer pins the base ref.
const worktreePresenceQuery = `
	SELECT r.id::text, r.status,
	       EXISTS (SELECT 1 FROM request.openspec_changes c
	               WHERE c.tenant_id = r.tenant_id AND c.request_id = r.id AND c.status <> 'abandoned')
	FROM request.requests r
	WHERE r.tenant_id = $1 AND r.id = ANY($2::uuid[])`

func (p *WorktreePresence) Has(ctx context.Context, requestID string) (bool, domain.BaseRefLockReason, error) {
	parsed, err := uuid.Parse(requestID)
	if err != nil {
		return false, domain.BaseRefUnlocked, domain.ErrRequestNotFound(requestID)
	}
	id := parsed.String() // the database answers in canonical lower case
	locks, found, err := p.lookup(ctx, []string{id})
	if err != nil {
		return false, domain.BaseRefUnlocked, err
	}
	if !found[id] {
		return false, domain.BaseRefUnlocked, domain.ErrRequestNotFound(requestID)
	}
	reason := locks[id]
	return reason != domain.BaseRefUnlocked, reason, nil
}

// HasMany reads the whole page in one statement; ids that are no uuid or belong to another tenant are simply absent.
func (p *WorktreePresence) HasMany(ctx context.Context, requestIDs []string) (map[string]domain.BaseRefLockReason, error) {
	ids := make([]string, 0, len(requestIDs))
	for _, id := range requestIDs {
		if u, err := uuid.Parse(id); err == nil {
			ids = append(ids, u.String())
		}
	}
	if len(ids) == 0 {
		return map[string]domain.BaseRefLockReason{}, nil
	}
	locks, _, err := p.lookup(ctx, ids)
	return locks, err
}

// lookup returns the lock reasons of the locked Requests and the set of ids that exist for the tenant.
func (p *WorktreePresence) lookup(ctx context.Context, ids []string) (map[string]domain.BaseRefLockReason, map[string]bool, error) {
	locks := make(map[string]domain.BaseRefLockReason)
	found := make(map[string]bool)
	err := p.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.Query(ctx, worktreePresenceQuery, tenantID, ids)
		if err != nil {
			return fmt.Errorf("postgres: worktree presence: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id, status string
			var proposal bool
			if err := rows.Scan(&id, &status, &proposal); err != nil {
				return fmt.Errorf("postgres: scan worktree presence: %w", err)
			}
			found[id] = true
			if reason := domain.StoredBaseRefLock(domain.RequestStatus(status), proposal); reason != domain.BaseRefUnlocked {
				locks[id] = reason
			}
		}
		return rows.Err()
	})
	return locks, found, err
}
