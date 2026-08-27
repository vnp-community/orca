# TS → Go Migration Strategy

## Strategy: strangler fig, not a big-bang rewrite

The TS `backend/` stays the system of record and continues serving
production traffic throughout. Go services are stood up one at a time,
behind the *existing* `api-gateway`-equivalent (initially, the TS RPC
dispatcher itself, modified to proxy specific namespaces to a Go service
instead of handling them in-process) — the same pattern
[ADR-021 §5](../../../docs/adrs/v2/ADR-021-unified-postgres-microservices-platform.md)
already proposed for the TS system's own eventual service extraction
("mỗi namespace handler trở thành 1 thin proxy"), reused here as the
cutover mechanism from TS to Go specifically.

**Never**: run this as a single flag-flip cutover of the whole system. At
17 services and this much surface area, a big-bang rewrite is both a
correctness risk (subtle TS behavior not replicated) and a rollback risk
(no way to revert one thing without reverting everything).

## Phase sequencing

Ordered by risk (lowest first) and dependency (leaf services before the
services everything depends on) — same ordering principle ADR-021 already
used for its own Phase 3, applied here:

### Phase 0 — Foundations (no user-facing change)

- Stand up shared infrastructure: Vault, NATS, the `orca-go-common` module,
  CI/CD pipeline, one reference service built end-to-end (recommend
  `usage-service` — lowest risk, no dependents, matches ADR-021's own pilot
  choice for the equivalent TS extraction).
- Validate the full path: Go service ↔ its own Postgres ↔ Vault-issued
  credentials ↔ observability stack, in `dev` and `staging`, before
  touching anything with real traffic.

### Phase 1 — Leaf services (low risk, few/no dependents)

Build and cut over, one at a time, with a validation soak period between
each:

1. `usage-service`
2. `annotation-service`
3. `notification-service`
4. `issue-tracking-service`

Cutover mechanism per service: TS `backend/`'s RPC handler for that
namespace becomes a thin proxy to the new Go service's gRPC API (via
`grpc-gateway` or a direct gRPC client from Node — either works, pick
whichever has less TS-side plumbing). Data migration: one-time backfill
script, TS system's existing table(s) → Go service's new Postgres database,
run and verified against `staging` before touching `production` data.
**Dual-write period**: for each service, run TS write path AND proxy-forward
to Go in parallel for a bounded window (recommend 1–2 weeks), comparing
outputs, before flipping reads to the Go service and retiring the TS write
path for that namespace.

### Phase 2 — Mid-tier domain services

5. `ai-provider-service` + `credential-broker-service` together (they're
   interdependent — build both before cutting either over, since
   `ai-provider-service`'s credential-write path needs
   `credential-broker-service` to exist)
6. `automation-service`
7. `workflow-service`
8. `infra-fleet-service`
9. `project-service`
10. `task-service`
11. `orchestration-service`

Same dual-write-then-cutover mechanism per service, in this order because
each depends on services already cut over in earlier phases (`task-service`
depends on `tenant-service` for grant resolution — but `tenant-service`
isn't cut over yet at this point in the plan; **during the transition
period, an already-Go service is allowed to call back into the still-TS
`backend/` for a dependency that hasn't cut over yet**, via a thin
compatibility client — this is expected and temporary, not a design flaw,
and should be tracked explicitly per service as "still depends on legacy
TS for X" until that dependency's own cutover lands).

### Phase 3 — Gateway-facing services

12. `git-gateway-service`
13. `scm-integration-service` — this is also where TS Gap 1 (self-executed
    CLI, no per-tenant isolation) gets fixed, not carried forward broken;
    build the correct direct-API-client version from the start rather than
    porting the CLI-shelling behavior and fixing it later

### Phase 4 — Identity/tenancy (highest risk, do last)

14. `tenant-service`
15. `auth-service`

Everything else already depends on these by this point — cutting them over
last, after every dependent service has already proven its Go
implementation correct against the *old* auth/tenant system, minimizes the
number of moving parts changing at once for the highest-blast-radius
services in the system.

### Phase 5 — `api-gateway` cutover + TS retirement

- Once every service is running in Go and stable, replace the TS RPC
  dispatcher entirely with the real `api-gateway` (frontend/mobile/CLI now
  talk to it directly, no TS proxy layer in between).
- Decommission `backend/`'s TS codebase for server-mode deployment.
  **Electron desktop mode is unaffected** — it doesn't route through this
  migration at all (out of scope per this doc set's own scope statement in
  the README).

## Cutover criteria (per service, every phase)

A service is not considered cut over until:

1. Dual-write comparison shows zero unexplained divergence over the soak
   window.
2. Load test (k6) at production-comparable traffic passes the service's
   stated SLOs.
3. [Production-readiness checklist](../standards/production-readiness-checklist.md)
   fully green.
4. Rollback plan tested — proxy flag flips back to TS handling with no data
   loss (this is why dual-write runs *before* cutover, not just during: the
   TS write path must still be current if a rollback happens).
5. On-call runbook exists for the new service.

## What does NOT get ported as-is

Per [`domain-capability-service-mapping.md`](./domain-capability-service-mapping.md)
and the decomposition doc's "deliberately not a separate service" section:
`ai-vault.*`, `browser.*`/`computer.*`/`emulator.*` are not mechanically
carried forward — each needs an explicit product decision before Phase 1-5
work touches them, otherwise they're left running on the legacy TS system
indefinitely (acceptable outcome if no one decides otherwise) or dropped
from the server deployment.
