package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// WorktreePresence says whether a Request already has a worktree, which is what freezes its base ref. A reason of "" means no.
type WorktreePresence interface {
	// Has fails with REQUEST_NOT_FOUND for an unknown Request. A finished Request answers true with reason "terminal".
	Has(ctx context.Context, requestID string) (present bool, reason domain.BaseRefLockReason, err error)
	// HasMany answers for a whole page in one lookup; Requests without a lock are left out of the map.
	HasMany(ctx context.Context, requestIDs []string) (map[string]domain.BaseRefLockReason, error)
}

// TaskAwareWorktreePresence adds the worktrees of tasks, which live in task-service, to what request-service stores itself
// (finished Requests and OpenSpec proposal worktrees). The stored part answers first: it costs one query and no network call.
type TaskAwareWorktreePresence struct {
	Stored WorktreePresence
	Tasks  TaskClient
}

var _ WorktreePresence = (*TaskAwareWorktreePresence)(nil)

func (p *TaskAwareWorktreePresence) Has(ctx context.Context, requestID string) (bool, domain.BaseRefLockReason, error) {
	present, reason, err := p.Stored.Has(ctx, requestID)
	if err != nil || present {
		return present, reason, err
	}
	locked, err := p.withTaskWorktree(ctx, []string{requestID})
	if err != nil {
		return false, domain.BaseRefUnlocked, err
	}
	if locked[requestID] {
		return true, domain.BaseRefLockedTask, nil
	}
	return false, domain.BaseRefUnlocked, nil
}

func (p *TaskAwareWorktreePresence) HasMany(ctx context.Context, requestIDs []string) (map[string]domain.BaseRefLockReason, error) {
	out, err := p.Stored.HasMany(ctx, requestIDs)
	if err != nil {
		return nil, err
	}
	var open []string
	for _, id := range requestIDs {
		if _, locked := out[id]; !locked {
			open = append(open, id)
		}
	}
	if len(open) == 0 {
		return out, nil
	}
	withTask, err := p.withTaskWorktree(ctx, open)
	if err != nil {
		return nil, err
	}
	for id := range withTask {
		out[id] = domain.BaseRefLockedTask
	}
	return out, nil
}

// withTaskWorktree asks task-service once for all ids. A task-service failure is an error, never "no worktree": answering no
// would let a person change the base ref of a Request whose worktree we simply could not see.
func (p *TaskAwareWorktreePresence) withTaskWorktree(ctx context.Context, requestIDs []string) (map[string]bool, error) {
	tasks, err := p.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: requestIDs})
	if err != nil {
		return nil, domain.ErrExecutionTaskServiceUnavailable(err)
	}
	out := make(map[string]bool)
	for _, t := range tasks {
		if t.WorktreeID != "" {
			out[t.RequestID] = true
		}
	}
	return out, nil
}
