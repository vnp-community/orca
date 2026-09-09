# BE-SOL-004: Template inheritance deep-merge, Clone mode, conditional version bump

**Resolves:** [CR-WF-004](../../../../../../docs/crs/v4/workflow/CR-WF-004-template-inheritance-merge-and-clone.md)
**Service:** `workflow-service` only
**Affected files (proposed):**
- `backend-go/services/workflow-service/internal/usecase/resolve_template.go`
- `backend-go/services/workflow-service/internal/usecase/create_template.go`, `update_template.go`, `clone_template.go` (new)
- `backend-go/services/workflow-service/internal/domain/template.go`
- `backend-go/proto/orca/workflow/v1/workflow.proto` (`overrides`/`injectSteps`/`removeSteps` fields, `CloneTemplate` RPC)
**Status:** 📋 Proposed — not yet implemented

> **⚠️ Cập nhật sau khi viết task (2026-09-09):** §"Design — conditional
> version bump" gọi `uc.repo.UpdateConditional(...)` — method này **không
> tồn tại**. Method thật là `TemplateRepository.Update(ctx, tmpl
> domain.WorkflowTemplate, expectedVersion int32)` (`ports.go:35-38`,
> caller thật ở `update_template.go:61`), nhận nguyên 1
> `domain.WorkflowTemplate` đã dựng sẵn, không nhận `changes` rời. Xem
> [TASK-WF-004-03](../tasks/TASK-WF-004-03-conditional-version-bump.md)
> cho signature đã sửa (mở rộng `Update` thêm tham số `bump bool`, giữ
> nguyên optimistic-concurrency `WHERE version = $expectedVersion`).

---

## Current state — confirmed exactly as the CR describes, plus one nuance worth preserving

`resolveEffectiveTemplate` (`resolve_template.go:88-107`) is a **documented,
deliberate** policy, not an oversight: its own doc comment states the
resolution walks from most-specific to root and returns the **first
template in the chain with ≥1 step**, treating a template with an empty
`dag_json` as "opts into its parent's steps wholesale." Notably (and worth
preserving, not silently discarding): *"Scope (company/team/personal) is a
classification field on each row, not itself part of the resolution
algorithm."* This solution's deep-merge design should keep that same
property — `overrides`/`injectSteps`/`removeSteps` operate purely on the
parent chain's specificity order, independent of `Scope`.

`WorkflowTemplate.Version` (`template.go:73-75`) is confirmed bumped on
**every** `UpdateTemplate` write (optimistic-concurrency check, "SOL-030"
per its own comment) — this solution's conditional-bump change must not
break that concurrency check; see §3 below for how the two interact.

No `Overrides`/`InjectSteps`/`RemoveSteps` fields or `CloneTemplate` RPC
exist — confirmed, and also absent from `workflow-service.md`'s own TDD
(§5's schema section adds only `description`/`owner_id` beyond what's
built) — this is a genuine extension beyond the TDD's literal text, not a
"catch up to spec" change.

## Design — `overrides`/`injectSteps`/`removeSteps`

```go
// domain/template.go
type WorkflowTemplate struct {
    // ...existing fields unchanged...
    Overrides   map[string]any  `json:"overrides,omitempty"`
    InjectSteps []StepInjection `json:"injectSteps,omitempty"`
    RemoveSteps []string        `json:"removeSteps,omitempty"`
}
type StepInjection struct {
    AnchorStepID string      `json:"anchorStepId"`
    Position     string      `json:"position"` // "before" | "after"
    Step         domain.Step `json:"step"`
}
```

```go
// resolve_template.go — resolveEffectiveTemplate rewritten as a fold, not a first-match-wins pick
func resolveEffectiveTemplate(chain []domain.WorkflowTemplate) (domain.WorkflowTemplate, error) {
    base, err := firstWithSteps(chain) // KEEPS the existing "closest-with-steps-wins" base-selection policy unchanged
    if err != nil { return domain.WorkflowTemplate{}, err }
    dag, err := domain.ParseDAG(base.DAGJSON)
    if err != nil { return domain.WorkflowTemplate{}, err }
    // Apply every descendant-of-base template's overrides/injections/removals,
    // root-to-requested order (so a closer override always wins over a farther one).
    for _, tpl := range chain {
        if tpl.ID == base.ID { continue }
        dag = applyOverrides(dag, tpl.Overrides)
        dag = applyInjections(dag, tpl.InjectSteps)
        dag = applyRemovals(dag, tpl.RemoveSteps)
    }
    base.DAGJSON = dag.Serialize()
    return base, nil
}
```

