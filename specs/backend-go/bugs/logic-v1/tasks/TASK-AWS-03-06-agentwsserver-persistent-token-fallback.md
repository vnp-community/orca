# TASK-AWS-03-06: Reconcile `agentwsserver` handshake with persistent tokens + implement `LiveSessionCloser`

**From Solution:** SOL-AWS-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go`, `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go`
**Depends on:** TASK-AWS-03-04, TASK-AWS-03-05
**Status:** `[ ]` TODO

---

## Context

`Server.handleConnection` (`server.go:120-172`) today only checks
`Registry.Consume` (the ephemeral bootstrap slot). Add a fallback to the
new persistent `AgentTokenRepository.FindActiveByHash` so a direct-websocket
agent can reconnect with a long-lived, non-single-use token. Both
mechanisms coexist — `Registry`/`TokenIssuer` stay unchanged, per SOL-AWS-03.

## Changes to make

In `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go`,
add a narrow port and thread it through `Server`:

```go
// TokenValidator is the fallback token check for direct-websocket
// handshakes once Registry.Consume misses — backed by
// usecase.AgentTokenRepository.FindActiveByHash (TASK-AWS-03-04). Defined
// here, not in usecase/, since this package already defines its own narrow
// seams (see InboundSessionAttacher's doc comment).
type TokenValidator interface {
	// FindActiveByHash returns the DevServer ID a persistent, non-revoked
	// token hashes to. found=false means no match — try the caller's next
	// fallback / fail the handshake.
	FindActiveByHash(ctx context.Context, hash string) (devServerID string, tokenID string, found bool, err error)
	// TouchLastUsed is called best-effort on a hit — never blocks the
	// handshake on its result.
	TouchLastUsed(ctx context.Context, tokenID string)
}
```

Update `Server` and `handleConnection`:

```go
type Server struct {
	Registry *Registry
	Client   InboundSessionAttacher
	Tokens   TokenValidator // may be nil — falls back to Registry-only (bootstrap-flow-only deployments)
	Cfg      Config
	Logger   *slog.Logger
}
```

```go
// handleConnection ... (replace the Registry.Consume-only block)
devServerID, ok := s.Registry.Consume(params.AgentToken)
if !ok && s.Tokens != nil {
	var tokenID string
	hash := hashAgentToken(params.AgentToken) // sha256 hex, matches slots.go's hashToken
	devServerID, tokenID, ok = s.Tokens.FindActiveByHash(hctx, hash)
	if ok {
		s.Tokens.TouchLastUsed(hctx, tokenID)
	}
}
if !ok {
	s.rejectHandshake(hctx, conn, req.ID)
	return
}
```

Add the shared hash helper (mirrors `slots.go`'s unexported `hashToken`,
duplicated here rather than exported cross-package, matching this
package's existing self-containment):

```go
func hashAgentToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
```

(add `"crypto/sha256"`, `"encoding/hex"` imports)

Implement `usecase.LiveSessionCloser` on `devserveragent.Client`
(`client.go`) — reuses the existing `sessions` map (`client.go:93-99`):

```go
// CloseSessionsForDevServerToken implements usecase.LiveSessionCloser.
// direct-websocket only: this Client tracks at most one live session per
// devServerID (see the Client doc comment), so "close the session
// authenticated as tokenID" reduces to "close devServerID's current
// session" — the token/session binding itself isn't tracked separately
// (a revoked token's *next* handshake attempt is what actually enforces
// revocation; this call just also drops the currently-live connection, if
// any, per SOL-AWS-03's immediate-effect guarantee).
func (c *Client) CloseSessionsForDevServerToken(ctx context.Context, devServerID, tokenID string) (int, error) {
	c.mu.Lock()
	sess, ok := c.sessions[devServerID]
	c.mu.Unlock()
	if !ok || !sess.isHandshaked() {
		return 0, nil
	}
	sess.close()
	return 1, nil
}
```

Add `var _ usecase.LiveSessionCloser = (*Client)(nil)` near `Client`'s other
interface assertions in `client.go`.

In `services/infra-fleet-service/cmd/server/main.go`, construct
`agentTokenStore` (TASK-AWS-03-04) as `agentwsserver.Server.Tokens` — an
adapter/shim implementing `TokenValidator` over
`usecase.AgentTokenRepository` (thin wrapper, same pattern as other narrow
port adapters in this package).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/agentwsserver/... ./services/infra-fleet-service/internal/adapter/devserveragent/...
```

Expected: clean build/tests. Add to `agentwsserver/server_test.go`: a
handshake succeeds against a persistent (non-`Registry`) token, and
succeeds again on a **second** handshake with the same token (proves
non-single-use); a revoked token's handshake is rejected. Add to
`devserveragent`'s client tests: `CloseSessionsForDevServerToken` closes an
active session and returns `closed=1`; a devServer with no live session
returns `(0, nil)`, not an error.
