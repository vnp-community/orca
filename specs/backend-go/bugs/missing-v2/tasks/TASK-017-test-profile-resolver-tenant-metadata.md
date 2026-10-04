# TASK-017: Tests for `TenantProfileResolver.GetResolvedProfile`'s tenant-metadata forwarding fix

**From Solution:** SOL-008
**Priority:** P1
**Service:** `project-service`
**File:** `internal/adapter/grpcclient/profile_resolver_test.go` (new — confirmed no test file existed in this package before this task)
**Depends on:** TASK-016
**Status:** `[x]` DONE — added `profile_resolver_test.go` with `TestTenantProfileResolver_GetResolvedProfile_ForwardsTenantMetadata` (asserts `metadata.FromOutgoingContext` carries `grpcmw.MetadataTenantID`), `TestTenantProfileResolver_GetResolvedProfile_NoTenantInContext` (fail-closed before the RPC), and `TestTenantProfileResolver_GetResolvedProfile_NoAllowedServerTags` (existing branch, unchanged coverage). **Proven to actually regression-test the bug**: reverted TASK-016's fix locally, confirmed `TestTenantProfileResolver_GetResolvedProfile_ForwardsTenantMetadata` fails (`expected outbound call to carry gRPC metadata`) against the pre-fix code, then restored the fix — not just "it compiles." `go test ./services/project-service/...` clean.

---

## Context

TASK-016 makes `TenantProfileResolver.GetResolvedProfile` call
`withTenantMetadata(ctx)` before its outbound RPC. This needs the same two
test shapes this package's sibling adapters presumably already have for
their own `withTenantMetadata` call sites — confirm the exact existing
pattern in `workflow_execution_checker_test.go`/`task_execution_checker_test.go`
(if they exist) before writing new assertions from scratch, per this
directory's established discipline of grounding tests in real precedent
rather than guessing shapes.

## Changes to make

- Add a test that calls `TenantProfileResolver.GetResolvedProfile` with a
  `ctx` carrying **no** tenant ID (i.e. `tenant.RequireTenantID` would fail)
  → assert the returned error wraps `withTenantMetadata`'s error (message
  prefix `"grpcclient: profile resolver: "`), and that the fake/mock
  `tenantv1.TenantServiceClient`'s `GetResolvedProfile` was never called
  (mirrors the "fails closed before making the call" shape this bug's
  root cause depends on).
- Add a test that calls it with a valid tenant ID in `ctx` → assert the
  fake client received a context whose outgoing gRPC metadata
  (`metadata.FromOutgoingContext`) contains `grpcmw.MetadataTenantID` set to
  the expected tenant ID — the actual regression test for BUG-008 (this is
  exactly what was missing before TASK-016's fix).
- Existing behavior (JSON unmarshal of `resolved_settings_json`,
  `AllowedServerTags` presence/absence branches) is unchanged by this task —
  do not duplicate coverage for that if it already exists elsewhere; only
  add the two cases above.

## Verify

```bash
cd backend-go
go test ./services/project-service/internal/adapter/grpcclient/... -count=1 -v -run 'TestTenantProfileResolver'
go test ./services/project-service/... -count=1
```

Expected: all new tests pass, no regression in the rest of the package.
