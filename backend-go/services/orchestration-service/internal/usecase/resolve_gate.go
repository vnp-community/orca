package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// ResolveGateInput mirrors the ResolveGateRequest RPC message.
type ResolveGateInput struct {
	GateID      string
	OutcomeJSON string
}

// ResolveGateOutput carries the resolved gate plus any tasks whose status
// changed as a side effect (the promotion pass, §8).
type ResolveGateOutput struct {
	Gate            domain.DecisionGate
	AffectedTaskIDs []string
}

// ResolveGate is keyed by GateID — the closest available substitute for a
// coordinator_handle in this RPC's shape (ResolveGateRequest carries no
// handle field): it still guarantees the one-shot-resolution invariant
// (domain.ErrGateAlreadyResolved) can't race with itself for the same gate.
type ResolveGate struct {
	repo       GateRepository
	serializer HandleSerializer
}

func NewResolveGate(repo GateRepository, serializer HandleSerializer) *ResolveGate {
	return &ResolveGate{repo: repo, serializer: serializer}
}

func (uc *ResolveGate) Execute(ctx context.Context, in ResolveGateInput) (ResolveGateOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ResolveGateOutput{}, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	if in.GateID == "" {
		return ResolveGateOutput{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_GATE_ID", "gate_id is required", nil)
	}

	var out ResolveGateOutput
	err = uc.serializer.Do(ctx, in.GateID, func() error {
		gate, affected, err := uc.repo.ResolveGate(ctx, tenantID, in.GateID, in.OutcomeJSON)
		if err != nil {
			return err
		}
		out = ResolveGateOutput{Gate: gate, AffectedTaskIDs: affected}
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrGateNotFound):
			return ResolveGateOutput{}, apperrors.New(apperrors.KindNotFound, "ORCH_GATE_NOT_FOUND", "decision gate not found", err)
		case errors.Is(err, ErrGateNotPending), errors.Is(err, domain.ErrGateAlreadyResolved):
			return ResolveGateOutput{}, apperrors.New(apperrors.KindFailedPrecondition, "ORCH_GATE_ALREADY_RESOLVED", "decision gate is already resolved", err)
		default:
			return ResolveGateOutput{}, apperrors.New(apperrors.KindInternal, "ORCH_RESOLVE_GATE_FAILED", "failed to resolve decision gate", err)
		}
	}
	return out, nil
}
