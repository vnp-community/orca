# TASK-145: Tests for `projectHostSetup.*` (project-service usecases/repository + wscompat channels)

**From Solution:** SOL-022
**Priority:** P1
**Service:** `project-service`, `api-gateway`
**File:** `internal/domain/host_setup_test.go`, `internal/usecase/create_host_setup_test.go`, `setup_existing_folder_test.go` (all new, `project-service`); `internal/adapter/postgres/host_setup_repository_test.go` (new); `services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-141, TASK-142, TASK-143, TASK-144
**Status:** `[ ]` TODO

---

## Tests to add

### `project-service/internal/domain/host_setup_test.go`

- `TestHostSetupStatus_Valid` — table test over all 4 known values plus an
  unknown string.
- `TestNewHostSetup_StartsAtPending` — asserts the constructed status is
  always `HostSetupPending`, regardless of any caller intent.
- `TestNewHostSetup_RejectsEmptyRequiredFields` — table test over
  `tenantID`/`devServerID`/`folderPath`/`createdBy` each empty in turn.

### `project-service/internal/usecase/create_host_setup_test.go`

- `TestCreateHostSetup_ValidatesDevServerID` — fake `DevServerLister`:
  `Exists` returns `false`; asserts `apperrors.KindInvalidArgument` and
  that `repo.Create` is **never** called (no row written for an unknown
  dev server).
- `TestCreateHostSetup_PersistsOnValidDevServer` — `Exists` returns
  `true`; asserts the created setup's `Status` is `HostSetupPending`.
- `TestCreateHostSetup_DevServerLookupErrorIsInternal` — `Exists` returns
  an error (not `false`); asserts `apperrors.KindInternal`, distinct from
  the not-found case above.

### `project-service/internal/usecase/setup_existing_folder_test.go`

- `TestSetupExistingFolder_PathCheckFailureMarksFailedNoProjectCreated` —
  fake `DevServerRelay.Relay` returns a `{"exists":false}` result; asserts
  the setup transitions to `HostSetupFailed` and **no** `Project` is
  created (`uc.projects.Create` never called).
- `TestSetupExistingFolder_RelayErrorMarksFailed` — `Relay` itself errors
  (transport failure, not a negative check result); same "no project
  created" assertion.
- `TestSetupExistingFolder_SuccessCreatesExactlyOneProjectAndRepo` — fake
  `Relay` returns `{"exists":true,"isDir":true}`; asserts exactly one
  `Project` and one `Repo` created, and the setup ends `HostSetupCompleted`
  with `ProjectID` set to the new project's id.
- `TestSetupExistingFolder_RejectsAlreadyCompleted` — setup fetched with
  `Status: HostSetupCompleted`; asserts
  `apperrors.KindFailedPrecondition` and no relay call made at all (the
  guard fires before any I/O).
- `TestSetupExistingFolder_NeverStatsLocalFilesystem` — the regression
  this solution exists to prevent: assert no code path in this usecase
  calls anything resembling `os.Stat`/`os.ReadDir` on `folder_path`. A fake
  `DevServerRelay` that is the *only* path-existence source (no local
  filesystem fake wired in at all) enforces this structurally — the test
  passes only if the usecase never reaches for a filesystem port that
  doesn't exist in its dependency list.

### `project-service/internal/adapter/postgres/host_setup_repository_test.go` (`testcontainers-go`)

- `TestHostSetupRepository_TenantIsolation` — a setup created for tenant A
  is `ErrHostSetupNotFound` when looked up with tenant B's id.
- `TestHostSetupRepository_ProjectIDSetNullOnProjectDelete` — after
  `Complete`, delete the referenced project row directly; assert the host
  setup's `project_id` becomes NULL (the `ON DELETE SET NULL` FK behavior
  from migrations/0007), not a foreign-key error.
- `TestHostSetupRepository_UpdatePartialPatchLeavesOtherFieldUnchanged`.
- `TestHostSetupRepository_StatusCheckConstraintRejectsUnknownValue` — a
  raw `INSERT ... status = 'bogus'` violates the migration's `CHECK`
  constraint (defense in depth below the Go-level `Valid()` check).

### `api-gateway/internal/adapter/wscompat/channels_test.go`

5 tests, following the established `fake*Client` pattern:

- `TestProjectHostSetupCreateChannel_Success`
- `TestProjectHostSetupListChannel_Success`
- `TestProjectHostSetupUpdateChannel_Success`
- `TestProjectHostSetupDeleteChannel_Success`
- `TestProjectHostSetupSetupExistingFolderChannel_UsesLongerTimeout` —
  asserts (via a fake client inspecting the incoming context's deadline)
  the deadline is > `rpcTimeout`, same assertion shape as
  `TestProjectGroupScanNestedChannel_UsesLongerTimeout` (TASK-140).
- `TestProjectHostSetupSetupExistingFolderChannel_ReturnsProjectOnlyOnSuccess` —
  a fake response with `Setup.Status == "failed"` and `Project == nil`;
  assert the channel returns the raw response passthrough (the handler
  itself does no status branching — that's the gRPC layer's job per
  TASK-143 Step 9).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/project-service
go test ./internal/domain/... ./internal/usecase/... ./internal/adapter/postgres/... -count=1 -v

cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -run "TestProjectHostSetup" -v -race

cd /opt/repos/orca/backend-go
go build ./... && go vet ./...
```
