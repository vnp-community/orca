// Package grpcclient — infra-fleet-service's outbound client to
// ai-provider-service. NEW dependency edge (infra --> aiprov) on
// 02-microservices-decomposition.md's graph — see TASK-AG-04-03.
package grpcclient

import (
	"context"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
)

// AIProviderResolver implements usecase.AIProviderResolverClient by calling
// ai-provider-service's ResolveProvider RPC directly.
type AIProviderResolver struct {
	client aiproviderv1.AiProviderServiceClient
}

func NewAIProviderResolver(client aiproviderv1.AiProviderServiceClient) *AIProviderResolver {
	return &AIProviderResolver{client: client}
}

func (a *AIProviderResolver) ResolveProvider(ctx context.Context, tenantID, userID, projectID, excludeAccountID string) (providerType, accountID, status string, err error) {
	resp, err := a.client.ResolveProvider(ctx, &aiproviderv1.ResolveProviderRequest{
		TenantId: tenantID, UserId: userID, ProjectId: projectID, ExcludeAccountId: excludeAccountID,
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
