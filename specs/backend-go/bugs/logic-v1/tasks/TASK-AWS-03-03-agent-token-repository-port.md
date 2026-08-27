# TASK-AWS-03-03: Add `AgentTokenRepository`/`LiveSessionCloser` ports

**From Solution:** SOL-AWS-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/ports.go`
**Depends on:** TASK-AWS-03-02
**Status:** `[ ]` TODO

---

## Context

The usecases (TASK-AWS-03-05), the Postgres adapter (TASK-AWS-03-04), and
the `agentwsserver` handshake fallback (TASK-AWS-03-06) all need these two
ports defined first, per this codebase's Dependency Inversion convention
(ports live in `usecase/`, implemented in `adapter/*`).

## Changes to make

Append to `backend-go/services/infra-fleet-service/internal/usecase/ports.go`
(same file already holding `DevServerRepository`, `FleetHealthPort`, etc.):

```go
// AgentTokenRepository is the persistence port for infra.agent_tokens
// (migrations/0007_agent_tokens, TASK-AWS-03-01) — BL-AWS-03's persistent,
// named, per-DevServer agent token set. tenantID is threaded explicitly on
// every method, matching ConnectionResolver/TerminalSessionRepository's
// convention.
type AgentTokenRepository interface {
	// CountActive returns the number of non-revoked tokens for devServerID
	// — enforces domain.MaxActiveAgentTokensPerDevServer.
	CountActive(ctx context.Context, tenantID, devServerID string) (int, error)
	// Insert persists a new token row. Callers must set exactly one of
	// TokenHash/CredentialRefID (domain.AgentToken's own invariant,
	// enforced again by the table's exactly_one_secret_ref CHECK).
	Insert(ctx context.Context, t domain.AgentToken) error
	// ListActive returns every non-revoked token for devServerID, newest
	// first — backs ListAgentTokens.
	ListActive(ctx context.Context, tenantID, devServerID string) ([]domain.AgentToken, error)
	// FindActiveByHash looks up a non-revoked direct-websocket token by its
	// SHA-256 hash — the agentwsserver handshake fallback's read path
	// (TASK-AWS-03-06). found=false, err=nil means "no such active token".
	FindActiveByHash(ctx context.Context, hash string) (t domain.AgentToken, found bool, err error)
	// ActiveForDevServer returns the most-recently-created non-revoked
	// token for a relay-websocket DevServer — SOL-AWS-01's per-dial
	// resolution read. Relay-websocket DevServers are expected to carry
	// exactly one active token in ordinary operation. found=false, err=nil
	// means "no active token registered yet".
	ActiveForDevServer(ctx context.Context, tenantID, devServerID string) (t domain.AgentToken, found bool, err error)
	// TouchLastUsed bumps last_used_at — called best-effort on a successful
	// handshake/dial, never blocks the caller on its result.
	TouchLastUsed(ctx context.Context, id string) error
	// Revoke sets revoked_at and returns the updated row.
	Revoke(ctx context.Context, tenantID, id string) (domain.AgentToken, error)
}

// LiveSessionCloser closes any live direct-websocket session currently
// authenticated with a given agent token — RevokeAgentToken's
// immediate-effect guarantee (TASK-AWS-03-06's usecase calls this after
// AgentTokenRepository.Revoke). Implemented by devserveragent.Client, which
// already tracks one live session per devServerID.
type LiveSessionCloser interface {
	// CloseSessionsForDevServerToken closes any direct-websocket session on
	// devServerID currently authenticated as tokenID, with WS close code
	// 1008 and a "token revoked" reason — see SOL-AWS-02 for why 1008, not
	// the never-implemented 4001.
	CloseSessionsForDevServerToken(ctx context.Context, devServerID, tokenID string) (closed int, err error)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/internal/usecase/...
```

Expected: clean build (no implementations required yet — this is an
interface-only addition, the fake test implementations land with
TASK-AWS-03-05).
