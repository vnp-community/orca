package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

const oauthCodeBytes = 32

type OAuthIssueAuthCodeInput struct {
	TenantID      string // from gRPC metadata: the consenting user's tenant
	UserID        string // from gRPC metadata: the consenting user
	ClientID      string
	RedirectURI   string
	Scopes        []string
	CodeChallenge string
	Resource      string
	GrantID       string
}

type OAuthIssueAuthCodeOutput struct {
	Code      string // returned once; only its hash is stored
	ExpiresAt time.Time
}

// OAuthIssueAuthCode mints a single-use authorization code after consent.
// Only mcp-service may call it (the guard lives in the gRPC adapter): that
// is what guarantees no code exists without a recorded grant. The user's
// role is read from the users table, never trusted from metadata, and the
// consented scopes are capped by it.
type OAuthIssueAuthCode struct {
	repo  OAuthRepository
	users UserRepository
	audit oauthAuditor
	clock Clock
	cfg   OAuthConfig
}

func NewOAuthIssueAuthCode(repo OAuthRepository, users UserRepository, audit AuditRepository, clock Clock, cfg OAuthConfig) *OAuthIssueAuthCode {
	return &OAuthIssueAuthCode{repo: repo, users: users, audit: oauthAuditor{audit: audit, clock: clock}, clock: clock, cfg: cfg.WithDefaults()}
}

func (uc *OAuthIssueAuthCode) Execute(ctx context.Context, in OAuthIssueAuthCodeInput) (OAuthIssueAuthCodeOutput, error) {
	if in.TenantID == "" || in.UserID == "" {
		return OAuthIssueAuthCodeOutput{}, oauthErr(apperrors.KindUnauthenticated, CodeOAuthAccessDenied, "authenticated user context is required", nil)
	}
	if _, err := uuid.Parse(in.GrantID); err != nil {
		return OAuthIssueAuthCodeOutput{}, errOAuthInvalidRequest("grant_id must be a UUID")
	}
	if err := domain.ValidatePKCEChallenge(in.CodeChallenge, domain.PKCEMethodS256); err != nil {
		return OAuthIssueAuthCodeOutput{}, errOAuthInvalidRequest("PKCE with code_challenge_method=S256 is required")
	}
	if in.Resource != uc.cfg.ResourceURL {
		return OAuthIssueAuthCodeOutput{}, oauthErr(apperrors.KindInvalidArgument, CodeOAuthInvalidTarget, "resource must be the MCP resource URL", nil)
	}
	scopes, err := domain.NormalizeOAuthScopes(in.Scopes)
	if err != nil || len(scopes) == 0 {
		return OAuthIssueAuthCodeOutput{}, oauthErr(apperrors.KindInvalidArgument, CodeOAuthInvalidScope, "a non-empty set of known scopes is required", nil)
	}

	user, err := uc.users.GetUserByID(ctx, in.UserID)
	if errors.Is(err, ErrUserNotFound) || (err == nil && (!user.IsActive || user.TenantID != in.TenantID)) {
		return OAuthIssueAuthCodeOutput{}, oauthErr(apperrors.KindPermissionDenied, CodeOAuthAccessDenied, "user is not allowed to authorize", nil)
	}
	if err != nil {
		return OAuthIssueAuthCodeOutput{}, errOAuthInternal("failed to look up user", err)
	}
	if !domain.IsScopeSubset(scopes, domain.OAuthScopeCeilingForRole(user.Role)) {
		return OAuthIssueAuthCodeOutput{}, oauthErr(apperrors.KindInvalidArgument, CodeOAuthInvalidScope, "scope exceeds what the user's role allows", nil)
	}

	client, err := lookupOAuthClient(ctx, uc.repo, in.ClientID)
	if err != nil {
		return OAuthIssueAuthCodeOutput{}, err
	}
	if !domain.MatchRedirectURI(client.RedirectURIs, in.RedirectURI) {
		return OAuthIssueAuthCodeOutput{}, oauthErr(apperrors.KindInvalidArgument, CodeOAuthInvalidRedirectURI, "redirect_uri does not match a registered URI", nil)
	}
	if err := uc.requireClientAllowed(ctx, in.TenantID, in.ClientID); err != nil {
		return OAuthIssueAuthCodeOutput{}, err
	}
	if revoked, err := uc.repo.IsOAuthGrantRevoked(ctx, in.TenantID, in.GrantID); err != nil {
		return OAuthIssueAuthCodeOutput{}, errOAuthInternal("failed to check grant", err)
	} else if revoked {
		return OAuthIssueAuthCodeOutput{}, oauthErr(apperrors.KindPermissionDenied, CodeOAuthAccessDenied, "grant has been revoked", nil)
	}

	rawCode, err := generateRandomToken(oauthCodeBytes)
	if err != nil {
		return OAuthIssueAuthCodeOutput{}, errOAuthInternal("failed to generate authorization code", err)
	}
	now := uc.clock.Now()
	family := domain.OAuthTokenFamily{
		FamilyID: uuid.NewString(), TenantID: in.TenantID, UserID: in.UserID, ClientID: in.ClientID,
		GrantID: in.GrantID, Scope: domain.FormatOAuthScopes(scopes), Resource: in.Resource, CreatedAt: now,
	}
	code := domain.OAuthAuthCode{
		CodeHash: hashToken(rawCode), FamilyID: family.FamilyID, TenantID: in.TenantID,
		RedirectURI: in.RedirectURI, CodeChallenge: in.CodeChallenge, ExpiresAt: now.Add(uc.cfg.AuthCodeTTL),
	}
	if err := uc.repo.CreateOAuthFamilyWithCode(ctx, family, code); err != nil {
		return OAuthIssueAuthCodeOutput{}, errOAuthInternal("failed to store authorization code", err)
	}
	uc.audit.record(ctx, in.TenantID, in.UserID, "oauth.code_issued", "oauth_family", family.FamilyID,
		map[string]any{"client_id": in.ClientID, "grant_id": in.GrantID, "scope": family.Scope}, domain.OutcomeAllowed)
	return OAuthIssueAuthCodeOutput{Code: rawCode, ExpiresAt: code.ExpiresAt}, nil
}

func (uc *OAuthIssueAuthCode) requireClientAllowed(ctx context.Context, tenantID, clientID string) error {
	return requireOAuthClientAllowed(ctx, uc.repo, tenantID, clientID)
}

// requireOAuthClientAllowed fails closed: a missing status row, pending and
// blocked all deny.
func requireOAuthClientAllowed(ctx context.Context, repo OAuthRepository, tenantID, clientID string) error {
	st, err := repo.GetOAuthClientTenantStatus(ctx, tenantID, clientID)
	if errors.Is(err, ErrOAuthClientStatusNotFound) {
		return oauthErr(apperrors.KindPermissionDenied, CodeOAuthUnauthorizedClient, "client is not allowed in this tenant", nil)
	}
	if err != nil {
		return errOAuthInternal("failed to read client status", err)
	}
	if st.Status != domain.OAuthClientAllowed {
		return oauthErr(apperrors.KindPermissionDenied, CodeOAuthUnauthorizedClient, "client is not allowed in this tenant", nil)
	}
	return nil
}
