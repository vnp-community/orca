# automation-service

See [`specs/backend-go/services/automation-service.md`](../../../specs/backend-go/services/automation-service.md)
for the full design and
[`../usage-service/README.md`](../usage-service/README.md) for the Phase 0
pilot every service's package layout follows.

## What makes this service different

`automation-service` owns schedule/trigger definitions and run history, but
it **never executes a step itself** — `RunNow` delegates to
`workflow-service.ExecuteAdHocStep` over a real gRPC call. This is the whole
point of the service's design, not an incidental detail: TS's
`AutomationService.runNow` never had a working dispatcher wired
server-side, so every triggered run resolved `skipped_unavailable` in
production. This scaffold closes that gap for real.

**This is the one service in the current scaffold set where the key
cross-service call is REAL, not stubbed.** `internal/adapter/grpcclient/workflow_client.go`
constructs an actual `grpc.ClientConn` to `workflow-service` (dialed once in
`cmd/server/main.go`, address from `WORKFLOW_SERVICE_ADDR`) and calls
`workflowv1.NewWorkflowServiceClient(conn).ExecuteAdHocStep(...)` for real —
it imports the actual generated `workflow-service` proto types
(`github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1`), not a
hand-rolled interface pretending to be one. Both services' proto contracts
already exist in this repo (`proto/orca/automation/v1`,
`proto/orca/workflow/v1`), which is what makes a real client possible here
even though `workflow-service` isn't actually running when this service's
own tests execute in isolation — `internal/usecase`'s tests fake the
`WorkflowStepExecutor` port instead of standing up a live `workflow-service`.
A true end-to-end integration test (this service's real gRPC client talking
to a real running `workflow-service`) would need docker-compose wiring for
both services — out of scope for this scaffold's automated test suite, but
the client code that would make that call is real and compiles against the
real generated types today.

## What's implemented

- `internal/domain/` — `Automation`/`AutomationRun` entities with
  invariant-enforcing constructors and an explicit run status state machine
  (`pending -> running -> succeeded|failed`), plus `RecurrenceRule` — a pure
  value object over `github.com/teambition/rrule-go` implementing
  `NextOccurrenceAfter`/`LatestOccurrenceAtOrBefore` per the design doc's
  §4. `go get github.com/teambition/rrule-go` fetched cleanly and covers
  every recurrence feature the design doc calls out (frequency, interval,
  byweekday, count/until), so the hand-rolled "every N hours" fallback the
  task allowed for was not needed.
- `internal/usecase/` — `CreateAutomation`, `RunNow` (the core interactor),
  `ListRuns`, each tested against in-memory fakes, no real Postgres/gRPC
  needed. `RunNow`'s tests use a fake `WorkflowStepExecutor` to verify it's
  called with the right `step_config_json`/`step_type`/`tenant_id`/
  `request_id`, that a retried call with the same `request_id` returns the
  existing run without a second dispatch, and that a workflow-service
  failure is recorded as a `failed` run rather than silently swallowed.
- `internal/adapter/postgres/` — real `pgx`-backed repositories for both
  `automations` and `automation_runs`.
- `internal/adapter/grpcclient/workflow_client.go` — the real
  `WorkflowStepExecutor` implementation described above.
- `internal/adapter/grpc/` — implements the generated
  `automationv1.AutomationServiceServer`, pure wire<->usecase translation.
- `migrations/0001_init.{up,down}.sql` — `automation.automations`,
  `automation.automation_runs`, RLS policies, and the `(tenant_id,
  request_id)` unique index on `automation_runs` — the same idempotency
  pattern usage-service uses for `(tenant_id, request_id)` on `sessions`.
- `cmd/server/main.go` — composition root: config load, Postgres pool, a
  real `grpc.NewClient` connection to `workflow-service`, gRPC server with
  the shared interceptor chain, health/readiness HTTP server (including a
  `workflow-service` connection-state readiness check), graceful shutdown.

## Running locally

```sh
# from backend-go/
docker compose up -d postgres   # see ../../docker-compose.yml
migrate -path services/automation-service/migrations \
  -database "$DATABASE_DSN" up  # golang-migrate; see architecture/05

cd services/automation-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/automation?sslmode=disable \
WORKFLOW_SERVICE_ADDR=localhost:9091 \
  go run ./cmd/server
```

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

## Deviations from the design doc (and why)

