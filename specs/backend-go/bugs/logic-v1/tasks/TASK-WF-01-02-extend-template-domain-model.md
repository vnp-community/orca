# TASK-WF-01-02: Extend `domain.WorkflowTemplate` with owner/description/tags/inherit fields

**From Solution:** SOL-WF-01
**Priority:** P0
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/domain/template.go`
**Depends on:** TASK-WF-01-01
**Status:** `[x]` DONE — struct extended, `NewWorkflowTemplate` gained required `ownerID` + variadic `TemplateOption`s (`WithDescription`/`WithTags`/`WithUsageCount`/`WithClonedFrom`/`WithOverrides`/`WithInjectSteps`/`WithRemoveSteps`) for parseability-only JSON validation; all call sites updated; new `domain/template_test.go` table test passes (`go test ./internal/domain/... -run TestNewWorkflowTemplate`); full `go build ./... && go vet ./... && go test ./...` green. Note: found pre-existing bug in `postgres.Repository.Update` (`parent_template_id = NULLIF($4, '')` fails Postgres uuid-cast on a non-empty value) unrelated to this task — left as-is since Update isn't in this task's scope; likely needs fixing when TASK-WF-01-06 touches version-bump-on-write.

---

## Context

`workflow-service.md` §4 already names `OwnerID` as part of the target
domain model; backend-go's actual `domain.WorkflowTemplate` never grew it.
This task adds `OwnerID`, `Description`, `Tags`, the three Inherit-mode
merge-instruction fields, `UsageCount`, and `ClonedFromTemplateID` to the
struct, plus a required `ownerID` constructor parameter.

## Changes to make

In `backend-go/services/workflow-service/internal/domain/template.go`,
extend the struct:

```go
type WorkflowTemplate struct {
    ID       string
    TenantID string
    Name     string
    DAGJSON  string
    Scope    Scope
    ParentTemplateID string
    Version  int32

    OwnerID     string   // required — the authoring user, workflow-service.md §4
    Description string   // optional
    Tags        []string // optional, GIN-indexed for BUG-WF-03's library search

    // Inherit-mode merge instructions, applied against the resolved parent
    // chain by resolveEffectiveTemplate. Ignored when ParentTemplateID is
    // empty.
    OverridesJSON    string // map[stepId]json.RawMessage — shallow per-field merge onto that step's Config
    InjectStepsJSON  string // []domain.Step — appended after remove_steps is applied
    RemoveStepsJSON  string // []string — step ids to drop from the parent's resolved steps

    UsageCount int32 // incremented by workflow-service.Execute, read by the version-bump policy

    // ClonedFromTemplateID is a provenance-only pointer (Clone mode
    // deliberately has no live ParentTemplateID) — never walked by
    // ResolveChain, never affects resolution.
    ClonedFromTemplateID string
}
```

Update `NewWorkflowTemplate` to take a new required `ownerID string`
parameter (positional, added after the existing `parentTemplateID`
argument) and validate it:

```go
var ErrTemplateEmptyOwner = errors.New("workflow: template owner id must not be empty")

func NewWorkflowTemplate(id, tenantID, name, dagJSON string, scope Scope, parentTemplateID, ownerID string) (WorkflowTemplate, error) {
    // ... existing tenantID/name/dagJSON validation unchanged ...
    if ownerID == "" {
        return WorkflowTemplate{}, ErrTemplateEmptyOwner
    }
    // ... existing construction, now setting OwnerID: ownerID ...
}
```

`OverridesJSON`/`InjectStepsJSON`/`RemoveStepsJSON` are validated for
parseability only at construction time (valid JSON of their expected
shape, or empty) — semantic validation happens later at `ResolveTemplate`
time (see TASK-WF-01-04), matching how `dag.Validate()` already only
checks a template's own `DAGJSON` at construction.

Update all call sites in `create_template.go` and `update_template.go`
that call `NewWorkflowTemplate` to pass the caller's `ownerID` (from
`ExecutionContext`/request identity, matching how `tenantID` is already
threaded through those usecases).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/domain/... -run TestNewWorkflowTemplate
```

Expected: build is clean; a new/updated table test in
`domain/template_test.go` asserts `NewWorkflowTemplate` rejects an empty
`ownerID` with `ErrTemplateEmptyOwner`, accepts valid
`OverridesJSON`/`InjectStepsJSON`/`RemoveStepsJSON`, and rejects malformed
JSON in any of the three.
