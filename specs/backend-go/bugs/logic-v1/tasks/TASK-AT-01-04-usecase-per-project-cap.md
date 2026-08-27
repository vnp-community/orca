# TASK-AT-01-04: Enforce 20-automations-per-project cap in `CreateAutomation` (BR-AT-02)

**From Solution:** SOL-AT-01
**Priority:** P1
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/usecase/create_automation.go`
**Depends on:** TASK-AT-01-03
**Status:** `[ ]` TODO

---

## Context

`CreateAutomation.Execute` has no per-project count check today. BR-AT-02
caps a project at 20 automations; `ProjectID == ""` (unscoped/back-compat)
skips the cap.

## Changes to make

In `create_automation.go`, add a package-level constant and a check before
`domain.NewAutomation` is called:

```go
const maxAutomationsPerProject = 20 // BR-AT-02
```

```go
if in.ProjectID != "" {
	count, err := uc.repo.CountByProject(ctx, tenantID, in.ProjectID)
	if err != nil {
		return domain.Automation{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_COUNT_FAILED", "failed to count existing automations", err)
	}
	if count >= maxAutomationsPerProject {
		return domain.Automation{}, apperrors.New(apperrors.KindFailedPrecondition, "AUTOMATION_PROJECT_LIMIT_EXCEEDED", "project already has 20 automations", nil)
	}
}
```

Check the exact field/variable names in the current `CreateAutomationInput`
struct and `Execute` signature before editing — `tenantID` may come from
context (`tenant.FromContext(ctx)`) rather than `in`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/usecase/... -run TestCreateAutomation
```

Expected: a fake `AutomationRepository.CountByProject` returning 20 →
`AUTOMATION_PROJECT_LIMIT_EXCEEDED`; returning 19 → succeeds; `ProjectID ==
""` → cap skipped regardless of count. Add these three cases to
`create_automation_test.go` if not already present.
