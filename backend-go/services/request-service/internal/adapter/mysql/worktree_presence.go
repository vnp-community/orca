package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// WorktreePresence answers the part of "does this Request have a worktree?" that request-service stores itself: a finished
// Request and an OpenSpec proposal worktree. Task worktrees live in task-service (usecase.TaskAwareWorktreePresence adds them).
type WorktreePresence struct {
	*Repository
}

func NewWorktreePresence(r *Repository) *WorktreePresence { return &WorktreePresence{Repository: r} }

var _ usecase.WorktreePresence = (*WorktreePresence)(nil)

// presenceChunk keeps one statement far below MySQL's placeholder limit.
const presenceChunk = 500

func (p *WorktreePresence) Has(ctx context.Context, requestID string) (bool, domain.BaseRefLockReason, error) {
	parsed, err := uuid.Parse(requestID)
	if err != nil {
		return false, domain.BaseRefUnlocked, domain.ErrRequestNotFound(requestID)
	}
	id := parsed.String() // ids are stored in canonical lower case
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

// HasMany reads the page in one statement per 500 ids; ids that are no uuid or belong to another tenant are simply absent.
func (p *WorktreePresence) HasMany(ctx context.Context, requestIDs []string) (map[string]domain.BaseRefLockReason, error) {
	out := make(map[string]domain.BaseRefLockReason)
	ids := make([]string, 0, len(requestIDs))
	for _, id := range requestIDs {
		if u, err := uuid.Parse(id); err == nil {
			ids = append(ids, u.String())
		}
	}
	for start := 0; start < len(ids); start += presenceChunk {
		end := min(start+presenceChunk, len(ids))
		locks, _, err := p.lookup(ctx, ids[start:end])
		if err != nil {
			return nil, err
		}
		for id, reason := range locks {
			out[id] = reason
		}
	}
	return out, nil
}

// lookup returns the lock reasons of the locked Requests and the set of ids that exist for the tenant.
func (p *WorktreePresence) lookup(ctx context.Context, ids []string) (map[string]domain.BaseRefLockReason, map[string]bool, error) {
	locks := make(map[string]domain.BaseRefLockReason)
	found := make(map[string]bool)
	err := p.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		// An abandoned change keeps its row but its worktree is gone, so it no longer pins the base ref.
		query := `SELECT r.id, r.status,
			EXISTS (SELECT 1 FROM openspec_changes c WHERE c.tenant_id = r.tenant_id AND c.request_id = r.id AND c.status <> 'abandoned')
			FROM requests r WHERE r.tenant_id = ? AND r.id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + `)`
		args := make([]any, 0, len(ids)+1)
		args = append(args, tenantID)
		for _, id := range ids {
			args = append(args, id)
		}
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("mysql: worktree presence: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id, status string
			var proposal bool
			if err := rows.Scan(&id, &status, &proposal); err != nil {
				return fmt.Errorf("mysql: scan worktree presence: %w", err)
			}
			found[id] = true
			if reason := lockReason(domain.RequestStatus(status), proposal); reason != domain.BaseRefUnlocked {
				locks[id] = reason
			}
		}
		return rows.Err()
	})
	return locks, found, err
}

// lockReason ranks a finished Request above a proposal worktree: it cannot change whatever else exists.
func lockReason(status domain.RequestStatus, proposal bool) domain.BaseRefLockReason {
	switch {
	case status.IsTerminal():
		return domain.BaseRefLockedTerminal
	case proposal:
		return domain.BaseRefLockedProposal
	}
	return domain.BaseRefUnlocked
}
