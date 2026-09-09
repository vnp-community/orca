# TASK-WF-004-01: Template inheritance deep-merge — `overrides`/`injectSteps`/`removeSteps`

**From Solution:** BE-SOL-004
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/domain/template.go`, `backend-go/services/workflow-service/internal/usecase/resolve_template.go`, `backend-go/proto/orca/workflow/v1/workflow.proto` (`WorkflowTemplate` message)
**Depends on:** None
**Status:** `[ ]` TODO

---

## ⚠ Must-preserve constraint — re-stated from BE-SOL-004, verified live

`resolveEffectiveTemplate` (`internal/usecase/resolve_template.go:88-107`,
re-read directly and confirmed byte-for-byte matching BE-SOL-004's
citation) is a **documented, deliberate** base-selection policy, not an
oversight: it walks `chain` from its LAST element (most specific,
`chain[len-1]`) back toward `chain[0]` (topmost ancestor) and returns the
FIRST template with `len(dag.Steps) > 0` — "closest-with-steps-wins."
`ResolveTemplate`'s own doc comment (`resolve_template.go:35-46`) states
explicitly: *"Scope (company/team/personal) is a classification field on
each row, not itself part of the resolution algorithm."*

**This task must NOT change that base-selection policy.** It only adds a
second pass, applied AFTER the existing base-selection logic picks
`base`, that folds every descendant-of-base template's
`overrides`/`injectSteps`/`removeSteps` onto `base`'s DAG. A template
using none of the 3 new fields (every template that exists today) must
resolve byte-identical to current behavior — this is the regression test
this task must add, not just aspire to.

## Context

`WorkflowTemplate` (`internal/domain/template.go:63-77`) confirmed to
have exactly 7 fields today: `ID, TenantID, Name, DAGJSON, Scope,
ParentTemplateID, Version` — no `Overrides`/`InjectSteps`/`RemoveSteps`.
`domain.DAGDefinition` (`internal/domain/dag.go:50-52`) is `{Steps []Step}`
with JSON tags (`json:"steps"`) but **no `Serialize()` method** —
confirmed via direct search of `dag.go`; only `ParseDAG`, `Validate`, and
`BuildWaves` exist. BE-SOL-004's sketch calls a nonexistent
`dag.Serialize()` — this task must either add that method (a thin
`json.Marshal(d)` wrapper, matching `DAGDefinition`'s existing JSON tags)
or call `json.Marshal` directly at the two call sites that need to
re-flatten a mutated `DAGDefinition` back to the `DAGJSON string` column
shape. Adding the method is the cleaner choice since `ParseDAG`/
`Validate`/`BuildWaves` already live as methods on `DAGDefinition`.

## Changes to make

**1. `internal/domain/template.go`** — add the 3 new fields:

```go
type WorkflowTemplate struct {
	// ...existing 7 fields unchanged...
	Overrides   map[string]any  `json:"overrides,omitempty"`
	InjectSteps []StepInjection `json:"injectSteps,omitempty"`
	RemoveSteps []string        `json:"removeSteps,omitempty"`
}

