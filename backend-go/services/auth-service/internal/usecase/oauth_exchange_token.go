package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

const (
	oauthRefreshTokenBytes = 32
	oauthAccessJTIBytes    = 32
	oauthTokenUseOAuth     = "mcp_oauth"
)

type OAuthExchangeTokenInput struct {
	GrantType    string
	Code         string
	RedirectURI  string
	CodeVerifier string
	ClientID     string
	RefreshToken string
	Resource     string
	Scope        string
}

type OAuthTokenOutput struct {
	AccessToken  string
	ExpiresIn    int // seconds
	RefreshToken string
	Scope        string
}

// OAuthExchangeToken is the token endpoint: authorization_code (PKCE S256
// verified) and refresh_token (rotating, with reuse detection). It is
// anonymous by necessity; the tenant comes from the token family row.
type OAuthExchangeToken struct {
	repo   OAuthRepository
	users  UserRepository
	signer TokenSigner
	audit  oauthAuditor
	clock  Clock
	cfg    OAuthConfig
}

func NewOAuthExchangeToken(repo OAuthRepository, users UserRepository, signer TokenSigner, audit AuditRepository, clock Clock, cfg OAuthConfig) *OAuthExchangeToken {
	return &OAuthExchangeToken{repo: repo, users: users, signer: signer, audit: oauthAuditor{audit: audit, clock: clock}, clock: clock, cfg: cfg.WithDefaults()}
}

func (uc *OAuthExchangeToken) Execute(ctx context.Context, in OAuthExchangeTokenInput) (OAuthTokenOutput, error) {
	switch in.GrantType {
	case oauthGrantAuthorizationCode:
		return uc.exchangeCode(ctx, in)
	case oauthGrantRefreshToken:
		return uc.rotateRefresh(ctx, in)
	default:
		return OAuthTokenOutput{}, oauthErr(apperrors.KindInvalidArgument, CodeOAuthUnsupportedGrantType, "grant_type must be authorization_code or refresh_token", nil)
	}
}

func (uc *OAuthExchangeToken) exchangeCode(ctx context.Context, in OAuthExchangeTokenInput) (OAuthTokenOutput, error) {
	if in.Code == "" || in.RedirectURI == "" || in.CodeVerifier == "" || in.ClientID == "" {
		return OAuthTokenOutput{}, errOAuthInvalidRequest("code, redirect_uri, code_verifier and client_id are required")
	}
	now := uc.clock.Now()
	codeHash := hashToken(in.Code)
	code, err := uc.repo.GetOAuthAuthCode(ctx, codeHash)
	if errors.Is(err, ErrOAuthCodeNotFound) {
		return OAuthTokenOutput{}, errOAuthInvalidGrant("authorization code is invalid")
	}
	if err != nil {
		return OAuthTokenOutput{}, errOAuthInternal("failed to look up authorization code", err)
	}
	family, err := uc.repo.GetOAuthFamily(ctx, code.FamilyID)
	if err != nil {
		return OAuthTokenOutput{}, errOAuthInternal("failed to look up token family", err)
	}

	// Replay: a used code is presented again. Tokens already issued from it are
	// revoked, as RFC 6749 section 4.1.2 recommends.
	if code.UsedAt != nil {
		return OAuthTokenOutput{}, uc.codeReplayed(ctx, family)
	}
	if !now.Before(code.ExpiresAt) {
		return OAuthTokenOutput{}, errOAuthInvalidGrant("authorization code has expired")
	}
	if family.ClientID != in.ClientID {
		return OAuthTokenOutput{}, errOAuthInvalidGrant("authorization code was not issued to this client")
	}
	if in.RedirectURI != code.RedirectURI {
		return OAuthTokenOutput{}, errOAuthInvalidGrant("redirect_uri does not match the authorization request")
	}
	if !domain.VerifyPKCES256(in.CodeVerifier, code.CodeChallenge) {
		return OAuthTokenOutput{}, errOAuthInvalidGrant("PKCE verification failed")
	}
	if in.Resource != "" && in.Resource != family.Resource {
		return OAuthTokenOutput{}, oauthErr(apperrors.KindInvalidArgument, CodeOAuthInvalidTarget, "resource does not match the authorization request", nil)
	}
	user, err := uc.checkFamilyUsable(ctx, family)
	if err != nil {
		return OAuthTokenOutput{}, err
	}
	scopes, err := effectiveScopes(family.Scope, user.Role)
	if err != nil {
		return OAuthTokenOutput{}, err
	}

	// Sign before claiming so a signer outage doesn't burn the code.
	access, expiresAt, err := uc.signAccessToken(ctx, user, family, scopes, now)
	if err != nil {
		return OAuthTokenOutput{}, err
	}
	rawRefresh, err := generateRandomToken(oauthRefreshTokenBytes)
	if err != nil {
		return OAuthTokenOutput{}, errOAuthInternal("failed to generate refresh token", err)
	}
	first := domain.OAuthRefreshToken{
		TokenHash: hashToken(rawRefresh), FamilyID: family.FamilyID, TenantID: family.TenantID,
		ExpiresAt: family.CreatedAt.Add(uc.cfg.RefreshTokenTTL),
	}
	if err := uc.repo.ClaimOAuthAuthCode(ctx, codeHash, now, first); err != nil {
		if errors.Is(err, ErrOAuthCodeAlreadyUsed) {
			return OAuthTokenOutput{}, uc.codeReplayed(ctx, family)
		}
		return OAuthTokenOutput{}, errOAuthInternal("failed to consume authorization code", err)
	}
	_ = uc.repo.TouchOAuthClientUsed(ctx, family.ClientID, now)
	uc.audit.record(ctx, family.TenantID, family.UserID, "oauth.token_issued", "oauth_family", family.FamilyID,
		map[string]any{"client_id": family.ClientID, "grant_id": family.GrantID, "scope": domain.FormatOAuthScopes(scopes)}, domain.OutcomeAllowed)
	return OAuthTokenOutput{
		AccessToken: access, ExpiresIn: int(expiresAt.Sub(now).Seconds()),
		RefreshToken: rawRefresh, Scope: domain.FormatOAuthScopes(scopes),
	}, nil
}

