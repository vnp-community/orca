# Clean Architecture Guidelines

Every one of the 18 services (see
[`02-microservices-decomposition.md`](./02-microservices-decomposition.md))
follows the same internal layering. This is not optional per-service — a
production-readiness gate in
[`standards/production-readiness-checklist.md`](../standards/production-readiness-checklist.md)
checks for it. The goal: business rules are testable without a database,
without a network call, and without Go's `context.Context` even, and
swapping Postgres/Vault/gRPC for something else touches only the outermost
layer.

## The dependency rule

Dependencies point **inward only**. Nothing in an inner layer imports
anything from an outer layer.

```
┌─────────────────────────────────────────────────────┐
│  adapter (delivery + infrastructure)                 │
│  ┌─────────────────────────────────────────────┐    │
│  │  usecase (application services)              │    │
│  │  ┌───────────────────────────────────┐       │    │
│  │  │  domain (entities, value objects)  │       │    │
│  │  └───────────────────────────────────┘       │    │
│  └─────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────┘
        outward-pointing arrow = compile-time import
```

## Standard package layout

```
<service>/
├── cmd/
│   └── server/main.go              # composition root: wires everything, nothing else
├── internal/
│   ├── domain/                     # entities, value objects, domain errors, domain events
│   │   ├── task.go                 #   NO imports outside stdlib + other domain/ packages
│   │   └── task_test.go            #   unit tests need no mocks — pure functions/methods
│   ├── usecase/                    # application services ("interactors"); one file per use case
│   │   ├── create_task.go          #   depends on domain/ + ports it defines itself
│   │   ├── ports.go                #   interfaces the usecase layer needs (Repository, EventPublisher, ...)
│   │   └── create_task_test.go     #   tested against in-memory fakes of the ports, no real DB/network
│   ├── adapter/
│   │   ├── grpc/                   # inbound: gRPC service implementation, proto<->domain mapping
│   │   ├── http/                   # inbound: REST handlers where a service exposes REST directly (rare — usually via gateway)
│   │   ├── postgres/               # outbound: implements usecase's Repository port against Postgres (sqlc-generated + hand-written)
│   │   ├── vault/                  # outbound: implements usecase's SecretStore port against Vault
│   │   ├── eventbus/               # outbound: implements usecase's EventPublisher port against NATS
│   │   └── external/               # outbound: clients for GitHub/GitLab/Jira/etc. (only in the services that need them)
│   └── config/                     # env/flag parsing, typed Config struct — no business logic
├── migrations/                     # golang-migrate SQL files, this service's DB only
├── proto/                          # this service's .proto definitions (or imported from a central buf module — see 04)
├── deploy/                         # Dockerfile, Helm values overrides
└── go.mod                          # each service is its own Go module
```

## Layer contracts

### `domain/`

- Entities (`Task`, `WorkflowExecution`, `AIProviderAccount`, …) and value
  objects, with their invariants enforced in constructors/methods — not
  validated later in a handler.
- Domain errors as typed values (`ErrTaskNotFound`, `ErrCyclicDependency`),
  not raw `fmt.Errorf` strings — the adapter layer maps these to gRPC status
  codes / HTTP status codes, one place, not scattered.
- **Zero imports** from `usecase/`, `adapter/`, any framework, any driver.
  If a domain file needs to import `database/sql` or a gRPC-generated type,
  it's in the wrong layer.

### `usecase/`

- One exported type per use case (or a small cohesive group), each with a
  single `Execute(ctx, input) (output, error)`-shaped method — mirrors the
  granularity of today's RPC methods (`CreateTask`, `ResolveTaskGrant`,
  `PromoteReadyTasks`) so the TS→Go mapping in
  [`migration/domain-capability-service-mapping.md`](../migration/domain-capability-service-mapping.md)
  stays traceable method-for-method.
- Defines the **ports** (Go interfaces) it needs — `TaskRepository`,
  `EventPublisher`, `Clock` — in this layer, not in `adapter/`. This is the
  Dependency Inversion half of Clean Architecture: the interface lives with
  its consumer, the implementation lives in the outer layer that satisfies
  it.
- `context.Context` is allowed here (for cancellation/tracing propagation)
  but usecases must not reach into it for business data — pass explicit
  arguments.

### `adapter/`

- **Inbound adapters** (`grpc/`, `http/`) translate wire format → usecase
  input, call the usecase, translate output → wire format. No business
  logic — if an inbound handler has an `if` statement deciding business
  behavior (not just translating an error to a status code), that logic
  belongs in `usecase/`.
- **Outbound adapters** (`postgres/`, `vault/`, `eventbus/`, `external/`)
  implement the ports `usecase/` defined. A Postgres repository adapter
  knows SQL; it does not know what a "grant" means beyond mapping rows to
  domain structs.
- This is where `sqlc`-generated code, `pgx` pool handling, Vault SDK calls,
  and third-party HTTP clients live — see
  [`04-tech-stack.md`](./04-tech-stack.md).

### Quy ước "không có -> NotFound" (repo, usecase, test)

Một id lạ phải ra `NotFound` (404), không bao giờ là 5xx; một lỗi hạ tầng (DB sập, timeout) phải ra
`Internal`, không bao giờ là 404. Hai lớp chia việc như sau (TASK-E2E-017):

