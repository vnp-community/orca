# BE-SOL-003: `ExecuteRequest.inputs` + `{{outputs.&lt;stepId&gt;.*}}` interpolation + `action`/`parallel` step types

**Resolves:** [CR-WF-003](../../../../../../docs/crs/v4/workflow/CR-WF-003-variable-interpolation-and-step-types.md)
**Service:** `workflow-service` only
**Depends on:** [BE-SOL-002](./BE-SOL-002-server-and-provider-resolution.md); **coordinate proto timing** with [`docs/crs/v3/flow-task/CR-FLOW-TASK-002`](../../../../../../docs/crs/v3/flow-task/CR-FLOW-TASK-002-workflow-as-task-execution-engine.md), which also adds a field to `ExecuteRequest` (`origin_task_id`)
**Affected files (proposed):**
- `backend-go/proto/orca/workflow/v1/workflow.proto` (`ExecuteRequest.inputs`, `StepType` enum additions)
- `backend-go/services/workflow-service/internal/usecase/execute.go`, `interpolate.go` (new)
- `backend-go/services/workflow-service/internal/domain/step.go` (`StepTypeAction`, `StepTypeParallel`)
- `backend-go/services/workflow-service/internal/usecase/wave_dispatcher.go`
**Status:** 📋 Proposed — not yet implemented

---

## Current state

`ExecuteRequest` (`workflow.proto:84-89`) is exactly `{template_id,
project_id, root_trace_id, request_id}` — confirmed directly, no `inputs`
field. `ExecuteInput` (`internal/usecase/execute.go`) mirrors this. No
interpolation pass exists anywhere in the service. `StepType`
(`step.go:16-23`) is a closed 5-value enum
(`agent|shell|notification|webhook|condition`) with a `Valid()` switch that
rejects anything else — confirmed directly, no `action`/`parallel`.

## Design — `ExecuteRequest.inputs`

```protobuf
message ExecuteRequest {
  string template_id = 1;
  string project_id = 2;
  string root_trace_id = 3;
  string request_id = 4;
  google.protobuf.Struct inputs = 5;       // NEW
  string origin_task_id = 6;               // NEW, coordinated with CR-FLOW-TASK-002 — leave empty when run standalone
}
```

## Design — interpolation pass

```go
// internal/usecase/interpolate.go
// Runs once per step, immediately before that step's executor is invoked —
// NOT once for the whole execution up front, since {{outputs.<stepId>.*}}
// is only resolvable once stepId has actually completed.
func Interpolate(raw string, inputs map[string]any, outputs map[string]domain.StepResult) (string, error) {
    return templatePattern.ReplaceAllStringFuncE(raw, func(match string) (string, error) {
        path := extractPath(match)
        if rest, ok := strings.CutPrefix(path, "outputs."); ok {
            return resolveStepOutput(outputs, rest) // error if stepId not yet in outputs — DAG author error, not a silent empty string
        }
        return fmt.Sprintf("%v", inputs[path]), nil
    })
}
```

Called from `wave_dispatcher.go` on every string-typed field of a step's
config JSON before marshaling params for its executor — same call site for
all 5 (soon 7) step types, one implementation.

## Design — `action` step type

```go
// step.go
const StepTypeAction StepType = "action"
type ActionStepConfig struct {
    Action string         `json:"action"` // "git.createBranch" | "github.createPR" | ...
    Params map[string]any `json:"params"`
}
```

Dispatch registry maps `Action` string to a handler calling the relevant
existing service client (`git-gateway-service` for `git.*`,
`issue-tracking-service` for `github.*`/`jira.*` — both already have gRPC
clients used elsewhere in the codebase; no new client wiring invented here,
reused as-is). **The set of supported actions is a product decision** —
this solution defines the dispatch mechanism only; which actions actually
ship is out of scope (see CR-WF-003 §3's own risk note).

## Design — `parallel` step type

```go
const StepTypeParallel StepType = "parallel"
type ParallelStepConfig struct {
    Steps               []Step `json:"steps"`
    AllowPartialFailure  bool   `json:"allowPartialFailure"`
}
```

```go
// wave_dispatcher.go — new case in executeStep()
case domain.StepTypeParallel:
    results := make([]domain.StepResult, len(cfg.Steps))
    var wg sync.WaitGroup
    for i, sub := range cfg.Steps {
        wg.Add(1)
        go func(i int, s domain.Step) { defer wg.Done(); results[i] = e.executeStep(ctx, s) }(i, sub)
    }
    wg.Wait()
    if !cfg.AllowPartialFailure && anyFailed(results) { return domain.StepResult{Status: domain.ResultStatusFailed}, ErrParallelStepFailed }
```

This is distinct from the wave-level parallelism `wave_dispatcher.go`
already does across independent DAG steps (`workflow-service.md` §4's
`BuildWaves` — real, unchanged by this solution) — `parallel` is a single
step's own nested fan-out, a different mechanism operating one level down.

## Test plan

- `{{feature_description}}`-style input interpolates correctly; a step
  referencing `{{outputs.stepX.field}}` before `stepX` has run in the DAG
  order → clear validation error at execute-start (DAG-shape check), not a
  runtime empty-string surprise.
- `parallel` step: one sub-step fails, `allowPartialFailure=true` → step
  succeeds with partial results; `false` → step fails.
- `action` step dispatch reaches the right downstream client for at least
  one registered action (whichever the team greenlights first).

## Not in scope (per the CR)

- Which specific `action` handlers ship — product decision.
- UI for `action`/`parallel` — [FE-SOL-001](../../../../../frontend/crs/v4/workflow/solutions/FE-SOL-001-frontend-builder-library-pause-resume.md).
- Nested `parallel`-within-`parallel` depth limits — not enforced by this
  solution; flag in review if the team wants a hard cap.

## References

- [CR-WF-003](../../../../../../docs/crs/v4/workflow/CR-WF-003-variable-interpolation-and-step-types.md)
- `backend-go/proto/orca/workflow/v1/workflow.proto:84-89`
- `backend-go/services/workflow-service/internal/domain/step.go:11-30`
