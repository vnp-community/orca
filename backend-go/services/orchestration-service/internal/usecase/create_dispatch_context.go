package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// taskDispatchedPayload is orca.orchestration.task.dispatched's JSON
// payload shape (BE-SOL-003/TASK-FT-003-01). OriginTaskID is carried
// directly (when known — an ad-hoc coordinator-only dispatch has no
// orchestration_task_id, and so no origin_task_id to resolve) so
// api-gateway's task.activity channel (TASK-FT-003-04) can filter by it
// without a cross-service lookup.
type taskDispatchedPayload struct {
	OrchestrationTaskID string `json:"orchestration_task_id"`
	CoordinatorRunID    string `json:"coordinator_run_id"`
	OriginTaskID        string `json:"origin_task_id"`
	Handle              string `json:"handle"`
}

// CreateDispatchContextInput mirrors the CreateDispatchContextRequest RPC
// message 1:1 — see architecture/03's note that usecase granularity mirrors
// today's RPC methods so the TS->Go mapping stays traceable.
type CreateDispatchContextInput struct {
	Handle              string
	CoordinatorRunID    string
	OrchestrationTaskID string // optional; see ports.go's DispatchContextRepository doc comment
	WorktreeID          string // optional, caller-supplied — see domain.DispatchContext.WorktreeID
}

// CreateDispatchContext is routed through the HandleSerializer keyed by
// Handle, per orchestration-service.md §8: dispatch-context creation is the
// kind of write a synchronous domain-event handler can fire as an
// uncoordinated async DB call, and two such calls for the same handle must
// not interleave.
type CreateDispatchContext struct {
	repo       DispatchContextRepository
	serializer HandleSerializer
	// tasks resolves in.OrchestrationTaskID's OriginTaskID for the
	// task.dispatched outbox payload (BE-SOL-003/TASK-FT-003-01,
	// TASK-FT-003-04's task.activity filter) — nil-safe (an ad-hoc dispatch
	// has no orchestration task at all, see CreateDispatchContextInput's
	// doc comment) so existing callers/tests that don't care about the
	// outbox event still compile without wiring this dependency.
	tasks OrchestrationTaskRepository
}

func NewCreateDispatchContext(repo DispatchContextRepository, serializer HandleSerializer, tasks OrchestrationTaskRepository) *CreateDispatchContext {
	return &CreateDispatchContext{repo: repo, serializer: serializer, tasks: tasks}
}

func (uc *CreateDispatchContext) Execute(ctx context.Context, in CreateDispatchContextInput) (domain.DispatchContext, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DispatchContext{}, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	if in.Handle == "" {
		return domain.DispatchContext{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_HANDLE", "handle is required", nil)
	}

	// userID comes from the authenticated identity, never a client-supplied
	// field — same rule as tenantID. May be empty (ok, bool) for a
	// system-initiated dispatch with no end-user caller.
	userID, _ := tenant.UserID(ctx)

	// Outbox event (BE-SOL-003/TASK-FT-003-01) — best-effort: an
	// OriginTaskID lookup failure (or no tasks port wired, or no
	// OrchestrationTaskID at all for an ad-hoc dispatch) degrades to
	// "still create the dispatch context, event carries an empty
	// origin_task_id" rather than failing the whole call.
	var event domain.OutboxEvent
	var originTaskID string
	if in.OrchestrationTaskID != "" && uc.tasks != nil {
		if task, gerr := uc.tasks.Get(ctx, tenantID, in.OrchestrationTaskID); gerr == nil {
			originTaskID = task.OriginTaskID
		}
	}
	if payload, merr := json.Marshal(taskDispatchedPayload{
		OrchestrationTaskID: in.OrchestrationTaskID, CoordinatorRunID: in.CoordinatorRunID,
		OriginTaskID: originTaskID, Handle: in.Handle,
	}); merr == nil {
		event = domain.OutboxEvent{
			ID: uuid.NewString(), Subject: "orca.orchestration.task.dispatched",
			OccurredAt: time.Now().UTC(), PayloadJSON: payload,
		}
	}

	var result domain.DispatchContext
	err = uc.serializer.Do(ctx, in.Handle, func() error {
		created, err := uc.repo.CreateDispatchContext(ctx, tenantID, userID, in.WorktreeID, in.Handle, in.CoordinatorRunID, in.OrchestrationTaskID, event)
		if err != nil {
			return err
		}
		result = created
		return nil
	})
	if err != nil {
		return domain.DispatchContext{}, apperrors.New(apperrors.KindInternal, "ORCH_CREATE_DISPATCH_FAILED", "failed to create dispatch context", err)
	}
	return result, nil
}
