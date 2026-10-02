package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type OAuthRevokeTokenInput struct {
	Token    string
	ClientID string // optional; when given it must own the token or the call is a no-op
}

// OAuthRevokeToken implements RFC 7009. It never reveals whether the token
// existed: unknown, malformed, expired and foreign tokens all succeed with no
// effect. Revoking either token type revokes the whole family, since a
// refresh token and its access tokens are one authorization.
type OAuthRevokeToken struct {
	repo   OAuthRepository
	signer TokenSigner
	audit  oauthAuditor
	clock  Clock
}

func NewOAuthRevokeToken(repo OAuthRepository, signer TokenSigner, audit AuditRepository, clock Clock) *OAuthRevokeToken {
	return &OAuthRevokeToken{repo: repo, signer: signer, audit: oauthAuditor{audit: audit, clock: clock}, clock: clock}
}

func (uc *OAuthRevokeToken) Execute(ctx context.Context, in OAuthRevokeTokenInput) error {
	if in.Token == "" {
		return nil
	}
	familyID := uc.familyOf(ctx, in.Token)
	if familyID == "" {
		return nil
	}
	f, err := uc.repo.GetOAuthFamily(ctx, familyID)
	if err != nil || (in.ClientID != "" && f.ClientID != in.ClientID) {
		return nil
	}
	if err := uc.repo.RevokeOAuthFamily(ctx, f.FamilyID, domain.OAuthRevokeUserRevoked, uc.clock.Now()); err != nil {
		return errOAuthInternal("failed to revoke token", err)
	}
	uc.audit.record(ctx, f.TenantID, f.UserID, "oauth.token_revoked", "oauth_family", f.FamilyID,
		map[string]any{"client_id": f.ClientID, "grant_id": f.GrantID}, domain.OutcomeAllowed)
	return nil
}

// familyOf resolves the family of a refresh token (by hash) or of a signed
// access token (by verified "fid" claim); "" when the token is neither.
func (uc *OAuthRevokeToken) familyOf(ctx context.Context, token string) string {
	if rt, err := uc.repo.GetOAuthRefreshToken(ctx, hashToken(token)); err == nil {
		return rt.FamilyID
	} else if !errors.Is(err, ErrOAuthRefreshNotFound) {
		return ""
	}
	if strings.Count(token, ".") != 2 {
		return ""
	}
	jwks, err := uc.signer.PublicJWKS(ctx)
	if err != nil {
		return ""
	}
	// The signature is verified first: an unverified "fid" must never pick
	// which family gets revoked.
	claims, err := jwtauth.Verify(jwks, token)
	if err != nil || claims.TokenUse != oauthTokenUseOAuth {
		return ""
	}
	return claims.FamilyID
}
