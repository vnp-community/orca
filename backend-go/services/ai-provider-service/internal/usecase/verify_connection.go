package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// verifyConnection is shared by CreateAccount (test-before-save gate,
// TASK-AIP-01-06) and TestConnection (on-demand check) — one relay call, one
// place that knows the agent method name and result-parsing shape. See
// TASK-AIP-SHARED-01 for the agent-side handler this targets.
func verifyConnection(ctx context.Context, infra InfraFleetClient, devServerID, credentialRef string, providerType domain.ProviderType) (ConnectionTestResult, error) {
	result, err := infra.Relay(ctx, devServerID, "ai.testProviderConnection", map[string]any{
		"credentialRef": credentialRef,
		"providerType":  string(providerType),
	})
	if err != nil {
		return ConnectionTestResult{}, err
	}
	return parseConnectionTestResult(result), nil
}
