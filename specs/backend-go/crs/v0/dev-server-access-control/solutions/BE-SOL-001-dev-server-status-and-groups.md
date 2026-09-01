# BE-SOL-001: Dev Server Status + Groups — Data Model (Phase 1)

**CR:** [CR-DS-006](../../../../../docs/crs/v2/dev-server/CR-DS-006-dev-server-approval-and-grouping.md)
**Service:** infra-fleet-service
**Status:** ✅ COMPLETED (2026-08-28)

> **Note:** originally authored as migration `0007` with a `status` column;
> renumbered to `0008`/`approval_status` after a deploy-time collision with
> a different, concurrently-in-flight (uncommitted) migration on the shared
> b15 server — see CR-DS-006's own "Cập nhật triển khai" note for details.

---

## 1. Migration

`backend-go/services/infra-fleet-service/migrations/0008_dev_server_approval_status_and_groups.{up,down}.sql`

- New table `infra.dev_server_groups` (`id UUID`, `tenant_id UUID`, `name TEXT`, `parent_group_id UUID` self-FK, RLS `tenant_isolation` policy — same shape as `infra.dev_servers`/`infra.ssh_targets` in `0001_init.up.sql`).
- `infra.dev_servers` gains `approval_status TEXT NOT NULL DEFAULT 'approved' CHECK (... IN ('pending_approval','approved','rejected'))` and `group_id UUID REFERENCES infra.dev_server_groups(id)`. Named `approval_status`, not `status` — `infra.dev_servers` already had an unrelated `status` column (health/bootstrap state) added by a separate, concurrently-in-flight migration discovered mid-deploy; see this doc's own note below and the migration file's header comment.
- Column-level default is `'approved'` (protects any row inserted outside the Go domain layer); the *application* default for freshly-registered dev servers is `'pending_approval'`, set in `domain.NewDevServer` — these are deliberately different, see the migration file's own comment.

## 2. Domain (`internal/domain/`)

- `dev_server.go`: new `DevServerStatus` type (`pending_approval | approved | rejected`) + `Valid()`. `DevServer` struct gains `Status DevServerStatus`, `GroupID string`. `NewDevServer`'s **signature is unchanged** — it now always sets `Status: DevServerStatusPendingApproval` internally. This was a deliberate choice to keep the change additive-only: `gitnexus_impact` on `NewDevServer`/`DevServer` came back CRITICAL (thousands of hits) because both names collide with unrelated symbols across the monorepo (old TS backend files, other backend-go services' own local `run`/`spawn` functions) — verified as a false positive by building all 17 backend-go services (`go build ./...` in each, zero errors) after the change; keeping the constructor's signature stable is what makes that verification valid (no caller needed to change).
- `dev_server_group.go` (new file): `DevServerGroup{ID, TenantID, Name, ParentGroupID}` + `NewDevServerGroup` constructor, same non-empty-tenant/name invariant pattern as `NewDevServer`.

## 3. Usecase (`internal/usecase/`)

- `ports.go`: new `DevServerGroupRepository` interface (`Create`, `List(tenantID)`).
- `create_dev_server_group.go`: `CreateDevServerGroup` usecase — tenant from context (never request body), delegates validation to `domain.NewDevServerGroup`.
- `list_dev_server_groups.go`: `ListDevServerGroups` usecase — tenant-scoped list, no per-group membership check yet (that's CR-DS-007).

**Not added in this pass** (intentionally — no RPC surface yet, nothing calls these usecases from outside the package): no proto changes, no gRPC handler, no wscompat channel. See BE-SOL-002 for the admin-approval RPC pass that will wire these in.

## 4. Postgres (`internal/adapter/postgres/`)

- `repository.go`: `Register`/`Get`/`List`/`FindBySshTarget` extended to read/write `approval_status`/`group_id` alongside existing columns (the Go-level local variable and `domain.DevServer.Status` field are still named `Status` — only the SQL column name is `approval_status`).
- `dev_server_group_repository.go` (new file): `DevServerGroupStore` implementing `DevServerGroupRepository` — split into its own type rather than a `Repository` method, same reason `SshTargetStore` is split (Go method-set collision on `List`/`Create` between different entity types on the same receiver).

## 5. Tests

- `internal/domain/dev_server_test.go`: `TestNewDevServer_DefaultsToPendingApproval`, `TestDevServerStatus_Valid`.
- `internal/domain/dev_server_group_test.go`: `TestNewDevServerGroup_ValidatesInvariants` (table test, mirrors `dev_server_test.go`'s style).
- `internal/usecase/create_dev_server_group_test.go`, `list_dev_server_groups_test.go`: fake-repository unit tests, same shape as `register_dev_server_test.go`.
- `internal/adapter/postgres/dev_server_group_repository_test.go` (integration, `//go:build integration`, Testcontainers): `TestDevServerGroupStore_CreateAndList` (tenant scoping + parent/child round-trip), `TestRepository_RegisterAndGet_PersistsStatusAndGroupID` (guards the exact "field silently dropped between domain struct and SQL" bug class this session found and fixed elsewhere — `repo.list`'s `PROJECT_MEMBERSHIP_LOOKUP_FAILED` investigation).

All new tests pass; full `go test ./...` and `go test -tags=integration ./...` for infra-fleet-service pass (one pre-existing, unrelated flake in `TestRepository_ResolveConnection_FoundAndNotFound` — a documented Testcontainers startup race, reproduced and confirmed to pass in isolation, not caused by this change).

## 6. Checklist

- [x] Migration 0008 up/down, runs clean on Testcontainers Postgres.
- [x] `domain.DevServerStatus` + `DevServer.Status`/`GroupID` fields.
- [x] `domain.DevServerGroup` + constructor.
- [x] `usecase.DevServerGroupRepository` port + `CreateDevServerGroup`/`ListDevServerGroups` usecases.
- [x] `postgres.DevServerGroupStore` + `Repository`'s existing methods extended.
- [x] Unit + integration tests, all passing.
- [x] `go build ./...` clean across all 17 backend-go services (impact-analysis false-positive verification).
- [ ] RPC/wscompat wiring — deferred to BE-SOL-002 (no external caller needs this yet).
