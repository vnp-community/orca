# workflow-service

Owns workflow templates and executions — see
[`specs/backend-go/services/workflow-service.md`](../../../specs/backend-go/services/workflow-service.md)
for the full design. Built following the exact package layout and
conventions demonstrated by
[`usage-service`](../usage-service/README.md), this repository's Phase 0
reference implementation.

## What's implemented

- `internal/domain/` — `WorkflowTemplate`/`WorkflowExecution` entities with
  invariant-enforcing constructors, the `StepType`/`StepResult`/
  `StepExecutor` strategy-pattern types, `DAGDefinition` parsing +
  structural validation, and a real, tested boolean expression evaluator
  for the Condition step type (`EvaluateCondition`: `==`/`!=`/`&&`/`||`
  over a flat key-value context map, no `eval`, fail-safe-false on
  unparseable input — see `domain/condition.go`'s doc comment for the full
  grammar). Pure unit tests throughout, no I/O.
- `internal/usecase/` — `CreateTemplate`, `Execute`, `GetExecution`,
  `PauseExecution`, `ResumeExecution`, and `ExecuteAdHocStep` (the port
  `automation-service`'s `RunNow` calls, per §3.1 — resolves a `StepType`
  to its registered `StepExecutor` and runs it directly, no
  throwaway-template indirection). Each tested against in-memory fakes; the
  Pause/Resume tests specifically exercise the running->paused/paused->running
  state-machine invariants (e.g. resuming a non-paused execution is
  rejected).
- `internal/adapter/postgres/` — real `pgx`-backed repository implementing
  both `TemplateRepository` and `ExecutionRepository`.
- `internal/adapter/stepexecutors/` — the five `StepExecutor`
  implementations:
  - **Condition** (real) — wraps `domain.EvaluateCondition`.
  - **Webhook** (real) — native `net/http`, with SSRF hardening: rejects
    unsupported schemes, resolves the target and rejects private/loopback/
    link-local IPs by default, supports a config-driven hostname allowlist
    (`WEBHOOK_ALLOWLIST_HOSTS`, empty by default) whose entries are trusted
    outright, disables redirect-following outright (rather than
    re-validating each hop), and bounds response size (1 MiB) and request
    time (client timeout).
  - **Agent / Shell / Notification** (stubs) — return
    `"not implemented — relays to infra-fleet-service execution plane, see
    workflow-service.md"`. These three step types are dispatched here but
    actually executed on the Dev Server Agent's execution plane, reached
    only through `infra-fleet-service`'s relay client (§2) —
    `infra-fleet-service` is itself a stub-based dependency in this
    scaffolding effort, so a real implementation isn't possible yet.
- `internal/adapter/grpc/` — implements the generated
  `workflowv1.WorkflowServiceServer` (`CreateTemplate`, `Execute`,
  `GetExecution`, `PauseExecution`, `ResumeExecution`,
  `ExecuteAdHocStep` — the RPC surface the generated proto actually
  exposes; the design doc's fuller sketch in §3, e.g.
  `ListTemplates`/`ResolveTemplate`/`CancelExecution`/
  `StreamExecutionEvents`, isn't in the generated proto and so isn't
  implemented here).
- `migrations/0001_init.{up,down}.sql` — `workflow.templates` (id,
  tenant_id, name, dag_json, scope) and `workflow.executions` (id,
  template_id, tenant_id, status, root_trace_id, paused_at) with RLS
  policies, matching usage-service's pattern.
- `cmd/server/main.go` — composition root: config load, Postgres pool,
  `StepExecutorRegistry` wired with all five step types (two real, three
  stubs), gRPC server with the shared interceptor chain, health/readiness
  HTTP server, graceful shutdown on SIGTERM.

## Running locally

```sh
# from backend-go/
docker compose up -d postgres   # see ../../docker-compose.yml
migrate -path services/workflow-service/migrations \
  -database "$DATABASE_DSN" up  # golang-migrate; see architecture/05

cd services/workflow-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/workflow?sslmode=disable \
WEBHOOK_ALLOWLIST_HOSTS=hooks.example.com \
  go run ./cmd/server
```

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/, adapter/stepexecutors/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

