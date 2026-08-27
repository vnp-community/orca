# SOL-AUTH-03: Retire BL-AUTH-03's fork-per-user model — document the stateless tenant-isolation replacement

**Resolves:** [BUG-AUTH-03](../BUG-AUTH-03-per-user-sandbox-not-implemented.md)
**Service:** N/A — this is a target-architecture documentation update, not a `backend-go` code change
**Affected files (proposed):**
- `docs/logic/auth/BL-AUTH-03-per-user-sandbox.md` — rewritten to describe the replacement isolation model (see full proposed content below)
**Status:** 📋 Proposed — **architecture decision, not an implementation task.** No `backend-go` code is proposed by this solution.

---

## Why this is a doc update, not an implementation gap

BUG-AUTH-03's own investigation already establishes the load-bearing fact:
an exhaustive `grep` across all of `backend-go` for `fork|chroot|Unix
socket|orca.sock|SessionManager|WsSessionRouter|child process|ChildProcess`
returns **zero matches** (BUG-AUTH-03 lines 19-22), and the bug report
itself identifies this is architectural, not an oversight — `backend-go`'s
services are stateless gRPC microservices scoped by `tenant_id`/`user_id`
columns in shared Postgres, "not by spawning a process per user"
(BUG-AUTH-03 line 24). This solution verifies that characterization against
the TDD's actual tenant-isolation design (`07-security-architecture.md`,
`02-microservices-decomposition.md`, `06-secrets-vault-architecture.md`) and
confirms it: **every one of these three architecture docs describes a
concrete, deliberately-chosen replacement for what BL-AUTH-03 calls
"per-user process isolation," and none of them describes it as a gap to
close.** Writing a "design — usecase" section that forks a child process
per user would contradict `02-microservices-decomposition.md`'s design
principle 1 ("no service reads another service's tables directly... each
service gets its own PostgreSQL database... independently scalable",
`02-microservices-decomposition.md:13-27`) and the horizontally-scaled,
stateless-except-for-the-DB non-functional requirement `auth-service.md` §8
states outright (`auth-service.md:259-261`: "Horizontally scaled, stateless
except for the DB"). A per-user long-lived child process is the opposite of
horizontally scaled and stateless — implementing it would be building
against the grain of every other service in the catalog.

The right fix is what BUG-AUTH-03's own text anticipates: update
`docs/logic/auth/BL-AUTH-03-per-user-sandbox.md` to describe the
**replacement isolation guarantee** backend-go actually provides, so the
"business logic" documentation set stops describing a mechanism that will
never be built and starts describing the mechanism that actually runs in
production.

## What the replacement isolation model actually is (verified against the TDD)

BL-AUTH-03's original mechanism bundled four distinct guarantees into one
per-user OS process: (1) compute/crash isolation, (2) filesystem isolation,
(3) secret-material isolation, (4) a supervision lifecycle (idle timeout,
spawn timeout, respawn-with-backoff, admin alert on abandonment). backend-go
replaces each with a different, independently-verifiable mechanism — not a
single 1:1 substitute, which is itself the most important thing for the
updated doc to make explicit rather than gloss over:

### 1. Tenant/user data isolation — layered defense, not OS boundaries

`07-security-architecture.md`'s "Multi-tenancy isolation" section
(`07-security-architecture.md:54-66`) specifies four independent layers:

1. **Database-per-service** — "blast radius of a compromised service is its
   own data, not the whole system" (not blast radius of a compromised
   *user*, notably a different guarantee than a per-user process gave).
2. **Application-layer `tenant_id` filtering on every query** (primary
   defense) — every repository method in every service (e.g.
   `backend-go/services/auth-service/internal/adapter/postgres/user_repository.go`'s
   `ListUsers(ctx, tenantID, ...)`) takes a `tenantID` scoping every SQL
   `WHERE` clause.
3. **Postgres RLS** (secondary defense) — a database-level backstop if the
   application-layer filter is ever missed.
4. **OPA policy input always includes the resolved tenant ID from the
   validated JWT/session — never trusted from the request body** — so even
   a correctly-authenticated user cannot forge a request for another
   tenant's data by passing a different `tenant_id` in the payload.

This is *user*-level isolation (not just tenant-level): `auth-service`'s own
tables scope by both `tenant_id` and `user_id`/`owner_id` throughout (e.g.
`auth.sessions.user_id`, per `auth-service.md:170`), and every service that
owns per-user data (`project-service` membership, `credential-broker-service`
metadata) follows the same `tenant_id`+`owner_id` scoping pattern BUG-AUTH-03
itself cites for `credential_metadata.go:110-124,133-173`.

