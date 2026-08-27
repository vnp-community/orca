# BUG-AUTH-03: Per-user OS process sandbox has no backend-go equivalent

**Business Logic:** [BL-AUTH-03](../../../../docs/logic/auth/BL-AUTH-03-per-user-sandbox.md) — Per-User Process Sandbox
**Priority (per spec):** P0
**Status:** NOT_IMPLEMENTED
**Severity:** High
**Symptom:** There is no operation a user or admin can perform that touches a per-user forked process, a per-user Unix socket, or a per-user `~/.orca/users/<userId>/` data directory — none of it exists in backend-go. A user's "process" in backend-go is just their `tenant_id`/`user_id` scoping a row in shared Postgres tables and a shared pool of stateless gRPC services; a crash in one user's request handling is isolated only in the sense that any stateless RPC call is isolated (no shared mutable state across requests), not because of OS-level process/fork isolation.

---

## Spec summary

Each logged-in user gets a dedicated Node.js child process (`fork()`), with its own Unix domain socket, its own per-user data directory (`~/.orca/users/<userId>/` containing `orca.sock`, `orca.db`, `credentials.enc`, `worktrees/`), a 4h idle timeout, a 30s spawn timeout, and up to 3 respawn attempts on crash before the process is abandoned and an admin is alerted.

## What backend-go has

Nothing that matches this mechanism. Confirmed by exhaustive search:

```
$ grep -rln "fork\|chroot\|Unix socket\|orca.sock\|SessionManager\|WsSessionRouter\|child process\|ChildProcess" backend-go --include="*.go" | grep -v _test.go
(no matches whatsoever across all of backend-go)
```

`backend-go`'s architecture is a set of stateless gRPC microservices (`api-gateway`, `auth-service`, `project-service`, `credential-broker-service`, etc.) behind a single `api-gateway` that terminates HTTP/WS and forwards to whichever service owns the resource, scoped by `tenant_id`/`user_id` columns in a shared Postgres database (`common/tenant/tenant.go:18-53`) — not by spawning a process per user. There is no `~/.orca/users/<userId>/` directory concept, no per-user SQLite file, no per-user Unix socket, and no idle-timeout/respawn logic anywhere in `backend-go/services`.

The one piece of the spec's *intent* — isolating a user's secret material — has a real, arguably stronger substitute: `credential-broker-service` stores credential *metadata* only (never a secret value) in Postgres, pointing at ciphertext held in HashiCorp Vault, scoped by `tenant_id`+`owner_id` (`backend-go/services/credential-broker-service/internal/domain/credential_metadata.go:110-124`, `:133-173`). This replaces the old `credentials.enc` (AES-256-GCM file per user) with a centralized secrets manager — a legitimate architectural alternative for *credential* isolation, but it says nothing about *process*/compute isolation, which this BL is fundamentally about.

## What's missing

- No `fork()`/child-process spawn of any kind, per user or otherwise.
- No per-user Unix domain socket (`orca.sock`) or any WS↔socket proxy.
- No per-user data directory layout (`orca.db`, `worktrees/` under a per-user root) — worktrees in backend-go live in `project-service`'s Postgres-backed `worktree_repository.go`, addressed by `tenant_id`/`project_id`, not a per-user filesystem path.
- No idle-timeout (spec: 4h) or spawn-timeout (spec: 30s) supervision loop.
- No crash/respawn logic (spec: max 3 respawns, then "abandon + alert admin") — there is nothing to crash or respawn.
- No admin action wired to "SIGTERM child process of userId" as part of deactivation (see BUG-AUTH-04 for what `DeactivateUser` actually does instead).

## See also

- BUG-AUTH-02 (session lifecycle) — the spec's WS routing step ("`WsSessionRouter.route(userId, ws)` → fork/reuse child process") depends entirely on the mechanism described here; that gap is cross-referenced there rather than duplicated.

## References

- `backend-go/services/credential-broker-service/internal/domain/credential_metadata.go:1-198` — the credential-isolation substitute (Vault-backed, not process-based)
- `backend-go/common/tenant/tenant.go:18-53` — the actual isolation primitive backend-go uses (`tenant_id` context value + scoped SQL), not OS processes
- `docs/logic/auth/BL-AUTH-03-per-user-sandbox.md`
