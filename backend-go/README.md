# orca-go

Go implementation of Orca's backend, per the target design in
[`specs/backend-go/`](../specs/backend-go/). **Status: scaffold** — every
service builds, vets, and tests green, and the architecturally-critical
pieces (Clean Architecture layering, database-per-service, Vault-mediated
secrets, one real cross-service gRPC call, one real event-driven consumer)
are genuinely implemented and tested. Most services' full business-logic
surface is not yet complete — see each service's own README for exactly
what's real vs. stubbed, and
[`docs/execution-plan.md`](./docs/execution-plan.md) for the ordered plan to
take this from scaffold to production.

## Layout

```
backend-go/
├── go.work                  # ties all 22 modules together for local dev (common, proto, cmd/orca-cli, 19 service dirs)
├── common/                  # orca-go-common — cross-cutting only, no business logic
├── proto/                   # buf module, orca.<service>.v1 packages, generated Go stubs in proto/gen/go/
├── services/<name>-service/ # one Go module per service, Clean Architecture layout (see any service's internal/ tree)
├── docker-compose.yml       # local dev: postgres (one DB per service, created by deploy/postgres-init-databases.sh, incl. `mcp`), vault (dev mode), nats
├── Makefile                 # build/vet/test/lint/fmt/proto-gen/migrate — see `make help`
└── docs/execution-plan.md   # the detailed task breakdown for finishing this
```

## Quickstart

```sh
make dev-up          # postgres :5432, vault :8200, nats :4222
make build            # go build every module
make test             # go test every module (unit tests, no Docker needed)
make test-integration # go test -tags=integration every module (needs make dev-up first)
```

To run one service against the local stack, see that service's own README
(e.g. [`services/usage-service/README.md`](./services/usage-service/README.md))
for its exact env vars — every service follows the same `DATABASE_DSN` /
`GRPC_PORT` / `HTTP_PORT` convention from `common/config`.

## Why this exists

This is a from-scratch Go rewrite of `backend/`'s TypeScript coordination-plane
role — see [`specs/backend/api/backend-agent-target-architecture.md`](../specs/backend/api/backend-agent-target-architecture.md)
for what that role is, and [`specs/backend-go/README.md`](../specs/backend-go/README.md)
for the full target architecture (microservices decomposition, Clean
Architecture, PostgreSQL, Vault, production-readiness bar) this codebase
implements against. It does not replace anything running today — see
[`specs/backend-go/migration/ts-to-go-migration-strategy.md`](../specs/backend-go/migration/ts-to-go-migration-strategy.md)
for how a real cutover would be sequenced.

## What's genuinely real in this scaffold (not just structure)

- **`usage-service`** — fully-implemented reference service: real Postgres
  repository with RLS + write-idempotency, real NATS event publishing, real
  gRPC server, passes unit + integration tests.
- **`task-service`** — the BFS grant-resolution algorithm and cycle
  detector are real, pure, and covered by ~20 test cases.
- **`orchestration-service`** — the `KeyedSerializer` concurrency primitive
  is real and proven correct under `go test -race` with a genuinely racy
  workload; the atomic task-promotion transaction is real SQL.
- **`credential-broker-service`** — the Vault adapter (Transit/KV) is real,
  calling `common/secrets` for real; a compile-time test proves no
  secret-value field exists anywhere in its schema; audit-before-return
  ordering is tested.
- **`automation-service` → `workflow-service`** — the one fully-real
  cross-service gRPC call in this scaffold: `automation-service`'s `RunNow`
  genuinely dials and calls `workflow-service.ExecuteAdHocStep`.
- **`notification-service`** — the event-bus consumer and in-process
  broadcaster fan-out are real, with per-user/per-tenant isolation tested.
- **`api-gateway`** — the `usage-service` REST reverse-proxy and the
  `notification-service` WebSocket↔gRPC-stream bridge are real end-to-end.
- **`workflow-service`** — the Condition-step boolean evaluator and the
  Webhook-step executor (with SSRF IP-range blocking) are real.
- **`git-gateway-service`** — local git status/diff via `os/exec` against a
  real git binary is real.
- **`mcp-service` + the `/mcp` adapter in `api-gateway`** (the 18th TDD service;
  `services/issue-status-sync` is a worker module outside the TDD list) —
  implemented with unit and integration tests: Streamable HTTP MCP server
  (official Go SDK), OAuth/PAT, tool catalog, policy/approval/kill switch/audit,
  sessions + SSE resume, resources/prompts, external MCP servers, `orca_mcp_*`
  Prometheus metrics, conformance tiers (`ci/mcp-conformance/`). Off by default
  (`MCP_ENABLED=false`). **Not** verified end-to-end against the full compose
  stack; gaps and the rollout runbook are in
  [`services/mcp-service/README.md`](./services/mcp-service/README.md).

- **`request-service`** — Request flow (docs: [`docs/guides/request/`](../docs/guides/request/README.md)).
  Off by default (`REQUEST_FLOW_ENABLED=false` plus a per-tenant setting; both must be on). Real handlers today:
  `RequestService` — `CreateRequest`, `GetRequest`, `ListRequests`, `ClassifyRequest` (async, returns `run_id`), `ConfirmRequestType`,
  `ChangeRequestType`, `ListRequestTypeHistory`, `ReturnToBacklog`, `ReopenRequest`, `CancelRequest`, `SpawnChildRequest`, `ListRequestLinks`,
  `GetRequestFlow`, `LookupRequestBySource` (internal, shared token), `GetRequestFlowSettings`, `SetRequestFlowSettings`;
  `ApprovalService` — `RequestApproval`, `Approve`, `Reject`, `Cancel`, `GetApproval`, `ListApprovals`, `ListPendingForUser`, `ExtendApproval`;
  `ApprovalPolicyAdminService` — `ListApprovalPolicies`, `UpsertApprovalPolicy`, `DeleteApprovalPolicy`.
  **Still `Unimplemented`:** `ListBacklog`, `GenerateSolution`, `ListSolutions`, `ChooseSolutionOption`, `GeneratePlan`, `GetPlanProposal`,
  `CommitPlan`, `StartPhase`, `ReportTaskOutcome`, the clarification/decision/artifact/impact/readiness/context/compliance RPCs, and
  `AiBudgetAdminService`. So the flow runs up to type confirmation (`analyzing` or `planning`); only the `request_type` approval gate has a
  real handler. Not verified against real Jira or a real dev-server agent; the T2 stack-dev e2e has not run on a real stack.

## What's intentionally stubbed (and why that's honest, not incomplete work hidden)

Every service that depends on another not-yet-real service (most
cross-service calls point at `infra-fleet-service`'s Dev Server Agent relay,
which requires porting the existing TS wire protocol — a substantial
standalone effort explicitly out of scope for this pass) has a clearly
commented stub adapter satisfying the right interface, so the calling
service's own logic is real and tested against that interface even though
the live call isn't wired. Every such stub is called out in that service's
own README's "Known gaps" section — read those before assuming a route
works end-to-end. **Do not deploy any service in this repository to a
production environment without first closing the gaps listed in its
README and clearing [`specs/backend-go/standards/production-readiness-checklist.md`](../specs/backend-go/standards/production-readiness-checklist.md).**
