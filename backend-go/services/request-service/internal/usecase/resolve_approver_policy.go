package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ResolveApproverPolicy picks the most specific tenant policy for the Request's real project, type, size and urgency,
// falling back to the built-in default when none matches.
type ResolveApproverPolicy struct {
	Repo ApprovalPolicyRepository
}

func policyContextFor(req domain.Request) domain.PolicyContext {
	urgency := string(req.Urgency)
	if urgency == "" {
		urgency = string(domain.UrgencyNormal)
	}
	return domain.PolicyContext{ProjectID: req.ProjectID, RequestType: string(req.Type), Size: string(req.Size), Urgency: urgency}
}

func (uc *ResolveApproverPolicy) Resolve(ctx context.Context, req domain.Request, subjectType domain.SubjectType) (domain.ApprovalPolicy, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ApprovalPolicy{}, err
	}
	pCtx := policyContextFor(req)
	candidates, err := uc.Repo.ListEnabledCandidates(ctx, tenantID, subjectType, pCtx.ProjectID, pCtx.RequestType, pCtx.Size, pCtx.Urgency)
	if err != nil {
		return domain.ApprovalPolicy{}, err
	}
	if selected, ok := domain.SelectPolicy(candidates, pCtx); ok {
		return selected, nil
	}
	return domain.DefaultPolicy(subjectType, pCtx), nil
}
