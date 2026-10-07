package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ResolveApproverPolicy struct {
	Repo ApprovalPolicyRepository
}

func (uc *ResolveApproverPolicy) Resolve(ctx context.Context, req domain.Request, subjectType domain.SubjectType) (domain.ApprovalPolicy, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ApprovalPolicy{}, err
	}

	projectIDStr := ""
	if req.ProjectID != "" {
		projectIDStr = req.ProjectID
	}
	reqTypeStr := string(req.Type)
	sizeStr := "S" // Stub size
	urgencyStr := "normal" // Stub urgency

	candidates, err := uc.Repo.ListEnabledCandidates(ctx, tenantID, subjectType, projectIDStr, reqTypeStr, sizeStr, urgencyStr)
	if err != nil {
		return domain.ApprovalPolicy{}, err
	}

	pCtx := domain.PolicyContext{
		ProjectID:   projectIDStr,
		RequestType: reqTypeStr,
		Size:        sizeStr,
		Urgency:     urgencyStr,
	}

	var p domain.ApprovalPolicy
	if selected, ok := domain.SelectPolicy(candidates, pCtx); ok {
		p = selected
	} else {
		p = domain.DefaultPolicy(subjectType, pCtx)
	}

	return p, nil
}
