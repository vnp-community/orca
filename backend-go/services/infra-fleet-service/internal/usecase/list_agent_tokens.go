package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// ListAgentTokens returns the active token summaries for a DevServer —
// never the plaintext or the hash/credential_ref_id.
type ListAgentTokens struct {
	repo AgentTokenRepository
}

func NewListAgentTokens(repo AgentTokenRepository) *ListAgentTokens {
	return &ListAgentTokens{repo: repo}
}

func (uc *ListAgentTokens) Execute(ctx context.Context, devServerID string) ([]AgentTokenSummary, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	tokens, err := uc.repo.ListActive(ctx, tenantID, devServerID)
	if err != nil {
		return nil, err
	}
	out := make([]AgentTokenSummary, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, AgentTokenSummary{ID: t.ID, Name: t.Name, CreatedAt: t.CreatedAt, LastUsedAt: t.LastUsedAt})
	}
	return out, nil
}

// AgentTokenSummary is the never-plaintext, never-secret-ref view of an
// AgentToken this usecase and the gRPC layer both use.
type AgentTokenSummary struct {
	ID         string
	Name       string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}