The generated proto (`proto/orca/automation/v1/automation.proto`, which this
service must not regenerate) is deliberately smaller than the design doc's
§3/§4/§5 sketch — it ships 3 RPCs (`CreateAutomation`, `RunNow`,
`ListRuns`) and a minimal `Automation` message (`id`, `tenant_id`, `name`,
`rrule`, `step_config_json`), not the full CRUD surface or the richer
`Automation`/`AutomationRun` field sets (`project_id`, `agent_id`,
`execution_target`, `next_run_at`, `workflow_execution_id`, etc.) the design
doc describes. This scaffold builds strictly against the proto that exists,
per the task's constraints, and notes the gaps here rather than silently
matching the doc's aspirational shape:

- **`tenant_id` on the wire is ignored in favor of context.** Both
  `Automation` and `CreateAutomationRequest` carry a `tenant_id` field, but
  per `architecture/05-data-architecture.md`'s tenant-isolation rule
  ("never trusted from the request body"), `CreateAutomation`'s usecase
  pulls `tenant_id` from `context` (populated by `grpcmw`'s tenant-extraction
  interceptor from caller metadata) — the same pattern
  `usage-service.RecordUsageSession` uses. `RunNow`/`ListRuns`'s requests
  don't carry `tenant_id` at all, consistent with this.
- **`step_type` travels inside `step_config_json`, not as a separate proto
  field.** `workflow-service.ExecuteAdHocStepRequest` has a `StepType` enum
  field, but `automation.Automation`/`RunNowRequest` have nowhere to carry
  it. `domain.ParseStepType` reads a `"step_type"` key out of the JSON body
  instead (`internal/domain/automation.go`), defaulting to `agent` if absent
  or unrecognized. A future proto revision should probably promote this to
  a first-class field once `CreateAutomationRequest` grows one.
- **No `dtstart`/`timezone`/`next_run_at`/scheduler loop.** The design doc's
  §7 in-process ticker loop (`adapter/scheduler/`) and the `next_run_at`/
  `missed_run_policy` columns it needs are not implemented — `RunNow` (the
  Gap 3 fix this design exists for) is fully real, but nothing calls it on a
  timer yet. `dtstart` is stored (defaulted to the automation's
  `created_at` timestamp in usecase.CreateAutomation, since
  `CreateAutomationRequest` has no `dtstart` field) purely so
  `RecurrenceRule`'s pure functions are usable/testable now; wiring the
  scheduler is future work once `next_run_at` has a column and RPC path to
  land in.
- **No `project-service` call.** §7/§9 describe `RunNow` re-resolving the
  owning user's current project access via `project-service` before every
  dispatch. `project-service`'s generated client isn't wired into this
  scaffold — `RunNow` here trusts the caller's already-validated tenant
  context (same trust boundary every other service in this scaffold set
  uses) rather than re-checking authorization per dispatch. Flagged, not
  silently dropped: revisit once `project-service`'s gRPC surface is stable
  enough to depend on.
- **No `HandleExternalTrigger` RPC.** The design doc explicitly flags this
  RPC's wire contract as pending review of TS's
  `external-automations-handler.ts` (§7) — not implemented here for the same
  reason, and the generated proto doesn't define it either.
- **`apperrors.Kind` has no `Unavailable` value.** §8 asks for `RunNow` to
  "fail closed with `UNAVAILABLE`" when `workflow-service` is unreachable.
  `common/apperrors` (which this service must not modify) only defines
  `KindNotFound`/`KindAlreadyExists`/`KindInvalidArgument`/
  `KindPermissionDenied`/`KindFailedPrecondition`/`KindUnauthenticated`/
  `KindInternal`. A workflow-service transport failure is mapped to
  `KindInternal` with `Code: "AUTOMATION_WORKFLOW_UNAVAILABLE"` instead —
  the gRPC status code ends up `codes.Internal` rather than
  `codes.Unavailable`, but the run is still recorded `failed` and the error
  is never swallowed, which is the behavioral guarantee §8 actually cares
  about.
- **No `sqlc` codegen wired**, same as usage-service — hand-written `pgx`
  SQL is a valid destination per `architecture/04-tech-stack.md` but not
  yet the codegen-checked default path.
- **`common/secrets` (Vault) and `common/tracing`'s OTLP exporter** are not
  wired here either, matching usage-service's own noted gaps.
