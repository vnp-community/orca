# BUG-AG-04: Provider-resolution cascade is real, but nothing detects rate limits or drives the stop→resolve→respawn hot-swap

**Business Logic:** [BL-AG-04](../../../../docs/logic/agent-orchestration/BL-AG-04-switch-account.md) — Switch Account / Provider khi Rate Limited
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** High
**Symptom:** A rate-limited agent produces no `agent:rateLimited` signal anywhere in backend-go (no PTY output is even pattern-matched server-side), so the UI has nothing to alert on. Even if a user manually decided to switch accounts, there is no single flow that stops the old PTY, resolves a new provider, and respawns with new credentials — only the provider-resolution step (`AIProviderResolver.resolve()`) exists in isolation, unconnected to any spawn/kill call.

---

## Spec summary

On PTY-output rate-limit detection, Orca marks the session stopped, kills the PTY (BL-AG-02), resolves a new AI provider account via the same user > project > company priority cascade, and spawns a new PTY (BL-AG-01) with the new provider's credentials — optionally resuming the session (BL-AG-03) if compatible. Credentials must never be stored in plaintext (BR-AG-11): they're read from `.enc` files on the Dev Server only at spawn time. Usage counters reset after switching (BR-AG-13).

## What backend-go has

- **The provider-resolution cascade itself is fully and correctly implemented**: `ResolveProvider.Resolve` (`backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go:44-89`) implements exactly the "narrowest scope wins" order the spec calls for — user scope (Tier 1) → project scope (Tier 2) → server/company scope (Tier 3), each tier only consulted if the previous had no `Resolvable()` candidate (resolve_provider.go:52-84). Covered by cascade-order tests (`resolve_provider_test.go`). Exposed over gRPC (`backend-go/services/ai-provider-service/internal/adapter/grpc/server.go:71-79`) and REST `GET /v1/aiprovider/resolve` (`backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:29,66-90`).
- `ResolveProvider` deliberately returns metadata only (id, provider_type, credential_ref) — never a key (resolve_provider.go:29-31 doc comment) — consistent with BR-AG-11's "no plaintext credential" rule, as far as this one RPC's contract goes.
- Kill and (shell-only) spawn primitives exist independently (see BUG-AG-01, BUG-AG-02) but are not composed into a switch flow anywhere, and are not agent-aware.

## What's missing

- **No rate-limit detection on PTY output at all.** Grepping backend-go for `RATE_LIMIT_PATTERNS`-equivalent regexes or any pattern matching applied to `terminal.output`/PTY stream data returns nothing — the only "rate limit" code in backend-go is `scm-integration-service`'s unrelated SCM-API rate-limit-header tracking (`backend-go/services/scm-integration-service/internal/domain/scm.go:112-116`, `GetRateLimitStatus`), which is about GitHub/GitLab API quotas, not agent CLI output.
- No `agent:rateLimited { resetAt }` event or equivalent push channel.
- No orchestrating usecase that chains: mark-session-stopped → kill PTY → `ResolveProvider` → spawn-with-new-env → optional-resume. Each step besides `ResolveProvider` either doesn't exist (agent-aware spawn/session update, see BUG-AG-01/03) or is a disconnected generic primitive (kill, see BUG-AG-02).
- No credential injection at spawn time from the resolved account into the PTY's env (`SpawnPtyInput` has no `Env` field — see BUG-AG-01) — so even with a resolved provider, there is no code path that reads its `.enc` file and passes `apiKeyEnvVar` into a new process.
- No usage-counter reset after switch (BR-AG-13) — `usage-service` exists as a separate service but nothing calls it from a switch-account flow (no such flow exists to call it).

## See also

- specs/backend-go/bugs/logic-v1/BUG-AG-01-khoi-dong-agent-partial.md — the credential-injecting respawn step this flow depends on doesn't exist.
- specs/backend-go/bugs/logic-v1/BUG-AG-02-dung-agent-partial.md — the kill step this flow depends on is generic/PTY-only, not agent-session-aware.
- specs/backend-go/bugs/logic-v1/BUG-AG-05-monitor-status-partial.md — rate-limit detection is a special case of the same missing PTY-output-parsing pipeline documented there.

## References

- `docs/logic/agent-orchestration/BL-AG-04-switch-account.md`
- `backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go:11-89`
- `backend-go/services/ai-provider-service/internal/adapter/grpc/server.go:71-79`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:29,66-90`
- `backend-go/services/scm-integration-service/internal/domain/scm.go:112-116` — unrelated rate-limit concept, cited to rule it out
