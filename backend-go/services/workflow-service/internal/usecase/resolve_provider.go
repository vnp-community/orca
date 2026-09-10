package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// ResolvedProvider is what ProviderResolver hands back to a step
// executor — Model is only ever populated from an explicit pin (see
// TASK-WF-002-02's Context: ai-provider-service's ResolveProvider RPC
// resolves an account, not a model).
type ResolvedProvider struct {
	AccountID string
	Model     string
}

// ProviderResolver implements the priority BE-SOL-002/workflow-service.md
// §7 specify: an explicit, still-active step.config.provider.accountId pin
// beats ai-provider-service's own priority-chain resolution (user > project
// > server). A pin that fails validation (inactive account) is a hard
// error, never a silent fall-back — a step author who pinned a specific
// account almost certainly wants to know that account stopped working, not
// have a different one silently substituted.
type ProviderResolver struct {
	aiProvider AIProviderClient
}

func NewProviderResolver(aiProvider AIProviderClient) *ProviderResolver {
	return &ProviderResolver{aiProvider: aiProvider}
}

func (r *ProviderResolver) Resolve(ctx context.Context, cfg domain.AgentStepConfig, projectID, triggeredBy string) (ResolvedProvider, error) {
	if cfg.Provider != nil && cfg.Provider.AccountID != "" {
		status, err := r.aiProvider.GetAccountStatus(ctx, cfg.Provider.AccountID)
		if err != nil {
			return ResolvedProvider{}, err
		}
		if status != "active" {
			return ResolvedProvider{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_PROVIDER_PIN_INACTIVE", "pinned provider account is not active", nil)
		}
		return ResolvedProvider{AccountID: cfg.Provider.AccountID, Model: cfg.Provider.Model}, nil
	}
	accountID, err := r.aiProvider.ResolveForProject(ctx, triggeredBy, projectID)
	if err != nil {
		return ResolvedProvider{}, err
	}
	return ResolvedProvider{AccountID: accountID}, nil
}
