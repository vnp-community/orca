# TASK-WF-01-06: Gate `templates.version` bump on breaking change + active usage

**From Solution:** SOL-WF-01
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/usecase/update_template.go`
**Depends on:** TASK-WF-01-02
**Status:** `[x]` DONE — `isBreakingChange`/`parseStepsByID` added, `UpdateTemplate.Execute` gates `bumpVersion := existing.UsageCount > 0 && isBreakingChange(...)`; `TemplateRepository.Update` gained a `bumpVersion bool` param (ports.go + fake + postgres adapter); postgres `UPDATE` now does `version = version + (CASE WHEN $8 THEN 1 ELSE 0 END)`. Also fixed a pre-existing bug found while here: `parent_template_id = NULLIF($4, '')` failed a Postgres uuid-cast on any non-empty value — now `NULLIF($4, '')::uuid`. Table test `TestUpdateTemplate_VersionBumpGate` covers all 4 matrix cases + a dag-unchanged case; new `TestRepository_Update_NoBump_LeavesVersionUnchanged` integration test added. `go build ./... && go vet ./... && go test ./...` green; `go test -tags=integration -run TestRepository_Update ./internal/adapter/postgres/...` passes 3/3 in isolation (shared Docker contention from concurrent worktree agents causes transient failures when run alongside the full suite — not a regression).

---

## Context

`UpdateTemplate` bumps `templates.version` unconditionally today
(`update_template.go:38,61`). BUG-WF-01's spec requires bumping only when
a template with active usage (`UsageCount > 0`) receives a breaking DAG
change (a step removed, or a step's `Type` changed under the same id).

## Changes to make

In `update_template.go`, after the existing `GetTemplate` fetch and
`NewWorkflowTemplate` construction:

```go
current, _ := uc.templates.GetTemplate(ctx, tenantID, in.ID) // already fetched — reused, not a second call
next, err := domain.NewWorkflowTemplate(...)

breaking := isBreakingChange(current, next)
bumpVersion := current.UsageCount > 0 && breaking
// Metadata-only edits (description/tags/scope) and non-breaking DAG edits
// (adding a step, adding a new optional dependsOn) never bump — only a
// breaking DAG change to an ACTIVELY-USED template does.

updated, err := uc.templates.Update(ctx, next, in.ExpectedVersion, bumpVersion)
```

Add the detector:

```go
// isBreakingChange reports true if any step id present in old is absent
// from new (a removed step — anything downstream referencing its output
// via {{outputs.stepId...}} silently breaks), or any step id present in
// both has a different Type. Config-only changes and new steps/edges are
// treated as non-breaking — a conservative first cut, flagged as a policy
// choice reviewable independently of schema/proto changes.
func isBreakingChange(old, next domain.WorkflowTemplate) bool {
    oldSteps := parseSteps(old.DAGJSON)
    newSteps := parseStepsByID(next.DAGJSON)
    for _, os := range oldSteps {
        ns, ok := newSteps[os.ID]
        if !ok || ns.Type != os.Type {
            return true
        }
    }
    return false
}
```

Extend `TemplateRepository.Update` in `internal/usecase/ports.go` with a
`bumpVersion bool` parameter:

```go
Update(ctx context.Context, tmpl domain.WorkflowTemplate, expectedVersion int32, bumpVersion bool) (domain.WorkflowTemplate, error)
```

Update the `adapter/postgres` implementation's `UPDATE` statement to
conditionally include `version = version + 1` based on `bumpVersion`,
still gated by the existing `WHERE version = $expected_version`
optimistic-concurrency check.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run TestUpdateTemplate
go test ./services/workflow-service/internal/adapter/postgres/... -run TestTemplateRepository_Update
```

Expected: `UsageCount == 0` + breaking edit → no bump; `UsageCount > 0` +
breaking edit → bump; `UsageCount > 0` + non-breaking edit (new step,
description-only change) → no bump; `UsageCount > 0` + step `Type`
changed under same id → bump. Repository test: `bumpVersion=false` leaves
`version` unchanged while updating every other column; `bumpVersion=true`
increments it; both still respect `expected_version`.
