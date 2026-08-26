// Package grpcclient — this file adds a second gRPC client dependency
// (ai-provider-service) distinct from infra-fleet-service's Relay client
// RelayExecutor already wraps — DiscoverCommitMessageModels resolves
// account metadata directly, it does not go through the execution-plane
// relay. See TASK-211.
package grpcclient

import (
	"context"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
)

// AIProviderResolver implements usecase.AIProviderResolver by calling
// ai-provider-service's ResolveProvider RPC directly.
type AIProviderResolver struct {
	client aiproviderv1.AiProviderServiceClient
}

func NewAIProviderResolver(client aiproviderv1.AiProviderServiceClient) *AIProviderResolver {
	return &AIProviderResolver{client: client}
}

func (a *AIProviderResolver) ResolveProvider(ctx context.Context, tenantID, userID string) (providerType, accountID, status string, err error) {
	resp, err := a.client.ResolveProvider(ctx, &aiproviderv1.ResolveProviderRequest{
		TenantId: tenantID, UserId: userID,
	})
	if err != nil {
		return "", "", "", err
	}
	account := resp.GetAccount()
	if account == nil {
		return "", "", "", nil
	}
	return account.GetType().String(), account.GetId(), account.GetStatus(), nil
}
