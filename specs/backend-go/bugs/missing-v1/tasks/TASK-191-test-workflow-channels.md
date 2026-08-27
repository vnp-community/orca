# TASK-191: Tests for `UpdateTemplate` usecase, versioned Postgres write, and `workflow.*` `wscompat` channels

**From Solution:** SOL-030
**Priority:** P1
**Service:** `workflow-service` + `api-gateway`
**File:** `internal/usecase/update_template_test.go` (new), `internal/adapter/postgres/repository_test.go` (extend), `services/api-gateway/internal/adapter/wscompat/channels_workflow_test.go` (new)
**Depends on:** TASK-188, TASK-189, TASK-190
**Status:** `[x]` DONE — wscompat `workflow.*` channel tests complete and verified (5/5 pass, all TASK-191 cases including the tenant-spoofing and FAILED_PRECONDITION-passthrough guards); `update_template_test.go` usecase tests spot-checked and already adequate (5/5 pass, all listed cases covered); no REST-vs-wscompat parity harness exists in this package yet (follow-up, not built here); postgres `repository_test.go` integration tests (`TestRepository_Update_CorrectVersion_Succeeds` / `_StaleVersion_ReturnsConflict`) already exist behind the `integration` build tag but were not executed in this pass per task scope.

---

## Changes to make

### `services/workflow-service/internal/usecase/update_template_test.go` (new)

Fake `TemplateRepository`. Cases:

- Happy path: `Execute` calls `Update` with `expectedVersion` forwarded
  unchanged, returns the repository's bumped-version result.
- Stale `ExpectedVersion` → fake `Update` returns
  `domain.ErrTemplateVersionConflict` → `Execute` returns an error whose
  `apperrors` code is `WORKFLOW_TEMPLATE_VERSION_CONFLICT` (assert via
  `apperrors.ToGRPCStatus`'s resulting `codes.FailedPrecondition`, not just
  a non-nil error).
- Setting `ParentTemplateID` to an id that is `in.ID`'s own descendant
  (fake `ResolveChain` returns a chain containing `in.ID`) →
  `WORKFLOW_TEMPLATE_CYCLE`, and fake `Update` is never called (assert call
  count 0 — the cycle check must short-circuit before the write).
- `ParentTemplateID == ""` → `ResolveChain` is never called (no parent to
  validate).
- Updating a template whose `id` a completed execution's
  `DefinitionSnapshot` already references succeeds without touching
  execution state — this usecase never reads `ExecutionRepository` at all;
  assert `UpdateTemplate`'s constructor/`Execute` signature has no such
  dependency (a structural regression guard: if a future edit adds one,
  this test's fake wiring will fail to compile, which is the intended
  signal).

### `services/workflow-service/internal/adapter/postgres/repository_test.go`

Add (testcontainers-go, following this file's existing pattern):

- `TestRepository_Update_CorrectVersion_Succeeds`: insert a template
  (version 1), `Update(ctx, tmpl, 1)` succeeds and the returned
  `Version == 2`; a follow-up `GetTemplate` confirms the row's
  `dag_json`/`name`/`scope`/`parent_template_id` all reflect the update.
- `TestRepository_Update_StaleVersion_ReturnsConflict`: same setup,
  `Update(ctx, tmpl, 99)` (wrong version) returns
  `domain.ErrTemplateVersionConflict` and the row is unchanged (re-`Get`
  still shows version 1 and the original fields).

### `services/api-gateway/internal/adapter/wscompat/channels_workflow_test.go` (new)

One test per `workflow.*` channel against a fake
`workflowv1.WorkflowServiceClient` (follow `channels_test.go`'s existing
fake-client pattern in this package). Cases:

- `workflow.execute` → calls `Execute` with the decoded args, returns
  `GetExecution()`.
- `workflow.cancel` → calls `CancelExecution`, returns `GetExecution()`.
- `workflow.template.create` → calls `CreateTemplate` with
  `TenantId: id.TenantID` set from `Identity`, not from the decoded args
  (regression guard: a malicious/buggy frontend payload setting a
  different `tenantId` in `args` must not leak into the outbound request).
- `workflow.template.update` → calls `UpdateTemplate`; a gRPC
  `FAILED_PRECONDITION` status from the fake client (simulating the
  version-conflict path) surfaces as the handler's returned error
  unmodified — assert the error is not swallowed or replaced with a
  generic message.

### Contract test (optional but recommended, mirrors SOL-001's REST/wscompat parity guard)

If this package already has a REST-vs-wscompat parity test harness (check
for one testing `/admin/api/*` against `admin.*` wscompat channels from an
earlier task pass) — extend it with `workflow.execute`/`.cancel`/
`.template.create` vs. the equivalent `/v1/workflows/*` REST calls,
asserting both resolve to the same RPC and produce structurally identical
response shapes. If no such harness exists yet, skip this and note it as a
follow-up rather than building a new one from scratch in this task.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/workflow-service
go test ./internal/usecase/... ./internal/adapter/postgres/... -count=1 -v

cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -count=1 -v
```
