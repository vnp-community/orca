# TASK-BE-008: gRPC + wscompat Wiring for BE-SOL-002/003/004

> **Status: ✅ COMPLETED** — 2026-08-28
> **Files modified:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
> (+ regenerated `.pb.go`/`_grpc.pb.go` via `buf generate`),
> `internal/adapter/grpc/server.go` (12 new RPC handlers + view-struct
> converters), `cmd/server/main.go` (wiring)
> **Files created (api-gateway):**
> `internal/adapter/wscompat/channels_dev_server_access_control.go`
> (+ `*_test.go`)

**Solutions:** BE-SOL-002/003/004 | **CRs:** CR-DS-006/007/008
**Depends on:** TASK-BE-005/006/007

---

## Goal

Real proto RPCs (not hand-written pb.go) for all 12 new operations, wired
through infra-fleet-service's gRPC server into the corresponding usecases,
then exposed to the frontend as 12 wscompat channels on api-gateway.

## Key correctness finding, fixed in this task

Every new wscompat channel converts through an explicit camelCase view
struct (`devServerGroupView`, `devServerGroupGrantView`,
`devServerAccessRequestView`; `devServerView` extended with
`approvalStatus`/`groupId`) — **never a raw `*infrafleetv1.X` proto
message**. protoc-gen-go's `encoding/json` struct tags are snake_case
(e.g. `json:"tenant_id,omitempty"`); the wscompat envelope serializes
`Result any` via plain `encoding/json`, not `protojson`, so a raw proto
message would silently ship snake_case keys a camelCase-typed frontend
would never see populated. See solutions/README.md's "What shipped in
this pass" section for the wider note (this may affect other pre-existing
channels too — not fixed here, flagged for later).

## New channels

`devServer.approve`, `.reject`, `.assignGroup`, `.listForUser`,
`.requestAccess`, `.listPendingAccessRequests`, `.resolveAccessRequest`;
`devServerGroup.create`, `.list`, `.grant`, `.revoke`, `.listGrants`.

`devServer.listForUser`/`.requestAccess` resolve the caller's department
via `tenant-service.GetUserProfile` at the gateway edge (matching
`worktree.detectedList`'s "multi-service view assembled at api-gateway,
not inside either owning service" precedent) — infra-fleet-service never
gains a dependency on tenant-service.

## Acceptance Criteria

- [x] `buf generate` regenerates cleanly; `go build ./...` clean across
      all 17 backend-go services both immediately after the proto change
      and after all Go code landed.
- [x] Admin-gated channels attach `Identity.Role` via a new
      `attachAdminIdentity` helper; every other existing wscompat channel
      in the package is unmodified (still omits Role).
- [x] `devServer.requestAccess` fails closed when the caller has no
      department (`departmentID == ""`) — tested.
- [x] List-shaped channels return `[]` not `null` when empty (established
      convention from this session's earlier `repo.list`/`devServer.listSshTargets`
      fixes) — tested for `devServerGroup.list`.
- [x] Full `api-gateway` test suite passes; full `go build`/`go test` clean
      across all 17 backend-go services.
