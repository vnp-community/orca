package usecase

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// executionAccessTTL is how long a confirmed grant is trusted before task-service is asked again.
const executionAccessTTL = 2 * time.Minute

// ExecutionAccess gives the execution actor the right to run the Request's Plan tree. Only the user who committed
// the Plan owns it in task-service, but whoever approves the Plan, starts a Phase or reported the Request may drive
// its execution. Those people were authorized here (approval policy, StartPhaseAuthorizer, ownership), so request-service
// writes the missing grant, level 'user' (read, write, execute; not manage), as the Plan's owner.
type ExecutionAccess struct {
	Tasks TaskClient
	Log   *slog.Logger
	// Now is replaceable in tests.
	Now func() time.Time

	mu    sync.Mutex
	known map[string]time.Time
}

// Ensure makes actor able to execute the Request's live task tree. A Request with no tree yet needs nothing.
// domain.ErrExecuteForbidden when the tree has no owner who can write the grant.
func (a *ExecutionAccess) Ensure(ctx context.Context, req domain.Request, actor string) error {
	if actor == "" {
		return nil
	}
	tasks, err := a.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: []string{req.ID}, TaskTypes: executionTaskTypes})
	if err != nil {
		return err
	}
	return a.EnsureOnRoots(ctx, tasks, actor)
}

// EnsureOnRoots is Ensure for a caller that already listed the Request's tasks. The roots are the live tasks with no
// parent: the Plan, or the single fix task of a hotfix, which has no Plan.
func (a *ExecutionAccess) EnsureOnRoots(ctx context.Context, tasks []domain.TaskView, actor string) error {
	for _, t := range tasks {
		if t.ParentID == "" && t.Status != domain.TaskStatusCancelled {
			if err := a.ensureOnRoot(ctx, t, actor); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *ExecutionAccess) ensureOnRoot(ctx context.Context, root domain.TaskView, actor string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ErrRequestTenantRequired()
	}
	key := tenantID + "|" + root.ID + "|" + actor
	if a.confirmed(key) {
		return nil
	}
	ok, err := a.Tasks.CanExecute(ctx, root.ID, actor)
	if err != nil {
		return err
	}
	if !ok {
		if root.OwnerID == "" || root.OwnerID == actor {
			return domain.ErrExecuteForbidden()
		}
		if err := a.Tasks.GrantExecute(ctx, root.ID, root.OwnerID, actor); err != nil {
			if errors.Is(err, domain.ErrTaskForbidden) {
				return domain.ErrExecuteForbidden()
			}
			return err
		}
		a.log().InfoContext(ctx, "granted execute access on the plan tree to the execution actor",
			slog.String("request_id", root.RequestID), slog.String("root_id", root.ID), slog.String("actor", actor), slog.String("granted_as", root.OwnerID))
	}
	a.confirm(key)
	return nil
}

func (a *ExecutionAccess) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a *ExecutionAccess) log() *slog.Logger {
	if a.Log != nil {
		return a.Log
	}
	return slog.Default()
}

func (a *ExecutionAccess) confirmed(key string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	until, ok := a.known[key]
	return ok && a.now().Before(until)
}

func (a *ExecutionAccess) confirm(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.known == nil {
		a.known = map[string]time.Time{}
	}
	now := a.now()
	for k, until := range a.known {
		if !now.Before(until) {
			delete(a.known, k)
		}
	}
	a.known[key] = now.Add(executionAccessTTL)
}
