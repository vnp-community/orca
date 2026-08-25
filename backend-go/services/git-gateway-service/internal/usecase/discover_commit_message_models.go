package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type DiscoverCommitMessageModelsInput struct {
	TenantID string
	UserID   string
}

type ModelInfo struct {
	ProviderType string
	AccountID    string
	Status       string
}

// DiscoverCommitMessageModels is not a worktree-dispatch operation — it
// answers "what AI account would be used" by calling ai-provider-service
// directly, no ConnectionResolver/GitExecutor involved. Reports the one
// account ResolveProvider's scope-cascade would pick (0 or 1 entries), not
// a full multi-account list — ai-provider-service has no account-listing
// RPC today. See TASK-211's Context section for the follow-up this defers.
type DiscoverCommitMessageModels struct {
	aiProviders AIProviderResolver
}

func NewDiscoverCommitMessageModels(aiProviders AIProviderResolver) *DiscoverCommitMessageModels {
	return &DiscoverCommitMessageModels{aiProviders: aiProviders}
}

func (uc *DiscoverCommitMessageModels) Execute(ctx context.Context, in DiscoverCommitMessageModelsInput) ([]ModelInfo, error) {
	if in.TenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_TENANT_ID", "tenant_id is required", nil)
	}
	providerType, accountID, status, err := uc.aiProviders.ResolveProvider(ctx, in.TenantID, in.UserID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_DISCOVER_MODELS_FAILED", "failed to resolve AI provider account", err)
	}
	if accountID == "" {
		return []ModelInfo{}, nil
	}
	return []ModelInfo{{ProviderType: providerType, AccountID: accountID, Status: status}}, nil
}
