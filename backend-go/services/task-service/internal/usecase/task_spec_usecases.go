package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// MaxTaskSpecsPerRead bounds GetTaskSpecs and the adapters' IN lists.
const MaxTaskSpecsPerRead = 200

// maxSpecSubtreeDepth bounds the walk LockTaskSpecs does below a plan or phase.
const maxSpecSubtreeDepth = domain.DefaultMaxAncestorDepth

// taskPermissionChecker is *ResolvePermission, kept narrow so tests need no OPA.
type taskPermissionChecker interface {
	Execute(ctx context.Context, in ResolvePermissionInput) (domain.GrantLevel, error)
}

// requireGrantWhenUser checks the caller's grant when a user identity is present. Calls from
// sibling services (request-service consumers) carry only the tenant, as for ReportTaskExecutionResult
// and GetTask today; task-service has no peer identity yet, so those calls are tenant-scoped only.
func requireGrantWhenUser(ctx context.Context, perm taskPermissionChecker, taskID, action string) error {
	userID, _ := tenant.UserID(ctx)
	if userID == "" || perm == nil {
		return nil
	}
	_, err := perm.Execute(ctx, ResolvePermissionInput{TaskID: taskID, UserID: userID, Action: action})
	return err
}

func specError(err error) error {
	switch {
	case errors.Is(err, domain.ErrTaskSpecInvalid):
		return apperrors.New(apperrors.KindInvalidArgument, "TASK_SPEC_INVALID", err.Error(), err)
	case errors.Is(err, domain.ErrTaskSpecLocked):
		return apperrors.New(apperrors.KindFailedPrecondition, "TASK_SPEC_LOCKED", "task spec is locked by an approved plan", err)
	case errors.Is(err, domain.ErrTaskSpecVersionConflict):
		return apperrors.New(apperrors.KindAborted, "TASK_SPEC_VERSION_CONFLICT", "task spec was changed by someone else", err)
	case errors.Is(err, domain.ErrTaskSpecNotFound):
		return apperrors.New(apperrors.KindNotFound, "TASK_SPEC_NOT_FOUND", "task spec not found", err)
	}
	return apperrors.New(apperrors.KindInternal, "TASK_SPEC_FAILED", "task spec operation failed", err)
}

type SetTaskSpecInput struct {
	TaskID          string
	SchemaVersion   int
	SpecJSON        []byte
	ExpectedVersion int64
}

// SetTaskSpec stores a task's spec. ExpectedVersion 0 creates it; resending an identical spec
// with 0 succeeds without a version bump so request-service can safely retry a half-finished CommitPlan.
type SetTaskSpec struct {
	tasks TaskRepository
	specs TaskSpecRepository
	perm  taskPermissionChecker
}

func NewSetTaskSpec(tasks TaskRepository, specs TaskSpecRepository, perm taskPermissionChecker) *SetTaskSpec {
	return &SetTaskSpec{tasks: tasks, specs: specs, perm: perm}
}

func (uc *SetTaskSpec) Execute(ctx context.Context, in SetTaskSpecInput) (domain.TaskSpec, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.TaskSpec{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.TaskID == "" {
		return domain.TaskSpec{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_MISSING_ID", "task_id is required", nil)
	}
	if _, err := uc.tasks.Get(ctx, tenantID, in.TaskID); err != nil {
		return domain.TaskSpec{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	if err := requireGrantWhenUser(ctx, uc.perm, in.TaskID, "write"); err != nil {
		return domain.TaskSpec{}, err
	}
	spec, err := domain.NewTaskSpec(in.TaskID, tenantID, in.SchemaVersion, in.SpecJSON)
	if err != nil {
		return domain.TaskSpec{}, specError(err)
	}
	if in.ExpectedVersion == 0 {
		existing, err := uc.specs.GetMany(ctx, tenantID, []string{in.TaskID})
		if err != nil {
			return domain.TaskSpec{}, specError(err)
		}
		if len(existing) == 1 && existing[0].Digest == spec.Digest && existing[0].SchemaVersion == spec.SchemaVersion {
			return canonicalForRead(existing[0])
		}
	}
	saved, err := uc.specs.Upsert(ctx, spec, in.ExpectedVersion)
	if err != nil {
		return domain.TaskSpec{}, specError(err)
	}
	return canonicalForRead(saved)
}

func canonicalForRead(s domain.TaskSpec) (domain.TaskSpec, error) {
	out, err := s.Canonicalized()
	if err != nil {
		return domain.TaskSpec{}, specError(err)
	}
	return out, nil
}

// GetTaskSpecs reads specs in bulk. Like GetTask it is tenant-scoped; tasks without a spec are omitted.
type GetTaskSpecs struct {
	specs TaskSpecRepository
}

func NewGetTaskSpecs(specs TaskSpecRepository) *GetTaskSpecs { return &GetTaskSpecs{specs: specs} }

func (uc *GetTaskSpecs) Execute(ctx context.Context, taskIDs []string) ([]domain.TaskSpec, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if len(taskIDs) == 0 || len(taskIDs) > MaxTaskSpecsPerRead {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "TASK_SPEC_TOO_MANY", "task_ids must hold between 1 and 200 ids", nil)
	}
	found, err := uc.specs.GetMany(ctx, tenantID, taskIDs)
	if err != nil {
		return nil, specError(err)
	}
	out := make([]domain.TaskSpec, 0, len(found))
	for _, s := range found {
		c, err := canonicalForRead(s)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// LockTaskSpecs freezes every spec under a plan or phase when its approval lands. Idempotent: a
// second call locks nothing and reports 0.
type LockTaskSpecs struct {
	tasks TaskRepository
	specs TaskSpecRepository
	perm  taskPermissionChecker
	clock Clock
}

func NewLockTaskSpecs(tasks TaskRepository, specs TaskSpecRepository, perm taskPermissionChecker) *LockTaskSpecs {
	return &LockTaskSpecs{tasks: tasks, specs: specs, perm: perm, clock: SystemClock{}}
}

func (uc *LockTaskSpecs) WithClock(c Clock) *LockTaskSpecs {
	uc.clock = c
	return uc
}

func (uc *LockTaskSpecs) Execute(ctx context.Context, planTaskID string) (int, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return 0, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if planTaskID == "" {
		return 0, apperrors.New(apperrors.KindInvalidArgument, "TASK_MISSING_ID", "plan_task_id is required", nil)
	}
	if err := requireGrantWhenUser(ctx, uc.perm, planTaskID, "write"); err != nil {
		return 0, err
	}
	nodes, _, err := uc.tasks.GetSubtree(ctx, tenantID, planTaskID, maxSpecSubtreeDepth)
	if err != nil {
		return 0, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found while resolving subtree", err)
	}
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.ID)
	}
	locked, err := uc.specs.LockSubtree(ctx, tenantID, ids, uc.clock.Now().UTC())
	if err != nil {
		return 0, specError(err)
	}
	slog.InfoContext(ctx, "task: specs locked", slog.String("plan_task_id", planTaskID), slog.Int("tasks", len(ids)), slog.Int("locked", locked))
	return locked, nil
}