### 2. Secret-material isolation — Vault, not a per-user encrypted file

`06-secrets-vault-architecture.md`'s table (`06-secrets-vault-architecture.md:17-29`)
is explicit that user secrets never touch application memory at rest: OAuth
tokens live in "Vault KV v2, one path per `(tenant, service, user)`", AI
provider keys go through "Vault Transit engine for encrypt/decrypt-as-a-service...
plaintext never touches application memory longer than the single call that
needs it." `credential-broker-service`'s Vault policy is scoped so "every
other service's Vault identity is scoped *only* to its own dynamic-DB-credential
lease, not to any tenant secret path at all — a compromised `workflow-service`
pod cannot read GitHub tokens even if it wanted to" (`06-secrets-vault-architecture.md:56-57`).
This is a **stronger** guarantee than the old `credentials.enc` per-user
AES-256-GCM file: a compromised pod that happens to be mid-request for user
A cannot read user B's secrets even transiently, because it never had Vault
policy access to that path at all — regardless of which user's OS process
(if any) it would have run in under the old model.

### 3. Compute/crash isolation — horizontal scaling + statelessness, not process supervision

`auth-service.md` §8 (`auth-service.md:259-261`) states the target directly:
"Horizontally scaled, stateless except for the DB." Every service in
`02-microservices-decomposition.md`'s catalog follows this shape. The
consequence for what BL-AUTH-03 called "crash isolation": a panic or crash
inside a request handler affects only that in-flight request (Go's
per-goroutine panic recovery, standard in every gRPC server here), and a
whole-*pod* crash is handled by the orchestrator (Kubernetes) restarting the
pod, with load balancing already routing new/retried requests to the
remaining healthy replicas — a mechanism that requires zero application code
per service, unlike the old model's bespoke idle-timeout/spawn-timeout/
respawn-with-backoff/abandon-and-alert state machine, because there is no
long-lived per-user process to supervise in the first place. This is a
**genuine, honest tradeoff to name explicitly** rather than imply is a
strict improvement in every dimension: two different users' requests can
execute concurrently within the same OS process/address space on the same
replica, which a `fork()`-per-user model prevented by construction (hard
OS-level memory isolation). backend-go accepts this tradeoff in exchange for
horizontal scalability, relying on (a) Go's memory safety (no
arbitrary-pointer cross-goroutine corruption class of bug), (b) statelessness
(no cross-request mutable state to leak between users sharing a replica),
and (c) secret material never resident in process memory beyond a single
call's lifetime (§2 above) to make that shared-address-space fact low-risk
rather than eliminating it.

### 4. Filesystem isolation — no per-user directory; worktrees scoped by ID in Postgres + per-host paths

BUG-AUTH-03 itself confirms this: "worktrees in backend-go live in
`project-service`'s Postgres-backed `worktree_repository.go`, addressed by
`tenant_id`/`project_id`, not a per-user filesystem path" (BUG-AUTH-03 line
32). `git-gateway-service` (`02-microservices-decomposition.md:68`) resolves
a worktree's owning host and either executes locally or relays to that
host's Dev Server Agent — filesystem access is scoped by `worktree_id`
resolved through `project-service`, never by a `~/.orca/users/<userId>/`
convention. There is deliberately no per-user data directory in this
architecture at all.

### 5. Service-to-service transport — mTLS, not a Unix socket

`07-security-architecture.md`'s "Service-to-service transport security"
section (`07-security-architecture.md:14-22`) specifies mTLS between every
internal service via a service mesh, with `api-gateway` as the only
externally-reachable listener and default-deny `NetworkPolicy` everywhere
else. This replaces the old per-user Unix domain socket's "only this user's
process can read/write this socket" guarantee with "only mesh-authenticated
services can call each other at all," a system-wide rather than per-user
boundary — appropriate given there is no per-user process on the other end
of a call to protect in the first place.

## Proposed rewrite of `docs/logic/auth/BL-AUTH-03-per-user-sandbox.md`

Structure to replace the current fork/socket/data-directory content:

