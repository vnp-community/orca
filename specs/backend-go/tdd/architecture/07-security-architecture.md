# Security Architecture

## AuthN

| Client | Mechanism | Issued/validated by |
|--------|-----------|----------------------|
| Browser (frontend SPA) | HTTP-only session cookie, `SameSite=strict`, `Secure` always on (unlike the TS system, which only enabled `Secure` when `NODE_ENV==='production'` — treat that as a bug this redesign doesn't repeat) | Issued by `auth-service`, validated at `api-gateway` on every request before routing |
| Mobile app | Short-lived JWT (RS256) + refresh token, obtained after the existing QR-pairing + TweetNaCl E2E handshake | `auth-service` issues; `api-gateway` validates against a JWKS endpoint `auth-service` publishes |
| CLI / service-to-service (gateway → internal services) | Short-lived JWT, mTLS as a second factor at the network layer | `auth-service` issues user-context JWTs; mTLS identity comes from the service mesh, not from `auth-service` |
| Dev Server Agent | Agent token (bearer), hashed at rest — same shape the TS system already uses correctly (SHA-256 hash, not plaintext) | `infra-fleet-service` |

## Service-to-service transport security

- **mTLS between every internal service**, provisioned by a service mesh
  (Linkerd by default — see [`10-deployment-infrastructure.md`](./10-deployment-infrastructure.md)
  for the Istio alternative and when to choose it). This is not optional for
  "enterprise level": internal gRPC traffic is encrypted and mutually
  authenticated even inside the cluster network, not just at the edge.
- `api-gateway` is the only service with an external-facing listener; every
  other service's only ingress is from the mesh, enforced by
  `NetworkPolicy` (default-deny, explicit allow per the dependency graph in
  [`02-microservices-decomposition.md`](./02-microservices-decomposition.md)).

## AuthZ — replacing the TS system's fragmented RBAC

The TS system has **two independent, non-unified permission mechanisms**
(`resolveUserPermissions()` for fleet/server-level, `TaskGrantService.resolvePermission()`
for task-graph BFS ancestor resolution — `backend-hld-c4.md`'s Cross-Cutting
Concerns section) plus a history of `requireAdmin`/`requireOwnerOrAdmin`
being login-only checks that had to be patched. This redesign doesn't carry
that fragmentation forward:

- **Open Policy Agent (OPA)**, one policy bundle (`orca-authz`), covering
  every authorization decision in the system — admin actions, task grants,
  project membership, workflow execution permission — as Rego policy, not
  scattered `if user.role === 'admin'` checks per service.
- `api-gateway` calls OPA (as a sidecar or embedded via the Go SDK) for
  coarse-grained "can this JWT even call this endpoint" checks before
  routing.
- Each service calls OPA (embedded, in-process — no network hop, policies
  loaded at startup and hot-reloaded on bundle update) for fine-grained,
  domain-specific checks that need data OPA doesn't have context for at the
  gateway (e.g., "does this user have a grant on this specific task,
  considering ancestor inheritance" — the input document passed to OPA
  includes whatever task-graph lookup `task-service` already did; OPA
  evaluates the policy, `task-service` doesn't reimplement the policy logic
  itself).
- **One place to review the entire system's authorization rules**: the Rego
  policy bundle, version-controlled, with its own test suite (`opa test`) —
  directly closing the class of bug that made `requireAdmin` a login-only
  check for as long as it was in the TS system, because that kind of logic
  error is now a policy-test failure, not a silent gap in one handler.

## Multi-tenancy isolation

Layered defense, consistent with
[`05-data-architecture.md`](./05-data-architecture.md):

1. Database-per-service (blast radius of a compromised service is its own
   data, not the whole system).
2. Application-layer `tenant_id` filtering on every query (primary).
3. Postgres RLS (secondary defense).
4. OPA policy input always includes the resolved tenant ID from the
   validated JWT/session — never trusted from request body — so even a
   correctly-authenticated user cannot forge a request for another tenant's
   data by passing a different `tenant_id` in the payload.

## Audit logging

- `auth-service` owns the system-wide audit log (carried forward from the
  TS system's `orca_audit_log`), but every service emits structured audit
  events (via the outbox pattern, same mechanism as domain events) for
  security-relevant actions in its own domain — credential access
  (`credential-broker-service`), permission changes (`auth-service`,
  `task-service` grants), admin actions.
- Audit events are **append-only** and shipped to the observability
  pipeline's log store (see
  [`09-observability-reliability.md`](./09-observability-reliability.md))
  with a longer retention policy than operational logs — a compliance
  requirement for "enterprise level," not just a debugging aid.

## Secrets — see dedicated doc

All secret-handling design (what's in Vault, rotation, the
`credential-broker-service` mediation pattern) is in
[`06-secrets-vault-architecture.md`](./06-secrets-vault-architecture.md) —
not duplicated here.

## Input validation & supply chain

- Every gRPC message validated against `.proto`-declared constraints
  (`buf` + `protovalidate`) before it reaches `usecase/` — the delivery
  layer's job per Clean Architecture, not scattered validation inside
  business logic.
- Dependency scanning (`govulncheck` in CI for every service module),
  container image scanning (Trivy) before any image is promoted past a
  staging environment — part of the
  [production-readiness checklist](../standards/production-readiness-checklist.md).
- SCM/PM integration credentials scoped to the minimum OAuth scope each
  integration actually needs, reviewed per-provider in
  [`services/scm-integration-service.md`](../services/scm-integration-service.md)
  and [`services/issue-tracking-service.md`](../services/issue-tracking-service.md).
