package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type TestConnectionInput struct {
	AccountID string
}

// TestConnection relays a live, lightweight provider API call to whichever
// dev server holds this account's pushed ciphertext — see TASK-028's
// context for why ResolveCredential cannot be used here. The plaintext key
// never crosses into this service's memory at any point.
type TestConnection struct {
	repo  ProviderAccountRepository
	infra InfraFleetClient
}

func NewTestConnection(repo ProviderAccountRepository, infra InfraFleetClient) *TestConnection {
	return &TestConnection{repo: repo, infra: infra}
}

func (uc *TestConnection) Execute(ctx context.Context, in TestConnectionInput) (ConnectionTestResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ConnectionTestResult{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	account, err := uc.repo.Get(ctx, tenantID, in.AccountID)
	if err != nil {
		return ConnectionTestResult{}, err
	}
	if account.DevServerID == "" {
		return ConnectionTestResult{}, apperrors.New(apperrors.KindFailedPrecondition, "AIPROVIDER_NO_DEV_SERVER", "account has no dev server bound yet — push a credential first", nil)
	}
	result, err := verifyConnection(ctx, uc.infra, account.DevServerID, account.CredentialRef, account.ProviderType)
	if err != nil {
		return ConnectionTestResult{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_TEST_CONNECTION_FAILED", "failed to relay connection test to dev server agent", err)
	}
	return result, nil
}

// parseConnectionTestResult maps the agent's generic map[string]any result
// onto ConnectionTestResult, defensively (the agent method doesn't exist
// yet, so this is best-effort against the documented future shape).
func parseConnectionTestResult(result map[string]any) ConnectionTestResult {
	out := ConnectionTestResult{}
	if v, ok := result["success"].(bool); ok {
		out.Success = v
	}
	if v, ok := result["message"].(string); ok {
		out.Message = v
	}
	return out
}