This is additive to the existing policy, not a replacement — a template
with no `Overrides`/`InjectSteps`/`RemoveSteps` (every template that exists
today) resolves exactly as before, since the new loop is a no-op for empty
fields.

## Design — Clone mode

```protobuf
rpc CloneTemplate(CloneTemplateRequest) returns (CloneTemplateResponse);
message CloneTemplateRequest { string source_template_id = 1; string new_name = 2; string scope = 3; }
```

```go
// clone_template.go
func (uc *CloneTemplate) Execute(ctx context.Context, in CloneTemplateInput) (domain.WorkflowTemplate, error) {
    resolved, err := uc.resolveTemplate.Execute(ctx, ResolveTemplateInput{TemplateID: in.SourceTemplateID})
    if err != nil { return domain.WorkflowTemplate{}, err }
    // NewWorkflowTemplate with NO ParentTemplateID — severs inheritance entirely,
    // matching ErrTemplateSelfParent's existing guard (a clone is a fresh root, never a child of its source).
    clone, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, in.NewName, resolved.Template.DAGJSON, domain.Scope(in.Scope), "")
    if err != nil { return domain.WorkflowTemplate{}, err }
    return uc.repo.Create(ctx, clone)
}
```

## Design — conditional version bump

```go
// update_template.go
func (uc *UpdateTemplate) Execute(ctx context.Context, in UpdateTemplateInput) (domain.WorkflowTemplate, error) {
    isBreaking := in.RemovesStep() || in.ChangesStepType() || in.ChangesDependency()
    hasActiveUsage, err := uc.repo.HasActiveExecutionsUsingTemplate(ctx, tenantID, in.ID)
    if err != nil { return domain.WorkflowTemplate{}, err }
    bump := isBreaking && hasActiveUsage
    return uc.repo.UpdateConditional(ctx, tenantID, in.ID, in.ExpectedVersion, changes, bump)
    // UpdateConditional's existing optimistic-concurrency WHERE-version-matches
    // clause (ErrTemplateVersionConflict) is UNCHANGED — bump is now a boolean
    // parameter to the same conditional UPDATE, not a separate code path.
}
```

**Product decision required before merge:** the exact definition of
"breaking" (remove step / change step type / change dependency, as
proposed here) needs team sign-off — this solution proposes a concrete
starting definition, not a final one.

## Test plan

- Template with `overrides` changing one step's `prompt` → resolved DAG has
  that field changed, every other field/step unchanged from parent.
- `injectSteps`/`removeSteps` each tested independently and combined.
- Template with none of the 3 new fields resolves byte-identical to current
  behavior (regression test against existing `resolve_template_test.go`
  cases).
- `CloneTemplate` then mutate source → clone unaffected (and vice versa).
- `UpdateTemplate` with a non-breaking change (e.g. rename) does NOT bump
  version even with active executions; a breaking change WITH active usage
  does; a breaking change with zero active executions does not.

## Not in scope (per the CR)

- Migrating existing templates to the new field shape — additive, no
  backfill needed (empty fields = current behavior).
- UI for authoring `overrides`/`injectSteps`/`removeSteps` —
  [FE-SOL-001](../../../../../frontend/crs/v4/workflow/solutions/FE-SOL-001-frontend-builder-library-pause-resume.md).

## References

- [CR-WF-004](../../../../../../docs/crs/v4/workflow/CR-WF-004-template-inheritance-merge-and-clone.md)
- `backend-go/services/workflow-service/internal/domain/template.go`
- `backend-go/services/workflow-service/internal/usecase/resolve_template.go`
