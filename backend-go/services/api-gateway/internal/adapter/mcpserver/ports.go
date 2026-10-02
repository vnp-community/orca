package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Principal is the authenticated caller of /mcp. It is produced ONLY by a
// TokenVerifier from a bearer token, never from cookies, headers or sessions.
type Principal struct {
	TenantID  string
	UserID    string
	Role      string
	ClientID  string
	TokenID   string
	GrantID   string
	Scopes    []string
	ExpiresAt time.Time // zero = bounded by the verifier (treated as short-lived)
	// Depth / Root come from the VERIFIED token's mcp_depth / mcp_root claims
	// (agent recursion guard); never from request data.
	Depth int
	Root  string
}

// Verifier errors; the middleware maps them to HTTP challenges.
var (
	// ErrNoCredentials: no bearer token presented -> 401 without error attr.
	ErrNoCredentials = errors.New("mcpserver: no bearer credentials")
	// ErrInvalidToken: token rejected (bad signature, expired, wrong aud,
	// revoked...) -> 401 error="invalid_token".
	ErrInvalidToken = errors.New("mcpserver: invalid token")
	// ErrInsufficientScope: valid token lacking scope -> 403.
	ErrInsufficientScope = errors.New("mcpserver: insufficient scope")
	// ErrKillSwitchActive: an administrator stopped this tenant/client/grant/
	// session -> 403 MCP_KILL_SWITCH_ACTIVE (not a 401 challenge).
	ErrKillSwitchActive = errors.New("mcpserver: MCP_KILL_SWITCH_ACTIVE")
	// ErrVerifierUnavailable: the token could not be checked (auth-service
	// down). Fail closed with 503, not 401, so clients do not loop on re-login.
	ErrVerifierUnavailable = errors.New("mcpserver: token verification unavailable")
)

// TokenVerifier validates the bearer token of a /mcp request (BE-MCP-SOL-005/006
// plug in here: signature, aud == resourceUrl, revocation, PAT lookup). It MUST
// NOT read cookies. Any error other than the sentinels above is treated as
// ErrInvalidToken (fail closed).
type TokenVerifier interface {
	Verify(ctx context.Context, r *http.Request) (Principal, error)
}

// denyAllVerifier is the default until real validation exists: nothing passes.
type denyAllVerifier struct{}

func (denyAllVerifier) Verify(_ context.Context, r *http.Request) (Principal, error) {
	if BearerToken(r) == "" {
		return Principal{}, ErrNoCredentials
	}
	return Principal{}, ErrInvalidToken
}

// BearerToken extracts the token of an "Authorization: Bearer <t>" header.
func BearerToken(r *http.Request) string {
	f := strings.Fields(r.Header.Get("Authorization"))
	if len(f) == 2 && strings.EqualFold(f[0], "bearer") {
		return f[1]
	}
	return ""
}

// ToolCatalog lists the tools visible to a principal (BE-MCP-SOL-007 implements
// it). The adapter owns pagination: return the full visible set; cursors are
// signed and bound to the MCP session by the adapter.
type ToolCatalog interface {
	ListTools(ctx context.Context, p Principal) ([]*mcp.Tool, error)
}

// ErrUnknownTool makes tools/call answer a JSON-RPC -32602 protocol error.
var ErrUnknownTool = errors.New("mcpserver: unknown tool")

// ToolExecutor runs tools/call (seam for BE-MCP-SOL-007/008). Return
// ErrUnknownTool for an unknown name (protocol error); any other error is
// turned into an isError:true tool result via MapToolError; a *CallToolResult
// with IsError may also be returned directly. Policy allow/deny/approval is the
// executor's concern (decided by mcp-service), not this adapter's.
type ToolExecutor interface {
	CallTool(ctx context.Context, p Principal, name string, args json.RawMessage) (*mcp.CallToolResult, error)
}

// emptyCatalog is the BE-003 default: no tools yet.
type emptyCatalog struct{}

func (emptyCatalog) ListTools(context.Context, Principal) ([]*mcp.Tool, error) { return nil, nil }

// Recorder is the metrics hook point (BE-MCP-SOL-015 supplies a real one).
type Recorder interface {
	AuthFailure(reason string)
	IdentityMismatch()
	RateLimited()
}

type noopRecorder struct{}

func (noopRecorder) AuthFailure(string) {}
func (noopRecorder) IdentityMismatch()  {}
func (noopRecorder) RateLimited()       {}

type principalKey struct{}

// ContextWithPrincipal / PrincipalFromContext carry the verified principal.
func ContextWithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
