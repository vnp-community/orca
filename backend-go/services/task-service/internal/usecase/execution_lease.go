package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// ExecutionLeaseRepository makes Engine 1 (direct_agent) runs recoverable.
//
// The run executes in a goroutine inside one task-service process; before this
// port a restart left the task at in_progress with nothing to clear it. A
// lease on the execution_links row, kept alive by a heartbeat while the run is
// alive, lets any instance detect an abandoned run. Lease times use the
// database clock so instances with skewed clocks agree.
type ExecutionLeaseRepository interface {
	// StartLease stamps the link with an owner, the task's pre-dispatch status
	// (so recovery can restore it) and an expiry of now()+ttl.
	StartLease(ctx context.Context, tenantID, linkID, owner, previousStatus string, ttl time.Duration) error
	// RenewLease extends the expiry. held is false when the link is no longer
	// in progress under this owner (it was swept or finished).
	RenewLease(ctx context.Context, tenantID, linkID, owner string, ttl time.Duration) (held bool, err error)
	// ClaimExpired atomically marks up to limit expired direct_agent links
	// failed and returns them; concurrent callers never receive the same row.
	ClaimExpired(ctx context.Context, limit int) ([]domain.ExpiredRun, error)
}

// Defaults chosen so a healthy run renews ~3 times per TTL: one missed
// heartbeat does not trigger recovery, a dead process is noticed within ~TTL.
const (
	DefaultLeaseTTL         = 90 * time.Second
	DefaultLeaseHeartbeat   = 30 * time.Second
	DefaultRecoveryInterval = 30 * time.Second
	recoveryBatch           = 50
)

// WithExecutionLeases enables lease + heartbeat for direct_agent dispatches.
// Left unset (nil), dispatch behaves as before. owner identifies this process.
func (uc *ExecuteTask) WithExecutionLeases(leases ExecutionLeaseRepository, owner string, ttl, heartbeat time.Duration) *ExecuteTask {
	if ttl <= 0 {
		ttl = DefaultLeaseTTL
	}
	if heartbeat <= 0 || heartbeat >= ttl {
		heartbeat = ttl / 3
	}
	uc.leases, uc.leaseOwner, uc.leaseTTL, uc.leaseHeartbeat = leases, owner, ttl, heartbeat
	return uc
}

// startHeartbeat renews the lease until the returned stop func is called.
func (uc *ExecuteTask) startHeartbeat(ctx context.Context, tenantID, linkID string) (stop func()) {
	if uc.leases == nil {
		return func() {}
	}
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		t := time.NewTicker(uc.leaseHeartbeat)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				held, err := uc.leases.RenewLease(ctx, tenantID, linkID, uc.leaseOwner, uc.leaseTTL)
				if err != nil {
					// Keep trying: a transient DB error should not abandon a live run.
					slog.WarnContext(ctx, "task: lease renew failed", slog.String("link_id", linkID), slog.Any("error", err))
					continue
				}
				if !held {
					slog.WarnContext(ctx, "task: lease lost, run was already recovered", slog.String("link_id", linkID))
					return
				}
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}

// RecoverInterruptedExecutions reverts tasks whose direct_agent run was
// abandoned (its lease expired). Safe to run on every instance concurrently:
// ClaimExpired hands each expired row to exactly one caller.
type RecoverInterruptedExecutions struct {
	leases ExecutionLeaseRepository
	repo   TaskRepository
}

func NewRecoverInterruptedExecutions(leases ExecutionLeaseRepository, repo TaskRepository) *RecoverInterruptedExecutions {
	return &RecoverInterruptedExecutions{leases: leases, repo: repo}
}

// Execute returns how many tasks were reverted.
func (uc *RecoverInterruptedExecutions) Execute(ctx context.Context) (int, error) {
	expired, err := uc.leases.ClaimExpired(ctx, recoveryBatch)
	if err != nil {
		return 0, err
	}
	reverted := 0
	for _, run := range expired {
		task, err := uc.repo.Get(ctx, run.TenantID, run.TaskID)
		if err != nil {
			slog.WarnContext(ctx, "task: recovery could not load task", slog.String("task_id", run.TaskID), slog.Any("error", err))
			continue
		}
		// Only undo our own dispatch: if the task moved on (finished, or a
		// newer dispatch took over the active link), leave it alone.
		if task.Status != domain.StatusInProgress || task.ActiveExecutionLinkID != run.LinkID {
			continue
		}
		restore := domain.Status(run.PreviousStatus)
		if restore == "" || restore == domain.StatusInProgress {
			restore = domain.StatusOpen
		}
		if err := uc.repo.UpdateStatus(ctx, run.TenantID, run.TaskID, restore); err != nil {
			slog.WarnContext(ctx, "task: recovery could not revert task", slog.String("task_id", run.TaskID), slog.Any("error", err))
			continue
		}
		slog.WarnContext(ctx, "task: reverted interrupted direct_agent run",
			slog.String("task_id", run.TaskID), slog.String("link_id", run.LinkID), slog.String("status", string(restore)))
		reverted++
	}
	return reverted, nil
}

// RunRecoveryLoop sweeps on a fixed interval until ctx is cancelled. A failed
// sweep is logged and retried on the next tick — it must never end the loop.
func (uc *RecoverInterruptedExecutions) RunRecoveryLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = DefaultRecoveryInterval
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := uc.Execute(ctx); err != nil {
				slog.ErrorContext(ctx, "task: recovery sweep failed", slog.Any("error", err))
			}
		}
	}
}

// TaskExecutionClaimer moves a task to in_progress only if it still has the
// status the caller read — a compare-and-set. ExecuteTask's earlier
// read-then-write let two near-simultaneous Execute calls both pass the "not
// in progress" check; with this exactly one wins.
type TaskExecutionClaimer interface {
	// ClaimForExecution returns false when the task's status is no longer from.
	ClaimForExecution(ctx context.Context, tenantID, taskID string, from domain.Status) (claimed bool, err error)
}

// WithExecutionClaim makes Execute claim the task atomically. nil keeps the
// previous read-then-write behavior.
func (uc *ExecuteTask) WithExecutionClaim(c TaskExecutionClaimer) *ExecuteTask {
	uc.claimer = c
	return uc
}
