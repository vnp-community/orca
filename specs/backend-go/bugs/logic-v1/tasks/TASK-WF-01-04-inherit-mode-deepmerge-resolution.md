# TASK-WF-01-04: Replace nearest-ancestor-wins with field-level deepMerge in `resolveEffectiveTemplate`

**From Solution:** SOL-WF-01
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/usecase/resolve_template.go`
**Depends on:** TASK-WF-01-02
**Status:** `[x]` DONE — `resolveEffectiveTemplate` replaced with field-level deepMerge (`parseSteps`/`removeSteps`/`applyOverrides`/`parseInjectSteps`/`mergeConfigOneLevel`); `ResolveTemplate.Execute` now builds the effective `WorkflowTemplate` from the leaf's own identity + merged dag_json. Updated `TestResolveTemplate_EmptyLeafInheritsFromParent`'s assertion (steps content, not ancestor-row identity — deliberate per the new "effective view of the requested template" semantics) and added override/remove/inject/regression/cyclic-merge table tests in `resolve_template_test.go`. `go build ./... && go vet ./... && go test ./...` green; `go test ./internal/usecase/... -run TestResolveTemplate -race` passes (10/10).

---

## Context

`resolveEffectiveTemplate` (`resolve_template.go:88-107`) today picks the
"nearest non-empty ancestor" wholesale. BUG-WF-01 requires field-level
`overrides`/`inject_steps`/`remove_steps` resolution across the chain
instead. The replacement must generalize the old behavior: a chain with
none of those three fields set anywhere resolves identically to before.

## Changes to make

Replace `resolveEffectiveTemplate` in `resolve_template.go`:

```go
// resolveEffectiveTemplate walks chain root-first (chain[0] = topmost
// ancestor, per ResolveChain's existing contract) and folds each level's
// own steps/overrides/inject_steps/remove_steps onto an accumulator.
func resolveEffectiveTemplate(chain []domain.WorkflowTemplate) ([]domain.Step, error) {
    acc := parseSteps(chain[0].DAGJSON) // topmost ancestor's own definition, may be empty
    for _, level := range chain[1:] {
        if steps := parseSteps(level.DAGJSON); len(steps) > 0 {
            acc = steps // own steps fully replace — same rule the old policy
                        // had, now scoped per-level instead of whole-chain
        }
        acc = removeSteps(acc, level.RemoveStepsJSON)
        acc = applyOverrides(acc, level.OverridesJSON)
        acc = append(acc, parseInjectSteps(level.InjectStepsJSON)...)
    }
    dag := domain.DAGDefinition{Steps: acc}
    if err := dag.Validate(); err != nil {
        return nil, err // mirrors the existing parse-failure error mapping below
    }
    return acc, nil
}

// applyOverrides unmarshals overridesJSON as map[string]json.RawMessage
// (step id -> partial JSON object) and, for each step whose id has an
// entry, merges that object's top-level keys into the step's own Config —
// a one-level-deep merge, not a recursive deep-merge into nested config
// structures (nested-key override is a documented non-goal).
func applyOverrides(steps []domain.Step, overridesJSON string) []domain.Step { /* ... */ }

// removeSteps drops any step whose id appears in removeStepsJSON, and
// also strips that id from every remaining step's DependsOn (a removed
// step's dependents lose that edge rather than becoming permanently
// unsatisfiable).
func removeSteps(steps []domain.Step, removeStepsJSON string) []domain.Step { /* ... */ }
```

`ResolveTemplate` returns a `KindInvalidArgument`/`WORKFLOW_INVALID_TEMPLATE`
error if the merged result fails `dag.Validate()`, matching
`resolve_template.go:80-83`'s existing error-mapping shape for a parse
failure.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run TestResolveTemplate -race
```

Expected: new table tests in `resolve_template_test.go` cover — a
3-level chain where only the middle level overrides a step id from the
top ancestor (merged `Config` reflects the override, other fields
survive); `remove_steps` on the leaf drops a step and strips it from
remaining `DependsOn`, result still passes `Validate()`; `inject_steps`
appends a step referencing an inherited step id; a regression case
reusing the old fixtures verbatim proves a chain with no
overrides/inject/remove resolves identically to the pre-change behavior;
a merge producing a cyclic/dangling DAG returns
`WORKFLOW_INVALID_TEMPLATE`.
