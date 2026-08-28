// Package usecase holds api-gateway's cross-cutting request handling —
// auth validation, rate-limit decisioning, WS-bridge lifecycle — not
// per-domain business use cases, because this service has none. See
// specs/backend-go/services/api-gateway.md §6.
package usecase

import "context"

// JWKSClient fetches auth-service's public keys for real RS256 JWT
// signature verification (Epic D). Implemented by
// internal/adapter/authclient.JWKSClient, which caches the JWKS with a
// short TTL per api-gateway.md §5.
type JWKSClient interface {
	// PublicKey returns the public key for the given key ID (kid), fetched
	// from auth-service's JWKS endpoint and cached with a short TTL
	// per api-gateway.md §5.
	PublicKey(ctx context.Context, kid string) (any, error)
}

// RateLimitStore is the storage port a shared, multi-replica rate limiter
// (Redis-backed, per api-gateway.md §5) would implement. RateLimiter in
// rate_limit.go is a real, working per-replica in-memory implementation
// that does not need this interface; RateLimitStore exists so a future
// shared store can be swapped in behind the same Allow(tenantID) decision
// shape without touching callers.
type RateLimitStore interface {
	Allow(ctx context.Context, tenantID string) (bool, error)
}

// WorktreeCreator wraps git-gateway-service's already-real CreateWorktree
// RPC — see SOL-WT-01 for its validated shape. This saga only needs the
// project_id/repo_id/branch/base_ref subset.
type WorktreeCreator interface {
	CreateWorktree(ctx context.Context, projectID, repoID, branch, baseRef string) (worktreeID, path, headSHA string, err error)
}

// AgentSpawner composes project-service.GetProjectContext +
// infra-fleet-service's ResolveConnection/SpawnTerminalSession — "starting
// an agent" in this architecture is spawning a PTY running the agent's CLI
// command (business-capabilities.md's project.agentSpawn -> agent.exec
// framing).
type AgentSpawner interface {
	SpawnAgentTerminal(ctx context.Context, projectID, worktreePath, agentType string) (ptyID, connectionID string, err error)
}

// PromptInjector wraps infra-fleet-service's AttachPty bidirectional stream
// — opens it, sends AttachToSession{pty_id} then PtyInput{data: prompt},
// closes.
type PromptInjector interface {
	InjectPrompt(ctx context.Context, connectionID, ptyID, prompt string) error
}
