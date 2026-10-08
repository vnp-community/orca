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
	// SetPreviousStatus records the pre-dispatch status on a link that has no
	// lease (Engines 2/3), so a failure report can restore it.
	SetPreviousStatus(ctx context.Context, tenantID, linkID, status string) error
	// ClaimLegacyStuck claims direct_agent links with no lease that started
	// more than olderThan ago — runs from before leases existed.
	ClaimLegacyStuck(ctx context.Context, olderThan time.Duration, limit int) ([]domain.ExpiredRun, error)
	// ListOrphanedRuns lists tasks still in_progress although their active
	// link ended more than grace ago.
	ListOrphanedRuns(ctx context.Context, grace time.Duration, limit int) ([]domain.StuckTask, error)
	// ReleaseUnlinkedInProgress moves up to limit tasks that are in_progress with
	// no active execution link and whose updated_at is older than grace back to
	// open, in one race-safe compare-and-set; it returns how many it moved.
	ReleaseUnlinkedInProgress(ctx context.Context, grace time.Duration, limit int) (released int, err error)
	TaskExecutionReleaser
}

// Defaults chosen so a healthy run renews ~3 times per TTL: one missed
// heartbeat does not trigger recovery, a dead process is noticed within ~TTL.
const (
	DefaultLeaseTTL         = 90 * time.Second
	DefaultLeaseHeartbeat   = 30 * time.Second
	DefaultRecoveryInterval = 30 * time.Second
	// LegacyStuckAfter must exceed the executor's hard run cap (15 minutes) so a
	// still-alive run from a not-yet-upgraded instance is never swept.
	LegacyStuckAfter = 30 * time.Minute
	// OrphanGrace lets the normal completion writes land before a task whose
	// link already ended is treated as abandoned.
	OrphanGrace = 2 * time.Minute
	// UnlinkedGrace must dwarf the seconds ExecuteTask legitimately spends
	// in_progress before it records the link, so a healthy dispatch is never swept.
	UnlinkedGrace = 30 * time.Minute
	recoveryBatch = 50
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

// TaskExecutionReleaser moves an in_progress task back out of in_progress, but
// only while the given link is still its active one (compare-and-set), so a
// slow sweeper or a late failure report can never undo a newer dispatch.
type TaskExecutionReleaser interface {
	// events are written to the outbox in the same transaction, only when the CAS matched.
	ReleaseExecution(ctx context.Context, tenantID, taskID, linkID string, to domain.Status, events []domain.OutboxEvent) (released bool, err error)
}

// restoreStatus picks where a task goes after an abandoned/failed run: the
// status it had before the dispatch, or open when that was never recorded.
func restoreStatus(previous string) domain.Status {
	s := domain.Status(previous)
	if s == "" || s == domain.StatusInProgress {
		return domain.StatusOpen
	}
	return s
}

// RecoverInterruptedExecutions reverts tasks stuck at in_progress:
//   - a direct_agent run whose lease expired (its process died),
//   - a direct_agent run from before leases existed, long past the run cap,
//   - a task whose active link already ended (failed, or a direct_agent run
//     that completed without the task's completion write landing),
//   - a task in_progress with no link at all, long past any dispatch window
//     (from before execution_links existed); previous status is unknown, so open.
//
// Safe to run on every instance concurrently: link claims hand each row to one
// caller and every task change is a compare-and-set on the active link.
type RecoverInterruptedExecutions struct {
	leases ExecutionLeaseRepository
	// reconcile, when set, repairs plan/phase statuses after each sweep: the bulk
	// releases above change leaf tasks without naming them.
	reconcile *ReconcileContainerStatuses
	// tasks, when set, lets a recovery release emit statuschanged for request-owned tasks.
	tasks taskGetter
	clock Clock
}

// taskGetter is the read the recovery sweep needs to know a task's request_id.
type taskGetter interface {
	Get(ctx context.Context, tenantID, id string) (domain.Task, error)
}

// WithRunEvents makes recovery releases of request-owned tasks emit a `recovery` event.
func (uc *RecoverInterruptedExecutions) WithRunEvents(tasks taskGetter, clock Clock) *RecoverInterruptedExecutions {
	uc.tasks = tasks
	uc.clock = clock
	return uc
}

func (uc *RecoverInterruptedExecutions) recoveryEvents(ctx context.Context, tenantID, taskID, linkID string, to domain.Status, reason string) []domain.OutboxEvent {
	if uc.tasks == nil {
		return nil
	}
	t, err := uc.tasks.Get(ctx, tenantID, taskID)
	if err != nil {
		slog.WarnContext(ctx, "task: recovery could not load task for its event", slog.String("task_id", taskID), slog.Any("error", err))
		return nil
	}
	now := time.Now()
	if uc.clock != nil {
		now = uc.clock.Now()
	}
	return runEvents(t, domain.StatusInProgress, to, CauseRecovery, linkID, "", reason, now)
}

// WithContainerReconcile adds the plan/phase reconcile step to every sweep.
func (uc *RecoverInterruptedExecutions) WithContainerReconcile(r *ReconcileContainerStatuses) *RecoverInterruptedExecutions {
	uc.reconcile = r
	return uc
}

// Sweep runs one recovery pass and then the container reconcile; a failing step never hides the other.
func (uc *RecoverInterruptedExecutions) Sweep(ctx context.Context) error {
	_, err := uc.Execute(ctx)
	if uc.reconcile != nil {
		if _, rerr := uc.reconcile.Execute(ctx); rerr != nil {
			slog.ErrorContext(ctx, "task: container reconcile failed", slog.Any("error", rerr))
		}
	}
	return err
}

func NewRecoverInterruptedExecutions(leases ExecutionLeaseRepository) *RecoverInterruptedExecutions {
	return &RecoverInterruptedExecutions{leases: leases}
}

// Execute returns how many tasks were put back.
func (uc *RecoverInterruptedExecutions) Execute(ctx context.Context) (int, error) {
	reverted := 0

	expired, err := uc.leases.ClaimExpired(ctx, recoveryBatch)
	if err != nil {
		return 0, err
	}
	reverted += uc.releaseClaimed(ctx, expired, "lease expired")

	// The sweeps below are independent of the lease path: a failure there
	// is logged and must not hide what the first sweep already did.
	legacy, err := uc.leases.ClaimLegacyStuck(ctx, LegacyStuckAfter, recoveryBatch)
	if err != nil {
		slog.ErrorContext(ctx, "task: legacy stuck sweep failed", slog.Any("error", err))
	} else {
		reverted += uc.releaseClaimed(ctx, legacy, "pre-lease run past the run cap")
	}

	orphaned, err := uc.leases.ListOrphanedRuns(ctx, OrphanGrace, recoveryBatch)
	if err != nil {
		slog.ErrorContext(ctx, "task: orphaned run sweep failed", slog.Any("error", err))
	} else {
		for _, o := range orphaned {
			to := restoreStatus(o.PreviousStatus)
			if o.LinkStatus == "completed" {
				// The run finished; only the task's completion write was lost.
				to = domain.StatusReview
			}
			if uc.release(ctx, o.TenantID, o.TaskID, o.LinkID, to, "active link already "+o.LinkStatus) {
				reverted++
			}
		}
	}

	unlinked, err := uc.leases.ReleaseUnlinkedInProgress(ctx, UnlinkedGrace, recoveryBatch)
	if err != nil {
		slog.ErrorContext(ctx, "task: unlinked in_progress sweep failed", slog.Any("error", err))
	} else if unlinked > 0 {
		slog.WarnContext(ctx, "task: released in_progress tasks with no execution link", slog.Int("count", unlinked))
		reverted += unlinked
	}
	return reverted, nil
}

func (uc *RecoverInterruptedExecutions) releaseClaimed(ctx context.Context, runs []domain.ExpiredRun, reason string) int {
	n := 0
	for _, run := range runs {
		if uc.release(ctx, run.TenantID, run.TaskID, run.LinkID, restoreStatus(run.PreviousStatus), reason) {
			n++
		}
	}
	return n
}

func (uc *RecoverInterruptedExecutions) release(ctx context.Context, tenantID, taskID, linkID string, to domain.Status, reason string) bool {
	released, err := uc.leases.ReleaseExecution(ctx, tenantID, taskID, linkID, to, uc.recoveryEvents(ctx, tenantID, taskID, linkID, to, reason))
	if err != nil {
		slog.WarnContext(ctx, "task: recovery could not release task", slog.String("task_id", taskID), slog.Any("error", err))
		return false
	}
	if !released {
		return false // finished, or a newer dispatch owns it now
	}
	slog.WarnContext(ctx, "task: released stuck in_progress task",
		slog.String("task_id", taskID), slog.String("link_id", linkID), slog.String("status", string(to)), slog.String("reason", reason))
	return true
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
			if err := uc.Sweep(ctx); err != nil {
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
	// events are written to the outbox in the same transaction, only when the claim won.
	ClaimForExecution(ctx context.Context, tenantID, taskID string, from domain.Status, events []domain.OutboxEvent) (claimed bool, err error)
}

// WithExecutionClaim makes Execute claim the task atomically. nil keeps the
// previous read-then-write behavior.
func (uc *ExecuteTask) WithExecutionClaim(c TaskExecutionClaimer) *ExecuteTask {
	uc.claimer = c
	return uc
}