func (uc *OAuthExchangeToken) rotateRefresh(ctx context.Context, in OAuthExchangeTokenInput) (OAuthTokenOutput, error) {
	if in.RefreshToken == "" || in.ClientID == "" {
		return OAuthTokenOutput{}, errOAuthInvalidRequest("refresh_token and client_id are required")
	}
	now := uc.clock.Now()
	oldHash := hashToken(in.RefreshToken)
	rt, err := uc.repo.GetOAuthRefreshToken(ctx, oldHash)
	if errors.Is(err, ErrOAuthRefreshNotFound) {
		return OAuthTokenOutput{}, errOAuthInvalidGrant("refresh token is invalid")
	}
	if err != nil {
		return OAuthTokenOutput{}, errOAuthInternal("failed to look up refresh token", err)
	}
	family, err := uc.repo.GetOAuthFamily(ctx, rt.FamilyID)
	if err != nil {
		return OAuthTokenOutput{}, errOAuthInternal("failed to look up token family", err)
	}
	if family.ClientID != in.ClientID {
		return OAuthTokenOutput{}, errOAuthInvalidGrant("refresh token was not issued to this client")
	}
	if family.RevokedAt != nil {
		return OAuthTokenOutput{}, errOAuthInvalidGrant("grant has been revoked")
	}
	if rt.UsedAt != nil {
		return OAuthTokenOutput{}, uc.refreshReused(ctx, family)
	}
	if !now.Before(rt.ExpiresAt) {
		return OAuthTokenOutput{}, errOAuthInvalidGrant("refresh token has expired")
	}
	if in.Resource != "" && in.Resource != family.Resource {
		return OAuthTokenOutput{}, oauthErr(apperrors.KindInvalidArgument, CodeOAuthInvalidTarget, "resource does not match the authorization request", nil)
	}
	user, err := uc.checkFamilyUsable(ctx, family)
	if err != nil {
		return OAuthTokenOutput{}, err
	}

	// A refresh may only narrow scope relative to the family.
	requested := family.Scope
	if in.Scope != "" {
		want, err := domain.ParseOAuthScopes(in.Scope)
		have, _ := domain.ParseOAuthScopes(family.Scope)
		if err != nil || len(want) == 0 || !domain.IsScopeSubset(want, have) {
			return OAuthTokenOutput{}, oauthErr(apperrors.KindInvalidArgument, CodeOAuthInvalidScope, "scope exceeds the originally granted scope", nil)
		}
		requested = domain.FormatOAuthScopes(want)
	}
	scopes, err := effectiveScopes(requested, user.Role)
	if err != nil {
		return OAuthTokenOutput{}, err
	}

	access, expiresAt, err := uc.signAccessToken(ctx, user, family, scopes, now)
	if err != nil {
		return OAuthTokenOutput{}, err
	}
	rawNext, err := generateRandomToken(oauthRefreshTokenBytes)
	if err != nil {
		return OAuthTokenOutput{}, errOAuthInternal("failed to generate refresh token", err)
	}
	// Absolute lifetime: rotation never extends past family creation + TTL.
	next := domain.OAuthRefreshToken{
		TokenHash: hashToken(rawNext), FamilyID: family.FamilyID, TenantID: family.TenantID,
		ExpiresAt: family.CreatedAt.Add(uc.cfg.RefreshTokenTTL),
	}
	if err := uc.repo.RotateOAuthRefreshToken(ctx, oldHash, now, next); err != nil {
		if errors.Is(err, ErrOAuthRefreshAlreadyUsed) {
			return OAuthTokenOutput{}, uc.refreshReused(ctx, family)
		}
		return OAuthTokenOutput{}, errOAuthInternal("failed to rotate refresh token", err)
	}
	_ = uc.repo.TouchOAuthClientUsed(ctx, family.ClientID, now)
	uc.audit.record(ctx, family.TenantID, family.UserID, "oauth.refresh_rotated", "oauth_family", family.FamilyID,
		map[string]any{"client_id": family.ClientID, "grant_id": family.GrantID}, domain.OutcomeAllowed)
	return OAuthTokenOutput{
		AccessToken: access, ExpiresIn: int(expiresAt.Sub(now).Seconds()),
		RefreshToken: rawNext, Scope: domain.FormatOAuthScopes(scopes),
	}, nil
}

