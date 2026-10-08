package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RequestApprovalFromAPI is the RequestApproval RPC: callers may open a pre_deploy gate, or reopen a gate whose
// previous approval closed (rejected, expired, cancelled). Every other gate is opened by its owning feature.
type RequestApprovalFromAPI struct {
	Open *OpenApproval
	Repo ApprovalRepository
}

func (uc *RequestApprovalFromAPI) Execute(ctx context.Context, in OpenApprovalInput) (*domain.Approval, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if userID, _ := tenant.UserID(ctx); userID == "" {
		return nil, domain.ErrNoUser
	}
	if tenant.ActorType(ctx) == tenant.ActorAgent {
		return nil, domain.ErrAgentForbidden
	}
	in.RequestedBy = "" // the RPC caller is always the requester
	if in.SubjectType != domain.SubjectPreDeploy {
		closed, err := uc.hasClosedGate(ctx, tenantID, in)
		if err != nil {
			return nil, err
		}
		if !closed {
			return nil, domain.ErrApprovalSubjectTypeNotAllowed
		}
	}
	return uc.Open.Execute(ctx, in)
}

func (uc *RequestApprovalFromAPI) hasClosedGate(ctx context.Context, tenantID string, in OpenApprovalInput) (bool, error) {
	if in.SubjectID == "" || in.RequestID == "" || !in.SubjectType.Valid() {
		return false, nil
	}
	token := ""
	for page := 0; page < 10; page++ {
		list, next, err := uc.Repo.List(ctx, tenantID, ApprovalListFilter{RequestID: in.RequestID, SubjectType: in.SubjectType, PageSize: maxApprovalPageSize, PageToken: token})
		if err != nil {
			return false, err
		}
		for _, a := range list {
			if a.SubjectID == in.SubjectID && a.Status.Terminal() && a.Status != domain.ApprovalStatusApproved {
				return true, nil
			}
		}
		if next == "" {
			break
		}
		token = next
	}
	return false, nil
}