// StepInjection adds a new step adjacent to an existing one in a parent
// template's resolved DAG — see resolveEffectiveTemplate's applyInjections.
type StepInjection struct {
	AnchorStepID string `json:"anchorStepId"`
	Position     string `json:"position"` // "before" | "after"
	Step         Step   `json:"step"`
}
```

(`Step` here is `domain.Step` from `dag.go:39-44` — same package, no
import needed.)

**2. `internal/domain/dag.go`** — add `Serialize`:

```go
// Serialize marshals d back to the JSON string shape the templates/
// executions tables persist in their dag_json column — the inverse of
// ParseDAG.
func (d DAGDefinition) Serialize() (string, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("domain: serialize dag: %w", err)
	}
	return string(b), nil
}
```

**3. `internal/usecase/resolve_template.go`** — extend
`resolveEffectiveTemplate` as a fold that runs strictly AFTER the existing
base-selection loop, changing nothing about how `base` itself is picked:

```go
func resolveEffectiveTemplate(chain []domain.WorkflowTemplate) (domain.WorkflowTemplate, error) {
	// UNCHANGED — the existing closest-with-steps-wins loop, verbatim:
	var base domain.WorkflowTemplate
	found := false
	for i := len(chain) - 1; i >= 0; i-- {
		dag, err := domain.ParseDAG(chain[i].DAGJSON)
		if err != nil {
			return domain.WorkflowTemplate{}, err
		}
		if len(dag.Steps) > 0 {
			base, found = chain[i], true
			break
		}
	}
	if !found {
		base = chain[len(chain)-1]
	}

	// NEW — fold every descendant-of-base template's overrides/injections/
	// removals, in root-to-requested order (chain is root-first per
	// ResolveChain's contract, so iterating chain forward after base's
	// index gives "farther overrides applied first, closer overrides
	// applied last and therefore winning" — matches BE-SOL-004's stated
	// intent).
	dag, err := domain.ParseDAG(base.DAGJSON)
	if err != nil {
		return domain.WorkflowTemplate{}, err
	}
	baseIdx := indexOf(chain, base.ID)
	for i := baseIdx + 1; i < len(chain); i++ {
		tpl := chain[i]
		dag = applyOverrides(dag, tpl.Overrides)
		dag = applyInjections(dag, tpl.InjectSteps)
		dag = applyRemovals(dag, tpl.RemoveSteps)
	}
	serialized, err := dag.Serialize()
	if err != nil {
		return domain.WorkflowTemplate{}, err
	}
	base.DAGJSON = serialized
	return base, nil
}
```

For a template using none of the 3 new fields, `tpl.Overrides == nil`,
`tpl.InjectSteps == nil`, `tpl.RemoveSteps == nil` for every `tpl` in the
loop — `applyOverrides`/`applyInjections`/`applyRemovals` must each be a
true no-op on nil/empty input (return `dag` unchanged), and the
`dag.Serialize()` round-trip (`ParseDAG` → `Serialize`) must reproduce
`base.DAGJSON` byte-for-byte for this case — verify this explicitly in
the regression test below, since a `json.Marshal` round-trip through an
intermediate struct can reorder map keys or drop/add whitespace even when
semantically unchanged; if the existing test suite or any caller
compares `DAGJSON` strings for byte-equality rather than semantic
equality, this is a real risk to flag in the PR.

**4. `applyOverrides`/`applyInjections`/`applyRemovals`** (new helpers in
`resolve_template.go` or a new `template_merge.go`):

- `applyOverrides(dag, overrides map[string]any)`: keys are dot-paths into
  a step's config (e.g. `"stepId.prompt"`) or another agreed addressing
  scheme — pick one and document it; BE-SOL-004 doesn't specify the
  key format precisely, only that "a template with `overrides` changing
  one step's `prompt`" must work, per its test plan.
- `applyInjections(dag, injections []StepInjection)`: inserts
  `injection.Step` into `dag.Steps` adjacent to `injection.AnchorStepID`
  per `injection.Position` ("before"/"after") — error (or skip, pick one
  and document it) if `AnchorStepID` doesn't exist in `dag.Steps`.
- `applyRemovals(dag, removeStepIDs []string)`: removes matching steps
  from `dag.Steps` AND strips them from every remaining step's
  `DependsOn` — a removal that leaves a dangling `dependsOn` reference
  would fail `DAGDefinition.Validate`'s existing
  `ErrStepDependencyNotFound` check later; decide whether that's the
  desired failure mode (removal + missing dependency = template author
  error) or whether removal should also prune dangling `dependsOn`
  entries silently. BE-SOL-004 doesn't specify; document whichever choice
  is made.

**5. `workflow.proto`**'s `WorkflowTemplate` message — add the mirror
fields (`google.protobuf.Struct overrides`, `repeated StepInjection
inject_steps`, `repeated string remove_steps`), plus a new
`StepInjection` message. Update `internal/adapter/grpc/server.go`'s
proto↔domain `WorkflowTemplate` conversion helper(s) (grep for the
existing `toProtoTemplate`/`toDomainTemplate`-style function used by
`CreateTemplate`/`UpdateTemplate`/`ListTemplates` handlers) to carry the
3 new fields through.

## Test plan

- Template with `overrides` changing one step's `prompt` → resolved DAG
  has that field changed, every other field/step unchanged from parent.
- `injectSteps`/`removeSteps` each tested independently and combined.
- **Regression, not just aspiration**: every existing case in
  `resolve_template_test.go` (re-run as-is) passes unchanged — a template
  with none of the 3 new fields resolves byte-identical (or, if the
  `Serialize` round-trip risk above is real, semantically identical —
  document which) to current behavior.
- Base-selection itself (which template in the chain has `len(dag.Steps)
  > 0`) is untouched: a test with a personal template with only
  `overrides` set and an empty `dag_json` must still resolve using its
  parent's steps as `base`, exactly as today — `overrides` on an
  empty-DAG template applies to whatever `base` the existing algorithm
  picks, it does not itself make that template `base`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/domain/... -run TestDAGDefinition_Serialize -v
go test ./services/workflow-service/internal/usecase/... -run TestResolveTemplate -v
```

Expected: clean build; every pre-existing `TestResolveTemplate` case
still passes unmodified; new `overrides`/`injectSteps`/`removeSteps`
cases pass; base-selection policy test explicitly confirms it's
unchanged.
