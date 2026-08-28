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
