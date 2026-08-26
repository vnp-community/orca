package grpcclient

import (
	"context"
	"fmt"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
)

// AIProviderContextResolver implements usecase.AIProviderContextResolver by
// calling ai-provider-service's ResolveProvider RPC — the existing
// user-scope -> project-scope priority cascade
// (proto/orca/aiprovider/v1/aiprovider.proto) already returns exactly the
// non-secret reference this port needs (never a plaintext key, per that
// proto's own doc comment). ProjectID is left empty: AIDecomposeInput
// carries no project_id of its own — ResolveProvider's cascade still
// resolves a user- or tenant-scope account without it.
type AIProviderContextResolver struct {
	client aiproviderv1.AiProviderServiceClient
}

func NewAIProviderContextResolver(client aiproviderv1.AiProviderServiceClient) *AIProviderContextResolver {
	return &AIProviderContextResolver{client: client}
}

func (r *AIProviderContextResolver) ResolveContext(ctx context.Context, tenantID, userID string) (string, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return "", err
	}
	resp, err := r.client.ResolveProvider(ctx, &aiproviderv1.ResolveProviderRequest{
		TenantId: tenantID,
		UserId:   userID,
	})
	if err != nil {
		return "", fmt.Errorf("grpcclient: ResolveProvider(tenant=%q, user=%q): %w", tenantID, userID, err)
	}
	// CredentialRef is a credential-broker-service metadata id, never a
	// secret value (see ProviderAccount's own doc comment) — safe to fold
	// into the AI-relay prompt as trace context.
	return resp.GetAccount().GetCredentialRef(), nil
}
