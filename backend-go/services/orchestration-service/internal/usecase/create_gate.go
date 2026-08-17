package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

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
}

func NewCreateGate(repo GateRepository, serializer HandleSerializer) *CreateGate {
	return &CreateGate{repo: repo, serializer: serializer}
}

func (uc *CreateGate) Execute(ctx context.Context, in CreateGateInput) (domain.DecisionGate, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DecisionGate{}, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	if in.DispatchContextID == "" {
		return domain.DecisionGate{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_DISPATCH_CONTEXT_ID", "dispatch_context_id is required", nil)
	}

	var gate domain.DecisionGate
	err = uc.serializer.Do(ctx, in.DispatchContextID, func() error {
		created, err := uc.repo.CreateGate(ctx, tenantID, in.DispatchContextID, in.Question, in.Options)
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