```markdown
# BL-AUTH-03: Per-User Isolation (superseded — see below)

**Status:** Superseded by backend-go's stateless tenant-scoped isolation
model, 2026 backend-go migration. This document previously described a
per-user OS process (fork()), Unix domain socket, and
`~/.orca/users/<userId>/` data directory model from the TS system. That
mechanism has no backend-go equivalent by design, not by omission — see
"Why this changed" below.

## What isolation guarantee this actually provides now

backend-go replaces the single per-user-process mechanism with four
independent, narrower guarantees, each verifiable on its own:

1. **Data isolation** — database-per-service + application-layer `tenant_id`/
   `user_id` filtering (primary) + Postgres RLS (secondary) + OPA policy
   input always carrying the JWT/session-resolved tenant ID, never a
   client-asserted one. See `specs/backend-go/tdd/architecture/07-security-architecture.md`
   "Multi-tenancy isolation."
2. **Secret isolation** — Vault-mediated via `credential-broker-service`;
   no service (including the one handling a given user's request) ever
   holds another user's — or its own user's, longer than one call —
   plaintext secret material at rest. See
   `specs/backend-go/tdd/architecture/06-secrets-vault-architecture.md`.
3. **Compute/crash isolation** — horizontal scaling + statelessness +
   Kubernetes pod restart, not process supervision. Accepted tradeoff:
   requests from different users may share a process's address space on the
   same replica (no OS-level memory boundary between them) — mitigated by
   Go's memory safety, statelessness, and secret non-residency (#2), not
   eliminated.
4. **Filesystem isolation** — no per-user directory; worktrees are scoped
   by `worktree_id`/`project_id` through `project-service`, executed
   locally or relayed to a Dev Server Agent by `git-gateway-service`.

## Why this changed

backend-go's target architecture (`specs/backend-go/tdd/architecture/02-microservices-decomposition.md`)
is a horizontally-scaled, stateless microservice fleet — every service is
"stateless except for the DB" (`auth-service.md` §8) and independently
scaled. A per-user long-lived forked process is structurally incompatible
with that model: it can't be horizontally scaled behind a load balancer
the way a stateless request handler can, and it reintroduces exactly the
kind of per-instance mutable state the rewrite was designed to eliminate.
This is a considered architecture decision, verified against the TDD's
actual security/secrets/decomposition design (not just an unexplored gap) —
see SOL-AUTH-03 in `specs/backend-go/bugs/logic-v1/solutions/` for the full
verification.

## What is NOT preserved, and is an accepted tradeoff

Hard OS-level memory isolation between concurrently-executing users'
requests on the same replica is not preserved. This is deliberate — see
"Compute/crash isolation" above for the mitigating factors and why this
tradeoff was accepted in exchange for horizontal scalability.
```

## What this solution does NOT propose

- No `fork()`/child-process usecase, port, or adapter.
- No per-user Unix socket or WS↔socket proxy.
- No per-user data directory.
- No idle-timeout/spawn-timeout/respawn supervision loop.
- No `backend-go` code changes of any kind — this is scoped entirely to the
  `docs/logic/auth/` business-logic documentation set.

If a genuine product requirement later emerges for OS-level compute
isolation per tenant (e.g. a compliance requirement stronger than the
layered-defense model above), that is a new architecture proposal against
`02-microservices-decomposition.md`'s service catalog (most plausibly a
dedicated sandboxed-execution service, not a retrofit onto every existing
stateless service) — out of scope for this bug, which is about correcting
the documentation to match the already-decided target architecture.

## Test plan

N/A — documentation-only change. No code paths are added or modified.
Verification is textual: `docs/logic/auth/BL-AUTH-03-per-user-sandbox.md`'s
"What's missing" checklist (fork, Unix socket, data directory, idle/spawn
timeout, respawn logic, admin alert) should no longer appear as unaddressed
gaps in `specs/backend-go/bugs/` once the doc is updated, since the doc will
no longer assert that mechanism as the target.

## References

- `specs/backend-go/bugs/logic-v1/BUG-AUTH-03-per-user-sandbox-not-implemented.md` — the exhaustive-search finding and the credential-isolation partial-substitute observation this solution builds on
- `specs/backend-go/tdd/architecture/07-security-architecture.md:14-22` (service-to-service mTLS), `:54-66` (multi-tenancy isolation, four layers)
- `specs/backend-go/tdd/architecture/06-secrets-vault-architecture.md:17-29` (what goes in Vault vs. Postgres), `:31-58` (`credential-broker-service`'s mediation role, Vault policy scoping)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:11-27` (design principles 1-2, database-per-service), `:77-108` ("What's deliberately not a separate service" — the same reasoning pattern this solution's "no per-user process" conclusion follows)
- `specs/backend-go/tdd/services/auth-service.md:259-261` (§8 "Horizontally scaled, stateless except for the DB")
- `docs/logic/auth/BL-AUTH-03-per-user-sandbox.md` — the file this solution proposes rewriting
