# Go Coding Standards

Applies to all 17 services and `orca-go-common`. Enforced by
`golangci-lint` in CI (config shared via `orca-go-common`'s
`.golangci.yml`, not reinvented per service) — this document explains the
*why* behind the rules the linter enforces mechanically; it doesn't restate
every lint rule.

## Style

- `gofmt`/`goimports` non-negotiable, run in a pre-commit hook and CI gate.
- Package names: short, lowercase, no underscores, no `util`/`common`/`helpers`
  package names — same "name after what it contains" principle this repo's
  own [`AGENTS.md`](../../../AGENTS.md) already states for the TS codebase,
  carried forward for Go (`taskgraph`, not `taskutils`; `pgcred`, not
  `dbhelpers`).
- Exported identifiers get doc comments starting with the identifier name
  (standard Go convention, enforced by `golint`/`revive`).

## Error handling

- Errors are values, wrapped with `fmt.Errorf("...: %w", err)` to preserve
  the chain — never swallowed, never logged-and-ignored without a reason
  documented at the swallow site.
- Domain errors are typed (`var ErrTaskNotFound = errors.New(...)` or a
  custom error type when structured data is needed), defined in
  `domain/`, checked with `errors.Is`/`errors.As` — never string-matching
  an error message.
- The adapter layer (specifically the gRPC inbound adapter) is the *only*
  place a domain error gets mapped to a wire-level status code
  (`codes.NotFound`, `codes.PermissionDenied`, …) — see
  [`../architecture/03-clean-architecture-guidelines.md`](../architecture/03-clean-architecture-guidelines.md).
  This mapping table lives in `orca-go-common` so every service maps the
  same domain-error categories to the same gRPC codes consistently.
- No `panic` in request-handling paths except for truly unrecoverable
  programmer errors (nil pointer from a bug, not a validation failure) —
  and even then, the shared gRPC interceptor recovers it and converts to
  `codes.Internal` rather than crashing the process, so one bad request
  can't take down a pod mid-flight for other in-progress requests.

## Concurrency

- `context.Context` is the first parameter of every function that can block
  (DB call, gRPC call, Vault call) — no exceptions, no background goroutine
  started without a context derived from either the request or an explicit
  service-lifetime context that's cancelled on shutdown.
- Goroutines started outside a request's lifecycle (background workers,
  outbox relay, event consumers) are managed by a supervisor in
  `cmd/server/main.go` that ensures graceful shutdown — `SIGTERM` drains
  in-flight requests (respecting Kubernetes' termination grace period)
  before exiting, not an abrupt process kill.
- Shared mutable state protected by `sync.Mutex`/`sync.RWMutex` or, more
  often, avoided entirely by preferring channel-based coordination or
  per-request-scoped state — a service handling concurrent tenant traffic
  must not have any implicit shared-state coupling between requests from
  different tenants.

## Dependency injection

- Plain constructor injection (`NewTaskUsecase(repo TaskRepository, bus
  EventPublisher) *TaskUsecase`) — no reflection-based DI framework
  (`wire`, `fx`) required at this scale; `main.go` wires the graph
  explicitly and readably. If a service's wiring genuinely grows complex
  enough to justify `wire` (compile-time DI codegen), that's an acceptable
  per-service exception, not a system-wide default.

## Testing conventions

Full policy in
[`testing-strategy.md`](./testing-strategy.md); the coding-level
convention: table-driven tests as the default shape for anything with more
than 2-3 cases, `t.Parallel()` wherever tests don't share mutable state, and
every exported `usecase/` type has a corresponding `_test.go` — a
production-readiness gate checks per-service coverage, not just presence.

## What NOT to do

- No `interface{}`/`any`-typed domain data passed across layer boundaries —
  every port is a concrete, purpose-built interface (`TaskRepository`, not
  `Repository[T any]` used generically for every entity — generics are fine
  *within* a layer for genuinely generic utility code, not as a substitute
  for explicit domain modeling).
- No global mutable state (package-level `var` holding a DB pool, a Vault
  client, etc.) — everything is constructed in `main.go` and passed down
  explicitly. This is what makes the `usecase/` layer testable without a
  real database in the first place.
- No business logic in `adapter/grpc/` handlers beyond request/response
  translation — see the Clean Architecture doc's layer contracts.
