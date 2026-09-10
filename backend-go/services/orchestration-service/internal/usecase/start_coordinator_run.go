package usecase

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// StartCoordinatorRunInput mirrors the StartCoordinatorRunRequest RPC message.
type StartCoordinatorRunInput struct {
	OriginTaskID string
	SpecJSON     json.RawMessage
	WorktreeID   string
}

// StartCoordinatorRun is orchestration-service's entry point into the
// complex execution path (orchestration-service.md §2.2/§3): validates and
// persists a new running CoordinatorRun plus its expanded OrchestrationTask
// DAG, then returns immediately — the tick loop (TASK-TASKV1-005-10)
// autonomously advances it from there.
type StartCoordinatorRun struct {
	repo       CoordinatorRunRepository
	serializer HandleSerializer
}

func NewStartCoordinatorRun(repo CoordinatorRunRepository, serializer HandleSerializer) *StartCoordinatorRun {
	return &StartCoordinatorRun{repo: repo, serializer: serializer}
}

func (uc *StartCoordinatorRun) Execute(ctx context.Context, in StartCoordinatorRunInput) (domain.CoordinatorRun, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	if in.OriginTaskID == "" {
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_ORIGIN_TASK_ID", "origin_task_id is required", nil)
	}

	// coordinatorHandle is this run's own mailbox identity; format matches
	// assignee_handle's informal "kind:id" convention elsewhere in this service.
	coordinatorHandle := "coordinator:" + uuid.NewString()
	run, err := domain.NewCoordinatorRun("", tenantID, in.OriginTaskID, coordinatorHandle, in.SpecJSON, 0)
	if err != nil {
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_INVALID_RUN", "invalid coordinator run", err)
	}
	run.Status = domain.RunStatusRunning // NewCoordinatorRun defaults to Idle; StartCoordinatorRun's whole point is starting it immediately
	run.WorktreeID = in.WorktreeID

	// Validate spec_json shape up front (fail fast, clear error, no
	// partial DB write) via a throwaway ExpandSpec call — the repository
	// re-runs ExpandSpec itself inside its own transaction with the real
	// minted run id (see TASK-TASKV1-005-05's CreateWithTasks), so this
	// call's only job is rejecting a malformed spec before touching the DB.
	if _, err := domain.ExpandSpec(tenantID, "placeholder", in.OriginTaskID, in.SpecJSON); err != nil {
		return domain.CoordinatorRun{}, mapExpandSpecErr(err)
	}

	var out domain.CoordinatorRun
	err = uc.serializer.Do(ctx, in.OriginTaskID, func() error {
		created, err := uc.repo.CreateWithTasks(ctx, tenantID, run)
		if err != nil {
			return err
		}
		out = created
		return nil
	})
	if err != nil {
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindInternal, "ORCH_START_COORDINATOR_RUN_FAILED", "failed to start coordinator run", err)
	}
	return out, nil
}

// mapExpandSpecErr maps domain.ExpandSpec's sentinel errors to
// KindInvalidArgument — a malformed spec_json is always the caller's
// fault, never an internal failure.
func mapExpandSpecErr(err error) error {
	switch {
	case errors.Is(err, domain.ErrEmptySpec):
		return apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_SPEC", "spec_json must contain at least one node", err)
	case errors.Is(err, domain.ErrDuplicateTempID):
		return apperrors.New(apperrors.KindInvalidArgument, "ORCH_DUPLICATE_TEMP_ID", "spec_json has a duplicate tempId", err)
	case errors.Is(err, domain.ErrDanglingDep):
		return apperrors.New(apperrors.KindInvalidArgument, "ORCH_DANGLING_DEP", "spec_json has a dep referencing an unknown tempId", err)
	default:
		return apperrors.New(apperrors.KindInvalidArgument, "ORCH_INVALID_SPEC", "spec_json is invalid", err)
	}
}
