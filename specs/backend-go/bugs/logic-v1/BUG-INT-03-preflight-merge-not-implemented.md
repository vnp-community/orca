# BUG-INT-03: mergePreflightStatuses has no backend-go equivalent — there is nothing to merge

**Business Logic:** [BL-INT-03](../../../../docs/logic/remote-integration/BL-INT-03-preflight-merge.md) — Preflight Status Merge (Local + Remote)
**Priority (per spec):** P1
**Status:** NOT_IMPLEMENTED
**Severity:** High
**Symptom:** The preflight panel can never distinguish a `[local]`-sourced check from a `[relay]`-sourced one, never shows a "Cannot reach Dev Server — showing local checks only" warning, and never actually runs any remote (Dev Server) checks to merge in the first place — every response is one hardcoded literal with no `source` field and no relay attempt.

---

## Spec summary

BL-INT-03's `mergePreflightStatuses()` combines local-Orca checks (git version, API token format, network reachability) with remote Dev-Server checks (via SSH relay: `gh`/`glab` auth status, Node version, disk space, port availability), with relay results overriding local ones by `id`, and a `relay-connectivity` warning check injected when the SSH relay itself is unreachable. Each `PreflightCheckResult` carries a `source: 'local' | 'relay'` field.

## What backend-go has

Nothing beyond the single stub handler shared with BL-INT-01:

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:565-573` (`registerPreflightChannels`) — returns a fixed 3-key map (`git`/`gh`/`glab`), not a list of `PreflightCheckResult` objects, and has no `id`/`source`/`status`/`message` schema at all.
- Confirmed via repo-wide search: `grep -rn "mergePreflight\|MergePreflight\|PreflightCheckResult\|relay-connectivity"` across all of `backend-go/` returns **zero matches**. No merge function, no result type, no relay-connectivity-warning concept exists anywhere in the Go codebase.
- There are no "local checks" being run either — `preflight.check`'s `git: {installed: true}` (`channels.go:568`) is a hardcoded literal, not the result of actually invoking `git --version` or checking `WebCredentialStore`/`credential-broker-service` token status.

## What's missing

- No `PreflightCheckResult` type (`id`, `status: ok|warning|error|skip`, `message`, `details?`, `source: local|relay`) anywhere in backend-go.
- No local-checks runner (git version probe, integration-token format validation, network reachability ping) — everything is a static literal.
- No relay-checks runner (SSH relay to Dev Server for `gh auth status`/`glab auth status`/`node --version`/disk-space/port-availability) — this also depends on BUG-INT-01's SSH-relay proxy, which doesn't exist.
- No merge algorithm — no seed-with-local-then-override-with-relay logic, no per-`id` `Map`-style merge, no `relay-connectivity` warning injection on SSH failure.
- No `RpcMethodContext`-equivalent `devServerManager` injected into `preflight.check`'s handler — it can't proxy to a Dev Server even in principle from its current signature.

## See also

- [BUG-INT-01](./BUG-INT-01-cli-auth-proxy-not-implemented.md) — the relay/CLI-check side this merge would need to combine with local results; also not implemented, for the same architectural reason (`scm-integration-service`'s direct-OAuth design deliberately doesn't proxy CLI checks to a Dev Server).

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:551-573` — the sole `preflight.*` handler, its doc comment, and the hardcoded response
- `docs/logic/remote-integration/BL-INT-03-preflight-merge.md` — spec (merge algorithm, `PreflightCheckResult` schema)
