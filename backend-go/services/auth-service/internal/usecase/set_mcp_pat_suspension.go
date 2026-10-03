package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

const maxMcpSuspensionReason = 500

// SetMcpPatSuspension suspends (or restores) every PAT of the caller's tenant.
// Only mcp-service calls it, from the kill-switch cleanup; a PAT is never
// revoked here, so switching the kill switch off brings them back.
type SetMcpPatSuspension struct {
	repo  McpPatSuspensionRepository
	audit AuditRepository
	clock Clock
}

func NewSetMcpPatSuspension(repo McpPatSuspensionRepository, audit AuditRepository, clock Clock) *SetMcpPatSuspension {
	return &SetMcpPatSuspension{repo: repo, audit: audit, clock: clock}
}

func (uc *SetMcpPatSuspension) Execute(ctx context.Context, tenantID, actorID string, suspended bool, reason string) error {
	if tenantID == "" {
		return errMcp(apperrors.KindUnauthenticated, CodeMcpTokenNotFound, "tenant is required")
	}
	if r := []rune(reason); len(r) > maxMcpSuspensionReason {
		reason = string(r[:maxMcpSuspensionReason])
	}
	now := uc.clock.Now()
	if err := uc.repo.SetMcpPatSuspension(ctx, tenantID, suspended, reason, now); err != nil {
		return apperrors.New(apperrors.KindInternal, "AUTH_MCP_SUSPEND_FAILED", "failed to update token suspension", err)
	}
	action := "mcp_token.suspended"
	if !suspended {
		action = "mcp_token.resumed"
	}
	if e, err := domain.NewAuditEntry(uuid.NewString(), tenantID, actorID, action, "", "mcp_tenant", tenantID,
		map[string]any{"reason": reason}, domain.OutcomeAllowed, "", now); err == nil {
		_ = uc.audit.Append(ctx, e)
	}
	return nil
}
