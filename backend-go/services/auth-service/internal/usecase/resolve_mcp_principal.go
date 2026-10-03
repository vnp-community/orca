package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// Reasons returned in ResolveMcpPrincipal when a token is no longer usable.
const (
	McpInactiveRevoked      = "revoked"
	McpInactiveExpired      = "expired"
	McpInactiveUserInactive = "user_inactive"
	McpInactiveClientBlock  = "client_blocked"
	McpInactiveGrantRevoked = "grant_revoked"
	McpInactiveUnknown      = "unknown_token"
	// McpInactiveSuspended: the tenant's kill switch suspends its PATs (reversible).
	McpInactiveSuspended = "suspended"
)

type ResolveMcpPrincipalInput struct {
	JTI, UserID, TenantID, TokenUse, FamilyID, GrantID, ClientID string
}

type ResolveMcpPrincipalOutput struct {
	Active         bool
	InactiveReason string
	Role           string
}

// ResolveMcpPrincipal answers "is this already-signature-verified MCP token
// still usable, and what role does its user hold right now". It reads only
// auth-service's own tables. The role is live (never from the token), so
// demotion and deactivation take effect without waiting for token expiry.
type ResolveMcpPrincipal struct {
	users  UserRepository
	tokens McpTokenRepository
	oauth  OAuthRepository
	audit  AuditRepository
	clock  Clock
	// suspended is optional; nil means PATs are never suspended.
	suspended McpPatSuspensionRepository
}

// WithPatSuspension makes PAT resolution honour the tenant kill-switch flag.
func (uc *ResolveMcpPrincipal) WithPatSuspension(r McpPatSuspensionRepository) *ResolveMcpPrincipal {
	uc.suspended = r
	return uc
}

func NewResolveMcpPrincipal(users UserRepository, tokens McpTokenRepository, oauth OAuthRepository, audit AuditRepository, clock Clock) *ResolveMcpPrincipal {
	return &ResolveMcpPrincipal{users: users, tokens: tokens, oauth: oauth, audit: audit, clock: clock}
}

func inactive(reason string) ResolveMcpPrincipalOutput {
	return ResolveMcpPrincipalOutput{Active: false, InactiveReason: reason}
}

func (uc *ResolveMcpPrincipal) Execute(ctx context.Context, in ResolveMcpPrincipalInput) (ResolveMcpPrincipalOutput, error) {
	if in.UserID == "" || in.TenantID == "" {
		return inactive(McpInactiveUnknown), nil
	}
	user, err := uc.users.GetUserByID(ctx, in.UserID)
	if errors.Is(err, ErrUserNotFound) {
		return inactive(McpInactiveUserInactive), nil
	}
	if err != nil {
		return ResolveMcpPrincipalOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_MCP_RESOLVE_FAILED", "failed to look up user", err)
	}
	if !user.IsActive || user.TenantID != in.TenantID {
		return inactive(McpInactiveUserInactive), nil
	}

	var out ResolveMcpPrincipalOutput
	switch in.TokenUse {
	case mcpTokenUsePAT:
		out, err = uc.checkPAT(ctx, in)
	case mcpTokenUseOAuth:
		out, err = uc.checkOAuth(ctx, in)
	default:
		return inactive(McpInactiveUnknown), nil
	}
	if err != nil || !out.Active {
		return out, err
	}
	out.Role = string(user.Role)
	return out, nil
}

func (uc *ResolveMcpPrincipal) checkPAT(ctx context.Context, in ResolveMcpPrincipalInput) (ResolveMcpPrincipalOutput, error) {
	t, err := uc.tokens.GetMcpToken(ctx, in.TenantID, in.JTI)
	if errors.Is(err, ErrMcpTokenNotFound) || (err == nil && t.UserID != in.UserID) {
		return inactive(McpInactiveUnknown), nil
	}
	if err != nil {
		return ResolveMcpPrincipalOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_MCP_RESOLVE_FAILED", "failed to look up token", err)
	}
	now := uc.clock.Now()
	switch {
	case t.RevokedAt != nil:
		return inactive(McpInactiveRevoked), nil
	case !now.Before(t.ExpiresAt):
		return inactive(McpInactiveExpired), nil
	}
	// Checked before usage bookkeeping so a suspended token leaves no trace of use.
	if uc.suspended != nil {
		sus, err := uc.suspended.IsMcpPatSuspended(ctx, in.TenantID)
		if err != nil {
			return ResolveMcpPrincipalOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_MCP_RESOLVE_FAILED", "failed to check token suspension", err)
		}
		if sus {
			return inactive(McpInactiveSuspended), nil
		}
	}
	// Usage bookkeeping is best effort: it must never turn a valid token away.
	if first, err := uc.tokens.MarkMcpTokenFirstUsed(ctx, t.TenantID, t.JTI, now); err == nil && first {
		ip, _ := tenant.ClientIP(ctx)
		if e, err := domain.NewAuditEntry(uuid.NewString(), t.TenantID, t.UserID, "mcp_token.first_used", "", "mcp_token", t.JTI,
			map[string]any{"client_ip": ip}, domain.OutcomeAllowed, ip, now); err == nil {
			_ = uc.audit.Append(ctx, e)
		}
	}
	_ = uc.tokens.TouchMcpTokenLastUsed(ctx, t.TenantID, t.JTI, now, now.Add(-mcpLastUsedRefreshWindow*time.Minute))
	return ResolveMcpPrincipalOutput{Active: true}, nil
}

func (uc *ResolveMcpPrincipal) checkOAuth(ctx context.Context, in ResolveMcpPrincipalInput) (ResolveMcpPrincipalOutput, error) {
	if in.FamilyID == "" {
		return inactive(McpInactiveUnknown), nil
	}
	f, err := uc.oauth.GetOAuthFamily(ctx, in.FamilyID)
	if errors.Is(err, ErrOAuthFamilyNotFound) || (err == nil && (f.TenantID != in.TenantID || f.UserID != in.UserID)) {
		return inactive(McpInactiveUnknown), nil
	}
	if err != nil {
		return ResolveMcpPrincipalOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_MCP_RESOLVE_FAILED", "failed to look up token family", err)
	}
	if f.RevokedAt != nil {
		return inactive(McpInactiveRevoked), nil
	}
	if revoked, err := uc.oauth.IsOAuthGrantRevoked(ctx, f.TenantID, f.GrantID); err != nil {
		return ResolveMcpPrincipalOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_MCP_RESOLVE_FAILED", "failed to check grant", err)
	} else if revoked {
		return inactive(McpInactiveGrantRevoked), nil
	}
	st, err := uc.oauth.GetOAuthClientTenantStatus(ctx, f.TenantID, f.ClientID)
	if err != nil && !errors.Is(err, ErrOAuthClientStatusNotFound) {
		return ResolveMcpPrincipalOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_MCP_RESOLVE_FAILED", "failed to check client", err)
	}
	if err == nil && st.Status == domain.OAuthClientBlocked {
		return inactive(McpInactiveClientBlock), nil
	}
	return ResolveMcpPrincipalOutput{Active: true}, nil
}
