package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// decisionGateOpenedPayload is orca.orchestration.decision_gate.opened's
// JSON payload shape (BE-SOL-003/TASK-FT-003-02). OriginTaskID is carried
// directly (best-effort — see Execute) so api-gateway's task.activity
// channel (TASK-FT-003-04) can filter by it without a cross-service lookup.
type decisionGateOpenedPayload struct {
	DispatchContextID   string   `json:"dispatch_context_id"`
	OrchestrationTaskID string   `json:"orchestration_task_id"`
	OriginTaskID        string   `json:"origin_task_id"`
	Question            string   `json:"question"`
	Options             []string `json:"options"`
}

// CreateGateInput mirrors the CreateGateRequest RPC message. Question/Options
// now flow through from the gRPC adapter — see docs/execution-plan.md Epic C
// and README "Deviations from the design doc". CreateGateRequest also carries
// orchestration_task_id, but it is deliberately NOT threaded onto this
// struct: CreateGate derives the owning task from DispatchContextID itself
// via a locked read (see Execute below and the postgres repository's
// CreateGate), so there is no caller-supplied override to trust here.
type CreateGateInput struct {
	DispatchContextID string
	Question          string
	Options           []string
}

// CreateGate is keyed by DispatchContextID — the closest available
// substitute for an assignee_handle in this RPC's shape (the proto message
// carries no handle field), still preventing two concurrent CreateGate
// calls against the same dispatch context from interleaving.
type CreateGate struct {
	repo       GateRepository
	serializer HandleSerializer
	// dispatchContexts/tasks resolve DispatchContextID -> orchestration_task_id
	// -> origin_task_id for the decision_gate.opened outbox payload
	// (BE-SOL-003/TASK-FT-003-02, TASK-FT-003-04's task.activity filter) —
	// nil-safe (see Execute) so existing callers/tests that don't care
	// about the outbox event still compile without wiring these.
	dispatchContexts DispatchContextRepository
	tasks            OrchestrationTaskRepository
}

func NewCreateGate(repo GateRepository, serializer HandleSerializer, dispatchContexts DispatchContextRepository, tasks OrchestrationTaskRepository) *CreateGate {
	return &CreateGate{repo: repo, serializer: serializer, dispatchContexts: dispatchContexts, tasks: tasks}
}

func (uc *CreateGate) Execute(ctx context.Context, in CreateGateInput) (domain.DecisionGate, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DecisionGate{}, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	if in.DispatchContextID == "" {
		return domain.DecisionGate{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_DISPATCH_CONTEXT_ID", "dispatch_context_id is required", nil)
	}

	// Outbox event (BE-SOL-003/TASK-FT-003-02) — best-effort, two hops
	// (dispatch context -> orchestration task -> origin task id), each
	// independently degrading to an empty field rather than failing the
	// whole call; GateRepository.CreateGate below still does its own
	// authoritative, transaction-locked resolution of dispatchContextID ->
	// orchestration_task_id for the actual gate row.
	var orchestrationTaskID, originTaskID string
	if uc.dispatchContexts != nil {
		if dc, gerr := uc.dispatchContexts.GetDispatchContext(ctx, tenantID, in.DispatchContextID); gerr == nil {
			orchestrationTaskID = dc.OrchestrationTaskID
		}
	}
	if orchestrationTaskID != "" && uc.tasks != nil {
		if task, gerr := uc.tasks.Get(ctx, tenantID, orchestrationTaskID); gerr == nil {
			originTaskID = task.OriginTaskID
		}
	}
	var event domain.OutboxEvent
	if payload, merr := json.Marshal(decisionGateOpenedPayload{
		DispatchContextID: in.DispatchContextID, OrchestrationTaskID: orchestrationTaskID,
		OriginTaskID: originTaskID, Question: in.Question, Options: in.Options,
	}); merr == nil {
		event = domain.OutboxEvent{
			ID: uuid.NewString(), Subject: "orca.orchestration.decision_gate.opened",
			OccurredAt: time.Now().UTC(), PayloadJSON: payload,
		}
	}

	var gate domain.DecisionGate
	err = uc.serializer.Do(ctx, in.DispatchContextID, func() error {
		created, err := uc.repo.CreateGate(ctx, tenantID, in.DispatchContextID, in.Question, in.Options, event)
		if err != nil {
			return err
		}
		gate = created
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrDispatchContextNotFound):
			return domain.DecisionGate{}, apperrors.New(apperrors.KindNotFound, "ORCH_DISPATCH_CONTEXT_NOT_FOUND", "dispatch context not found", err)
		case errors.Is(err, ErrDispatchContextHasNoTask):
			return domain.DecisionGate{}, apperrors.New(apperrors.KindFailedPrecondition, "ORCH_DISPATCH_CONTEXT_NO_TASK", "dispatch context has no owning orchestration task yet", err)
		default:
			return domain.DecisionGate{}, apperrors.New(apperrors.KindInternal, "ORCH_CREATE_GATE_FAILED", "failed to create decision gate", err)
		}
	}
	return gate, nil
}
