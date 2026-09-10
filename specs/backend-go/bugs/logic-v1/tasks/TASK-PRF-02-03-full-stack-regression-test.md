# TASK-PRF-02-03: Full-stack regression test combining both new merge rules with the existing merge

**From Solution:** SOL-PRF-02
**Priority:** P1
**Service:** `tenant-service`
**File:** `backend-go/services/tenant-service/internal/usecase/get_resolved_profile_test.go`
**Depends on:** TASK-PRF-02-01, TASK-PRF-02-02
**Status:** `[x]` DONE — combined regression test added to get_resolved_profile_test.go; full tenant-service suite green

---

## Context

TASK-PRF-02-01/02 each add unit coverage inside `profile_resolution_test.go`
for their own new merge step in isolation. This task adds one case in the
existing usecase-level test file that exercises a resolve call combining
both new rules with the pre-existing merge (security lock + pathAdditions +
approvedModels fallback + allowedServerTags intersection all in one call) —
a regression guard against a future edit to `ResolveProfile`'s call order
breaking either new step's interaction with the others or with `lockSecurity`/
`mergePathAdditions`/`mergeMCPServers`.

## Changes to make

In `backend-go/services/tenant-service/internal/usecase/get_resolved_profile_test.go`,
add a test case (follow this file's existing fixture-construction style for
`CompanyRepository`/`DepartmentRepository`/`TeamRepository`/`UserProfileRepository`
fakes) that sets up:

- Company settings: `security.sessionTimeoutHours`, `shell.pathAdditions`,
  `agent.approvedModels: ["claude-opus-4-5"]`, `fleet.allowedServerTags:
  ["gpu","eu"]`.
- Department settings: `shell.pathAdditions` (a second entry, to verify
  concatenation still works), `fleet.allowedServerTags: ["gpu"]` (narrowing).
- User settings: `agent.preferredModel: "gemini"` (not in company's
  approved list — should trigger the fallback).

Assert on the `GetResolvedProfile` usecase's output:
- `security` section present and equals company's value only (existing
  behavior, unaffected).
- `shell.pathAdditions` contains both company's and department's entries, in
  that order (existing behavior, unaffected).
- `agent.preferredModel == "claude-opus-4-5"` (fallback fired) and
  `agent._modelFallbackReason` is set.
- `fleet.allowedServerTags == ["gpu"]` (department narrowed company's set).
- `_sources["agent.preferredModel"] == "company"` (overwritten by the
  fallback, not `"user"` despite user having set it).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/tenant-service/...
go test ./services/tenant-service/internal/usecase/... -run GetResolvedProfile -v
go test ./services/tenant-service/... -v
```

Expected: this new case passes alongside every existing
`get_resolved_profile_test.go` case, and the full tenant-service suite
(including TASK-PRF-02-01/02's unit tests) is green.
