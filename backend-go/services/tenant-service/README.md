# tenant-service

Owns the identity hierarchy above the individual user — companies,
departments, per-user profile overrides, and teams — plus the one piece of
derived, non-persisted business logic: the profile-resolution deep merge.
See [`specs/backend-go/services/tenant-service.md`](../../../specs/backend-go/services/tenant-service.md)
for the full design, and
[`usage-service`](../usage-service/README.md) for the reference
implementation whose package layout and conventions this service follows.

Per ADR-021, `tenant.companies.id` **is** the `tenant_id` every other
service's schema logically references — this is the origin service.

## What's implemented

- `internal/domain/` — `Company`, `Department`, `Team`/`TeamMember`,
  `UserProfile` entities with invariant-enforcing constructors, plus
  `ResolveProfile` (`profile_resolution.go`): the 4-layer deep-merge
  algorithm (company < department < teams, ascending priority < user), pure
  and I/O-free, with the "security" section locked to the company layer,
  `shell.pathAdditions` concatenated additively, and `mcp.servers`
  deduplicated by name. Extensively unit tested, including nil-department,
  empty-user-override, priority-tiebreak, and security-lock-bypass-attempt
  cases.
- `internal/usecase/` — `CreateCompany`, `ValidateTenant`,
  `CreateDepartment`, `SetUserDepartment`, `GetResolvedProfile` (fetch all
  four layers, call the pure domain merge), `CreateTeam`, `AddTeamMember`,
  `ListTeamMembers` — one usecase per `tenant.proto` RPC. `GetResolvedProfile`
  is wrapped by `CachedGetResolvedProfile`, a usecase-layer **decorator**
  (not baked into `adapter/postgres`) implementing the in-process
  LRU-with-TTL cache design from tenant-service.md §6. `SetUserDepartment`
  and `AddTeamMember` each call `ProfileCache.Invalidate` for exactly the
  user they affect before returning success, per §8's invalidation-
  correctness requirement. All tested against in-memory fakes, including
  `GetResolvedProfile`'s merge behavior end-to-end through the usecase layer
  (four layers, priority tiebreak, cache-hit short-circuit) and cross-tenant
  not-found scoping for `SetUserDepartment`/`AddTeamMember`.
- `internal/adapter/postgres/` — real `pgx`-backed repositories, one per
  aggregate (`CompanyRepository`, `DepartmentRepository`, `TeamRepository`,
  `UserProfileRepository`) per tenant-service.md §6's package-layout note,
  hand-written SQL (see `architecture/04-tech-stack.md` — `sqlc` codegen is
  the eventual target). Every department/team/user-profile query is scoped
  by `company_id` in the same query, never filtered after the fact, per §9.
- `internal/adapter/cache/` — `LRUTTLCache`: a mutex-guarded, fixed-capacity
  LRU cache with lazy per-entry TTL expiry, implementing `usecase.ProfileCache`.
- `internal/adapter/grpc/` — implements the generated
  `tenantv1.TenantServiceServer`, pure wire<->usecase translation.
- `migrations/0001_init.{up,down}.sql` — real DDL: `tenant.companies`,
  `tenant.departments`, `tenant.user_profiles`, `tenant.teams`,
  `tenant.team_members`, RLS policies keyed on `company_id`. `companies` has
  no `tenant_id`/RLS of its own (it *is* the tenant root).
- `cmd/server/main.go` — a real, working composition root: config load,
  Postgres pool, the LRU-TTL cache wired as `GetResolvedProfile`'s decorator
  and as the invalidation target for `SetUserDepartment`/`AddTeamMember`,
  gRPC server with the shared interceptor chain, health/readiness HTTP
  server, graceful shutdown on SIGTERM. No NATS/eventbus wiring —
  tenant-service publishes no events and makes zero outbound synchronous
  service calls (tenant-service.md §7).

## Running locally

```sh
# from backend-go/
docker compose up -d postgres   # see ../../docker-compose.yml
migrate -path services/tenant-service/migrations \
  -database "$DATABASE_DSN" up  # golang-migrate; see architecture/05

cd services/tenant-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/tenant?sslmode=disable \
  go run ./cmd/server
```

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/, adapter/cache/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

## Known gaps / follow-ups (tracked, not silently skipped)

- **Profile-resolution cache is in-process only, not shared across
  replicas** — per tenant-service.md §6's explicit design choice (an
  in-process LRU-with-TTL cache, not a shared Redis read-through cache: the
  cached object is small, per-user, and cheap to recompute on a miss). Each
  replica has its own cache, so an update on replica A doesn't invalidate
  replica B within the 60s TTL window — an accepted staleness bound
  carried forward from the TS system's process-local cache, not a bug. The
  documented upgrade path if 60s staleness proves unacceptable is a
  distributed invalidation broadcast (publish `Invalidate` on this
  service's own outbox/NATS subject, consumed by every replica) — not built
  by default. **This is a real scaling consideration**: at higher replica
  counts or stricter staleness SLOs, revisit this trade-off before it
  becomes a correctness issue rather than a documented staleness bound.
- **`tenant.proto`'s current RPC surface is a reduced subset** of the design
  doc's full sketch (§3): no `UpdateCompany`/`UpdateDepartment`/`UpdateTeam`/
  `UpdateUserProfile`/`ListDepartments`/`RemoveTeamMember`/
  `InvalidateProfileCache` RPCs exist yet, so:
  - `Team.settings_json` has a column and a `domain.Team.Settings` field
    (needed for `ResolveProfile`'s team layer) but no RPC currently sets it
    — every team's settings layer is `{}` until an `UpdateTeam` RPC exists.
  - There's no RPC to set a `UserProfile`'s own `Settings` override
    directly — only `SetUserDepartment` (department assignment). The user
    layer is populated implicitly whenever a `user_profiles` row exists.
  - `AddTeamMemberRequest` has no `role` field (only `priority`); the
    `role` column defaults to `'member'` for every row.
  - `departments.parent_department_id` (the design doc's self-referencing
    tree, §5) isn't modeled in `domain.Department` at all, since nothing in
    the current proto surface exercises it.
  Extend `proto/orca/tenant/v1/tenant.proto` and this service's
  usecase/adapter layers together as more of the surface is needed.
- **No `sqlc` codegen wired** — `adapter/postgres/*_repository.go` is
  hand-written SQL via `pgx`, matching usage-service's scaffold.
- **`common/secrets` (Vault) is not wired into this service's `main.go`** —
  `DATABASE_DSN` is read directly from the environment for local dev; wire
  the Vault-Agent-rendered-credentials-file path before this service is
  deployed anywhere Vault is actually running. tenant-service.md §7 notes
  this is a dedicated-cluster database (like `auth`/`credential`), so this
  gap matters more here than for a lower-tier service.
- **`common/tracing` has no OTLP exporter configured** — spans are created
  but not shipped anywhere until a collector endpoint is wired in.
- **Cross-tenant isolation relies on application-layer scoping plus RLS as
  a secondary backstop, both implemented, but there is no adversarial
  fuzzing/property test suite yet** beyond the explicit not-found test
  cases for `SetUserDepartment`/`AddTeamMember`/`ListTeamMembers`/
  `DepartmentRepository`/`TeamRepository`. tenant-service.md §9 calls this
  out as a first-class test requirement, not an afterthought — worth
  expanding before this service handles real multi-tenant traffic.