## Known gaps / follow-ups (tracked, not silently skipped)

- **DAG execution doesn't do real topological wave-dispatch.** `Execute`
  parses `dag_json`, validates it (every `dependsOn` resolves, no step
  depends on itself), and records a new execution in `status=running` —
  matching workflow-service.md §7's diagram up through "persist execution".
  It does **not** implement Kahn's-algorithm wave computation, general
  (multi-node) cycle detection, or dispatch of any step to a
  `StepExecutor` as part of a template-driven run. A recorded execution
  never progresses past `running` on its own. `domain.DAGDefinition.Validate`
  only catches direct self-references and unresolved dependency ids, not
  longer cycles.
- **Agent/Shell/Notification step executors are stubs**, pending a real
  `infra-fleet-service` relay client (see above) — every call returns a
  clearly-labeled "not implemented" error rather than silently no-op'ing.
- **Webhook SSRF allowlist is a basic stub**, not the full §9 posture:
  redirects are refused outright instead of being re-validated per hop,
  there's no per-tenant allowlist (one process-wide list from config), and
  the IP-range block only covers `net.IP`'s loopback/link-local/private/
  unspecified predicates — not, e.g., IPv4-mapped-IPv6 edge cases or a
  cloud metadata-endpoint-specific denylist.
- **`ExecuteAdHocStep` doesn't persist a synthetic execution row.** Per
  §3.1, a real ad hoc step run should create one `executions` row (and,
  in the fuller schema, a `step_executions` row) so it gets the same
  observability/resumability/history as a template-driven step. This
  scaffold's `ExecuteAdHocStep` usecase only resolves the `StepExecutor`
  and returns its result — no persistence. Extend before
  `automation-service` depends on ad hoc run history existing.
- **Template inheritance (company -> team -> personal chain,
  `ResolveTemplate`) is not implemented** — `WorkflowTemplate.Scope` is a
  validated enum field, but there's no parent-chain resolution, no
  `parent_template_id` column, and no depth-bounded recursive query (§6's
  planned `sqlc` hand-written `WITH RECURSIVE` query). The data model is
  narrowed to exactly the columns this build's instructions named
  (id/tenant_id/name/dag_json/scope for templates;
  id/template_id/tenant_id/status/root_trace_id/paused_at for executions) —
  extend both schema and usecase together when inheritance is needed.
- **No boot-time recovery scan (§8).** The `idx_workflow_executions_resumable`
  index exists, but nothing on startup re-attaches to in-flight
  `running`/`paused` executions' `root_trace_id` or re-dispatches pending
  steps — there's no step dispatch to resume in the first place yet (see
  the DAG-execution gap above).
- **No event publishing wired.** Unlike usage-service's
  `internal/adapter/eventbus/`, this service doesn't publish
  `workflow.execution.started`/`completed`/`workflow.step_failed` (§7) —
  add it alongside real wave dispatch, once there's a state transition
  worth telling `notification-service` about.
- **No `sqlc` codegen wired** — same rationale as usage-service:
  `adapter/postgres/repository.go` is hand-written SQL via `pgx`.
- **`common/secrets` (Vault) and `common/tracing`'s OTLP exporter are not
  wired**, same as usage-service — see that service's README for the
  general rationale; not repeated here.

## Deviation from the design doc worth flagging explicitly

`specs/backend-go/services/workflow-service.md` §4 places the
`StepExecutor` interface in `internal/usecase/`, per
`architecture/03-clean-architecture-guidelines.md`'s general "interface
lives with its consumer" rule. This build's instructions place it in
`internal/domain/` instead, reasoning that the strategy contract itself
(`Execute(ctx, stepConfigJSON string) (StepResult, error)`) is a pure
domain concept independent of any adapter, even though four of five
concrete implementations need I/O. `usecase.StepExecutorRegistry` — the
port that *resolves* a `StepType` to a `StepExecutor` — still lives in
`usecase/`, matching the general rule for the one piece that's genuinely
usecase-shaped. See `domain/step.go`'s doc comment for the in-code note.