func (uc *OAuthExchangeToken) codeReplayed(ctx context.Context, f domain.OAuthTokenFamily) error {
	now := uc.clock.Now()
	_ = uc.repo.RevokeOAuthFamily(ctx, f.FamilyID, domain.OAuthRevokeReuseDetected, now)
	uc.audit.record(ctx, f.TenantID, f.UserID, "oauth.code_replay", "oauth_family", f.FamilyID,
		map[string]any{"client_id": f.ClientID, "grant_id": f.GrantID}, domain.OutcomeDenied)
	return errOAuthInvalidGrant("authorization code has already been used")
}

func (uc *OAuthExchangeToken) refreshReused(ctx context.Context, f domain.OAuthTokenFamily) error {
	now := uc.clock.Now()
	_ = uc.repo.RevokeOAuthFamily(ctx, f.FamilyID, domain.OAuthRevokeReuseDetected, now)
	uc.audit.record(ctx, f.TenantID, f.UserID, "oauth.refresh_reuse_detected", "oauth_family", f.FamilyID,
		map[string]any{"client_id": f.ClientID, "grant_id": f.GrantID}, domain.OutcomeDenied)
	return errOAuthInvalidGrant("refresh token has already been used")
}

// checkFamilyUsable enforces the live conditions every token issuance needs:
// family not revoked, client still allowed in the tenant, grant not revoked,
// user still active.
func (uc *OAuthExchangeToken) checkFamilyUsable(ctx context.Context, f domain.OAuthTokenFamily) (domain.User, error) {
	if f.RevokedAt != nil {
		return domain.User{}, errOAuthInvalidGrant("grant has been revoked")
	}
	if err := requireOAuthClientAllowed(ctx, uc.repo, f.TenantID, f.ClientID); err != nil {
		return domain.User{}, err
	}
	if revoked, err := uc.repo.IsOAuthGrantRevoked(ctx, f.TenantID, f.GrantID); err != nil {
		return domain.User{}, errOAuthInternal("failed to check grant", err)
	} else if revoked {
		return domain.User{}, errOAuthInvalidGrant("grant has been revoked")
	}
	user, err := uc.users.GetUserByID(ctx, f.UserID)
	if errors.Is(err, ErrUserNotFound) || (err == nil && (!user.IsActive || user.TenantID != f.TenantID)) {
		return domain.User{}, errOAuthInvalidGrant("user is no longer active")
	}
	if err != nil {
		return domain.User{}, errOAuthInternal("failed to look up user", err)
	}
	return user, nil
}

// effectiveScopes caps the family's scopes by the user's current role, so a
// demoted user's existing grant shrinks instead of keeping stale power.
func effectiveScopes(familyScope string, role domain.Role) ([]string, error) {
	granted, err := domain.ParseOAuthScopes(familyScope)
	if err != nil {
		return nil, errOAuthInternal("stored scope is invalid", err)
	}
	scopes := domain.IntersectOAuthScopes(granted, domain.OAuthScopeCeilingForRole(role))
	if len(scopes) == 0 {
		return nil, errOAuthInvalidGrant("no granted scope is permitted for the user's role")
	}
	return scopes, nil
}

func (uc *OAuthExchangeToken) signAccessToken(ctx context.Context, user domain.User, f domain.OAuthTokenFamily, scopes []string, now time.Time) (string, time.Time, error) {
	jti, err := generateRandomToken(oauthAccessJTIBytes)
	if err != nil {
		return "", time.Time{}, errOAuthInternal("failed to generate token id", err)
	}
	expiresAt := now.Add(uc.cfg.AccessTokenTTL)
	// iss stays jwtauth.Issuer so existing verifiers accept the signature; role
	// is deliberately absent and read live at validation time.
	claims := jwtauth.Claims{
		Claims: jwt.Claims{
			Issuer: jwtauth.Issuer, Subject: user.ID, Audience: jwt.Audience{uc.cfg.ResourceURL},
			IssuedAt: jwt.NewNumericDate(now), Expiry: jwt.NewNumericDate(expiresAt), ID: jti,
		},
		TenantID: f.TenantID, Scope: domain.FormatOAuthScopes(scopes), ClientID: f.ClientID,
		GrantID: f.GrantID, FamilyID: f.FamilyID, TokenUse: oauthTokenUseOAuth,
	}
	token, err := uc.signer.Sign(ctx, claims)
	if err != nil {
		return "", time.Time{}, errOAuthInternal("failed to sign access token", err)
	}
	return token, expiresAt, nil
}
