// Package providerresolver implements usecase.ProviderResolver — deciding
// which ai-provider-service account an Agent step should use. BUG-WF-02
// found AI provider selection never called at all: ai-provider-service's
// ResolveProvider cascade and its pin-beats-cascade rule
// (workflow-service.md §7) were unimplemented.
package providerresolver

import (
	"context"
	"fmt"
	"time"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// resolveTimeout bounds each Resolve call — same intra-cluster default as
// internal/adapter/serverresolver.
const resolveTimeout = 5 * time.Second

// activeStatus is the only ProviderAccount.Status a pin is allowed to use —
// "rotating"/"revoked" accounts fail closed rather than silently falling
// back to the cascade (workflow-service.md §7's "(validated active)").
const activeStatus = "active"

type resolver struct {
	aiprovider aiproviderv1.AiProviderServiceClient
}

// New builds a usecase.ProviderResolver against an already-dialed
// ai-provider-service client — see cmd/server/main.go for the real dial,
// and this package's tests for a fake.
func New(aiprovider aiproviderv1.AiProviderServiceClient) usecase.ProviderResolver {
	return &resolver{aiprovider: aiprovider}
}

func (r *resolver) Resolve(ctx context.Context, tenantID, userID, projectID string, pin *domain.ProviderPin) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	if pin != nil && pin.AccountID != "" {
		return r.resolvePinned(ctx, tenantID, pin.AccountID)
	}

	// No pin — delegate to ai-provider-service's own priority cascade.
	resp, err := r.aiprovider.ResolveProvider(ctx, &aiproviderv1.ResolveProviderRequest{
		TenantId: tenantID, UserId: userID, ProjectId: projectID,
	})
	if err != nil {
		return "", fmt.Errorf("providerresolver: resolve provider cascade: %w", err)
	}
	accountID := resp.GetAccount().GetId()
	if accountID == "" {
		return "", fmt.Errorf("providerresolver: no provider account resolved for tenant %s", tenantID)
	}
	return accountID, nil
}

// resolvePinned validates an explicit step.config.provider.accountId pin —
// workflow-service.md §7: an explicit pin beats ai-provider-service's
// priority-chain resolution, but only if that account still exists and is
// active; an inactive/unknown pin errors rather than silently falling back
// to the cascade (a caller who pinned an account wants THAT account, or an
// explicit failure, never a surprise substitution).
func (r *resolver) resolvePinned(ctx context.Context, tenantID, accountID string) (string, error) {
	accounts, err := r.aiprovider.ListAccounts(ctx, &aiproviderv1.ListAccountsRequest{TenantId: tenantID})
	if err != nil {
		return "", fmt.Errorf("providerresolver: list accounts to validate pin %s: %w", accountID, err)
	}
	for _, a := range accounts.GetAccounts() {
		if a.GetId() != accountID {
			continue
		}
		if a.GetStatus() != activeStatus {
			return "", fmt.Errorf("providerresolver: pinned account %s is not active (status=%s)", accountID, a.GetStatus())
		}
		return a.GetId(), nil
	}
	return "", fmt.Errorf("providerresolver: pinned account %s not found", accountID)
}
