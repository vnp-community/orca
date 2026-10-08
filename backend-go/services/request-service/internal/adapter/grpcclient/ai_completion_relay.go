package grpcclient

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const defaultAICompletionTimeout = 120 * time.Second

// AICompletionRelay runs ai.complete for solution generation. It shares the relay call and the
// connection fallback with the classifier (BUG-025: connection id = project id rarely resolves).
type AICompletionRelay struct {
	infra     infrafleetv1.InfraFleetServiceClient
	resolver  connectionResolver
	providers aiproviderv1.AiProviderServiceClient // optional: picks the account for ai.complete
	timeout   time.Duration
}

var _ usecase.ProjectAICompleter = (*AICompletionRelay)(nil)

func NewAICompletionRelay(infra infrafleetv1.InfraFleetServiceClient, resolver connectionResolver, providers aiproviderv1.AiProviderServiceClient, timeout time.Duration) *AICompletionRelay {
	if timeout <= 0 {
		timeout = defaultAICompletionTimeout
	}
	return &AICompletionRelay{infra: infra, resolver: resolver, providers: providers, timeout: timeout}
}

func (c *AICompletionRelay) Complete(ctx context.Context, projectID, prompt string) (string, error) {
	conn, err := c.resolver.ResolveForProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	return relayAIComplete(ctx, c.infra, conn, c.accountID(ctx, projectID), prompt, c.timeout)
}

// accountID is best effort: without it the dev server falls back to its own default account.
func (c *AICompletionRelay) accountID(ctx context.Context, projectID string) string {
	if c.providers == nil {
		return ""
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ""
	}
	userID, _ := tenant.UserID(ctx)
	pctx, err := withIdentityMetadata(ctx)
	if err != nil {
		return ""
	}
	resp, err := c.providers.ResolveProvider(pctx, &aiproviderv1.ResolveProviderRequest{TenantId: tenantID, UserId: userID, ProjectId: projectID})
	if err != nil {
		return ""
	}
	return resp.GetAccount().GetId()
}

// AnalysisConnections adapts the AI connection resolver to the analysis port, keeping repo path and worktree id.
type AnalysisConnections struct {
	resolver connectionResolver
}

var _ usecase.AnalysisConnectionResolver = (*AnalysisConnections)(nil)

func NewAnalysisConnections(resolver connectionResolver) *AnalysisConnections {
	return &AnalysisConnections{resolver: resolver}
}

func (a *AnalysisConnections) ResolveForProject(ctx context.Context, projectID string) (usecase.AnalysisConnection, error) {
	c, err := a.resolver.ResolveForProject(ctx, projectID)
	if err != nil {
		return usecase.AnalysisConnection{}, err
	}
	return usecase.AnalysisConnection{ConnectionID: c.ConnectionID, DevServerID: c.DevServerID, RepoPath: c.RepoPath, WorktreeID: c.WorktreeID}, nil
}
