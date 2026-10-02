package usecase

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/common/mcpscope"
)

var (
	// ErrAudienceMismatch: the token's aud is not exactly the MCP resource
	// (aud=orca-cli, missing aud and multi-valued aud all land here).
	ErrAudienceMismatch = errors.New("authvalidator: token audience is not the MCP resource")
	// ErrAudienceNotAccepted: REST/WS refuse a token minted for the MCP resource.
	ErrAudienceNotAccepted = errors.New("authvalidator: token audience is not accepted on this endpoint")
	// ErrPrincipalInactive: revoked, expired, user deactivated, client blocked...
	ErrPrincipalInactive = errors.New("authvalidator: mcp principal is not active")
	// ErrPrincipalLookupFailed: auth-service could not be asked (fail closed; /mcp answers 503).
	ErrPrincipalLookupFailed = errors.New("authvalidator: mcp principal lookup failed")
)

const (
	// McpTokenUseOAuth / McpTokenUsePAT are the token_use claim values.
	McpTokenUseOAuth = "mcp_oauth"
	McpTokenUsePAT   = "mcp_pat"
	// McpPATPrefix marks personal access tokens for humans and secret scanners.
	McpPATPrefix = "omp_"
)

// McpPrincipal is the verified caller of /mcp. Identity.Role is read live from
// auth-service, never from the token.
type McpPrincipal struct {
	Identity
	// Scopes are the token's scopes intersected with the live role's ceiling.
	Scopes    []string
	TokenUse  string
	ClientID  string
	GrantID   string
	JTI       string
	ExpiresAt time.Time
	// McpDepth / McpRoot are the VERIFIED mcp_depth / mcp_root claims (agent
	// recursion guard): 0/"" for tokens minted by a person.
	McpDepth int
	McpRoot  string
}

// McpResolveInput / McpResolveResult mirror auth-service ResolveMcpPrincipal.
type McpResolveInput struct {
	JTI, UserID, TenantID, TokenUse, FamilyID, GrantID, ClientID string
}

type McpResolveResult struct {
	Active         bool
	InactiveReason string
	Role           string
}

// McpPrincipalResolver asks auth-service whether an MCP token is still usable.
// Implementations cache briefly (see authclient.McpPrincipalResolver).
type McpPrincipalResolver interface {
	Resolve(ctx context.Context, in McpResolveInput) (McpResolveResult, error)
}

// ValidateMCP authenticates a /mcp request. It reads ONLY "Authorization:
// Bearer" - cookieToken is deliberately never consulted, so a browser session
// can never be replayed against /mcp. The token must be an MCP token whose aud
// is exactly resourceURL, and auth-service must still consider it active.
func (v *AuthValidator) ValidateMCP(r *http.Request, resourceURL string) (McpPrincipal, error) {
	raw := strings.TrimPrefix(bearerToken(r), McpPATPrefix)
	if raw == "" {
		return McpPrincipal{}, ErrNoCredential
	}
	kid, err := jwtauth.KeyID(raw)
	if err != nil {
		return McpPrincipal{}, ErrMalformedToken
	}
	key, err := v.jwks.PublicKey(r.Context(), kid)
	if err != nil {
		return McpPrincipal{}, ErrKeyLookupFailed
	}
	c, err := jwtauth.VerifyWithKey(key, raw)
	if err != nil {
		return McpPrincipal{}, ErrSignatureVerificationFailed
	}
	if len(c.Audience) != 1 || strings.TrimRight(c.Audience[0], "/") != strings.TrimRight(resourceURL, "/") {
		return McpPrincipal{}, ErrAudienceMismatch
	}
	if c.TenantID == "" || c.Subject == "" || c.ID == "" || (c.TokenUse != McpTokenUseOAuth && c.TokenUse != McpTokenUsePAT) {
		return McpPrincipal{}, ErrMissingIdentityClaims
	}
	scopes, err := mcpscope.Parse(c.Scope)
	if err != nil {
		return McpPrincipal{}, ErrMissingIdentityClaims
	}
	if v.McpPrincipals == nil {
		return McpPrincipal{}, ErrPrincipalLookupFailed
	}
	res, err := v.McpPrincipals.Resolve(r.Context(), McpResolveInput{
		JTI: c.ID, UserID: c.Subject, TenantID: c.TenantID, TokenUse: c.TokenUse,
		FamilyID: c.FamilyID, GrantID: c.GrantID, ClientID: c.ClientID,
	})
	if err != nil {
		return McpPrincipal{}, ErrPrincipalLookupFailed
	}
	if !res.Active || res.Role == "" {
		return McpPrincipal{}, ErrPrincipalInactive
	}
	p := McpPrincipal{
		Identity: Identity{TenantID: c.TenantID, UserID: c.Subject, Role: res.Role},
		Scopes:   mcpscope.Intersect(scopes, mcpscope.CeilingForRole(res.Role)),
		TokenUse: c.TokenUse, ClientID: c.ClientID, GrantID: c.GrantID, JTI: c.ID,
		McpDepth: c.McpDepth, McpRoot: c.McpRoot,
	}
	if c.Expiry != nil {
		p.ExpiresAt = c.Expiry.Time()
	}
	return p, nil
}

// audienceRejected reports whether any audience of the token is one the
// REST/WS validator must refuse, or the token is an MCP token regardless of aud.
func (v *AuthValidator) audienceRejected(aud []string, tokenUse string) bool {
	if strings.HasPrefix(tokenUse, "mcp_") {
		return true
	}
	for _, a := range aud {
		for _, rej := range v.RejectAudiences {
			if strings.TrimRight(a, "/") == strings.TrimRight(rej, "/") {
				return true
			}
		}
	}
	return false
}