1. **`domain/`** khai báo sentinel cụ thể cho từng thực thể (`ErrTaskNotFound`, `ErrShareLinkNotFound`).
   Không dùng một `ErrNotFound` chung cho cả service.
2. **Repo (`adapter/postgres`, `adapter/mysql`)** trả sentinel bọc `%w` ở **cả hai dialect** khi
   `sql.ErrNoRows`/`pgx.ErrNoRows` hoặc `RowsAffected() == 0` (Update/Delete/Revoke/Set). Cấm
   `fmt.Errorf("... not found")` hoặc `errors.New("... not found")` không có sentinel, và cấm bọc
   `ErrNoRows` thành lỗi chung: lớp trên không phân loại được nên id lạ thành 5xx.
3. **Usecase** ánh xạ bằng `errors.Is` (không dùng `==`, không dùng "mọi lỗi -> NotFound"):
   sentinel -> `apperrors.KindNotFound` với mã lỗi riêng của service, còn lại -> `apperrors.KindInternal`.
4. **Postgres, cột `uuid`**: id sai định dạng (ví dụ `"abc"`) bị server từ chối với SQLSTATE `22P02`
   (hoặc FK `23503` khi insert tham chiếu tới id không tồn tại). Id đó không trỏ tới hàng nào nên repo
   coi như not-found. MySQL lưu id dạng chuỗi nên chỉ cần nhánh `ErrNoRows`/`affected == 0`.
5. Ngoại lệ có chủ ý (ví dụ `MarkAsRead`, `Revoke` grant, `DeleteByEndpoint`) là idempotent: 0 hàng
   không phải lỗi. Ghi rõ trong doc comment của port để không bị "sửa" nhầm.

```go
// repo (cả postgres và mysql)
if errors.Is(err, pgx.ErrNoRows) || isMalformedUUID(err) {
    return domain.Task{}, fmt.Errorf("postgres: task %s: %w", id, domain.ErrTaskNotFound)
}

// usecase
if errors.Is(err, domain.ErrTaskNotFound) {
    return apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
}
return apperrors.New(apperrors.KindInternal, "TASK_FETCH_FAILED", "failed to fetch task", err)
```

**Cách test** (mỗi chỗ sửa có một test hồi quy):

- `usecase/`: bọc repo fake bằng một struct trả lỗi cố định rồi kiểm hai nhánh -- sentinel bọc `%w`
  -> `KindNotFound` đúng mã, lỗi khác (`errors.New("connection refused")`) -> `KindInternal`. Mẫu:
  `task-service/internal/usecase/task_not_found_contract_test.go`.
- `adapter/*`: test tích hợp (`-tags=integration`, testcontainers) gọi từng phương thức Get/Update/
  Delete/Revoke với một id hợp lệ-nhưng-không-tồn-tại và một id sai định dạng, khẳng định
  `errors.Is(err, domain.ErrXxxNotFound)`. Mẫu: `task-service/internal/adapter/{postgres,mysql}/
  not_found_integration_test.go`.
- Hợp đồng xuyên service (id lạ qua gateway -> 404): suite `notfound` trong `tests/stages/e2e/orca-flows`.

### `cmd/server/main.go`

- The only place allowed to know about *every* layer at once: reads config,
  constructs adapters, constructs usecases with those adapters injected,
  constructs inbound handlers with those usecases injected, starts the
  server. No manual DI framework required at this scale — plain
  constructor-injection is enough and keeps startup order explicit and
  debuggable.

## Cross-service shared code policy

A shared Go module (`orca-go-common`, versioned and published internally,
**not** a `replace` directive to a local path in production builds) holds
only truly cross-cutting, business-logic-free concerns:

- Observability middleware (structured logging, OpenTelemetry instrumentation, request-ID propagation)
- Error taxonomy shared at the gRPC boundary (a common `AppError` → `status.Status` mapping)
- Auth/tenant-context primitives (extracting the validated tenant/user from
  an incoming gRPC context — mirrors TS's `TenantContext`)
- Config-loading helpers, health-check/readiness-probe boilerplate

**Explicitly not shared:** domain types, repository interfaces, or
usecases. If two services seem to want the same domain type, that's a signal
either the service boundary is wrong (revisit
[`02-microservices-decomposition.md`](./02-microservices-decomposition.md))
or it's a legitimate case for a "logical FK" + an API call, not a shared
struct. Sharing domain models across service boundaries is exactly the
coupling database-per-service is meant to prevent.

## Testing implications of this layout

See [`standards/testing-strategy.md`](../standards/testing-strategy.md) for
the full policy; the layering above is what makes it possible:

- `domain/` — pure unit tests, no mocks needed.
- `usecase/` — unit tests against hand-written or `mockgen`-generated fakes
  of the ports; no real Postgres/Vault/NATS.
- `adapter/postgres/` — integration tests against `testcontainers-go`
  Postgres, verifying the port contract is actually satisfied.
- `adapter/grpc/` — contract tests against the `.proto` definition.
- End-to-end — a small number of cross-service scenarios run against a full
  `docker-compose` stack in CI, not per-service.
