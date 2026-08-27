# TASK-140: Tests for `projectGroup.*` (project-service usecases/repository + wscompat channels)

**From Solution:** SOL-021
**Priority:** P1
**Service:** `project-service`, `api-gateway`
**File:** `internal/domain/project_group_test.go`, `internal/usecase/move_project_test.go`, `scan_nested_test.go`, `import_nested_test.go` (all new, `project-service`); `internal/adapter/postgres/project_group_repository_test.go`; `services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-136, TASK-137, TASK-138, TASK-139
**Status:** `[x]` DONE — wscompat channel tests verified complete (7/7: all projectGroup.* channels incl. moveProject/scanNested/importNested, in `channels_tenant_project_test.go`, including the deadline>rpcTimeout and field-for-field NestedRepoCandidate mapping assertions the doc calls out); usecase/postgres coverage done in prior pass (postgres integration tests still blocked on no live Postgres in this environment).

---

## Tests to add

### `project-service/internal/domain/project_group_test.go`

- `TestParseNestedRepoCandidates_DecodesWireShape` — decodes a sample
  `result_json` payload, asserts field mapping.
- `TestParseNestedRepoCandidates_EmptyCandidatesIsNotError` — `{"candidates":[]}`
  decodes to an empty (not nil-panicking) slice.
- `TestNewProjectGroup_ProjectIDFieldRoundTrips` — a leaf group
  (`ProjectID` set) still round-trips through `NewProjectGroup` unaffected
  (constructor signature unchanged by this task).

### `project-service/internal/usecase/move_project_test.go`

- `TestMoveProject_RejectsNonexistentTargetGroup` — fake
  `GetProjectGroup` returns not-found; asserts `apperrors.KindNotFound`,
  asserts `UpsertLeafGroupForProject` never called.
- `TestMoveProject_RejectsOtherTenantsTargetGroup` — fake repo scoped by
  tenant returns not-found for a group id from a different tenant; same
  assertions.
- `TestMoveProject_CreatesLeafGroupWhenNoneExists` — fake
  `UpsertLeafGroupForProject` is the only call needed (find-or-create is
  the repository's job, not the usecase's) — asserts the usecase doesn't
  branch on "does a leaf group already exist."
- `TestMoveProject_DeniesNonOwnerActor` — fake OPA denies
  `projectActionOwnerOnly`.

### `project-service/internal/usecase/scan_nested_test.go`

- `TestScanNested_CallsCreateConnectionThenRelay` — fake `DevServerRelay`:
  asserts `CreateConnection` is called with `(devServerID, rootPath, "")`,
  then `Relay` with method `"fs.scanNestedRepos"` and `{"path":
  rootPath}`-shaped params.
- `TestScanNested_RelayErrorFailsClosed` — fake `Relay` returns an error;
  asserts `apperrors.KindInternal` and no candidates returned (no local
  fallback — matches the "no `if (connectionId) return []` shortcut"
  correctness bar `infra-fleet-service.md` §10 sets).
- `TestScanNested_CreateConnectionErrorMapsToFailedPrecondition` — fake
  `CreateConnection` returns an error (simulating an unknown
  `dev_server_id` — the only validation this usecase performs, per
  TASK-138's Context); asserts `apperrors.KindFailedPrecondition`.

### `project-service/internal/usecase/import_nested_test.go`

- `TestImportNested_CreatesOneGroupAndProjectPerCandidate` — fake
  `ProjectGroupRepository.ImportNested` returns N groups/projects for N
  selected candidates; asserts passthrough.
- `TestImportNested_RejectsNonexistentParentGroup`.
- `TestImportNested_NoTenantIsUnauthenticated`.

### `project-service/internal/adapter/postgres/project_group_repository_test.go` (`testcontainers-go`)

- `TestProjectGroupRepository_UpsertLeafGroupForProjectIsIdempotent` — call
  twice with different `targetParentGroupID`s; asserts exactly one row
  exists for that `project_id` (the partial unique index from
  migrations/0006), with the second call's `parent_group_id` winning.
- `TestProjectGroupRepository_ImportNestedRollsBackOnPartialFailure` — seed
  candidates where one insert is engineered to fail (e.g. a duplicate id
  forced via a pre-existing row); assert **zero** groups/projects/repos
  from that call exist afterward — the transactional guarantee
  `usecase.ImportNested`'s doc comment promises.
- `TestProjectGroupRepository_ImportNestedCreatesLinkedRepo` — asserts the
  created project has exactly one repo, with `url` set to the candidate's
  `path`.

### `api-gateway/internal/adapter/wscompat/channels_test.go`

7 tests (4 wiring-only + 3 new):

- `TestProjectGroupCreateChannel_Success`
- `TestProjectGroupUpdateChannel_Success`
- `TestProjectGroupDeleteChannel_Success`
- `TestProjectGroupListChannel_Success`
- `TestProjectGroupMoveProjectChannel_Success`
- `TestProjectGroupScanNestedChannel_UsesLongerTimeout` — asserts (via a
  fake client that inspects the incoming context's deadline) the deadline
  is > `rpcTimeout` — the exact assertion SOL-021's own test plan calls
  out.
- `TestProjectGroupImportNestedChannel_Success` — asserts `selected` array
  args map onto `projectv1.NestedRepoCandidate` field-for-field.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/project-service
go test ./internal/domain/... ./internal/usecase/... ./internal/adapter/postgres/... -count=1 -v

cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -run "TestProjectGroup" -v -race

cd /opt/repos/orca/backend-go
go build ./... && go vet ./...
```
