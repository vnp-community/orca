# TASK-WF-003-01: `ExecuteRequest.inputs` + `{{outputs.&lt;stepId&gt;.*}}` interpolation pass

**From Solution:** BE-SOL-003
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/proto/orca/workflow/v1/workflow.proto` (`ExecuteRequest`), `backend-go/services/workflow-service/internal/usecase/execute.go`, `backend-go/services/workflow-service/internal/usecase/interpolate.go` (new), `backend-go/services/workflow-service/internal/usecase/wave_dispatcher.go`, `backend-go/services/workflow-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-WF-002-03 (this task's interpolation pass runs on step config JSON before the resolvers/executors from that task consume it)
**Status:** `[ ]` TODO

---

## ⚠ Coordination requirement — `ExecuteRequest.origin_task_id`, cite per BE-SOL-003's own note

BE-SOL-003 explicitly flags this and it must be re-confirmed live before
merging this task's proto change: `docs/crs/v3/flow-task/CR-FLOW-TASK-002-workflow-as-task-execution-engine.md`
**also** adds a field to `ExecuteRequest` — `origin_task_id` — as part of
wiring workflow-service up as a Task execution engine. Both changes touch
the same message. Confirmed live: `ExecuteRequest`
(`proto/orca/workflow/v1/workflow.proto:84-89`) is currently exactly
`{template_id, project_id, root_trace_id, request_id}` — neither
`inputs` nor `origin_task_id` exists yet, so as of this writing there is
no collision on-disk. But if `CR-FLOW-TASK-002`'s task series lands its
`origin_task_id` field first (check
`specs/backend-go/crs/v3/flow-task/tasks/TASK-FT-002-01-proto-workflow-template-id-and-report-result.md`'s
status before starting), this task must add `inputs` as the NEXT free
field number after whatever `origin_task_id` claimed, not assume field
`5`/`6` in isolation. **Re-run a live `grep -n "message ExecuteRequest" -A
10 proto/orca/workflow/v1/workflow.proto` immediately before editing the
proto file** to pick the correct next-free field numbers for both fields
if only one of the two has landed so far.

## Context

`ExecuteInput` (`internal/usecase/execute.go:19-24`) mirrors
`ExecuteRequest` field-for-field today: `{TemplateID, ProjectID,
RootTraceID, RequestID}` — confirmed directly, no `Inputs` field. No
interpolation pass exists anywhere in `workflow-service` — confirmed via
direct search (no `Interpolate`, no `{{`-pattern handling, anywhere under
`internal/`).

The real step-dispatch call chain was read directly in
`wave_dispatcher.go`: `dispatchWave` → `dispatchStep` (`:172-191`) →
`runStep` (`:199-214`), which does:

```go
executor, err := d.registry.Resolve(step.Type)
...
result, err := executor.Execute(ctx, string(step.Config))
```

`step.Config` is `json.RawMessage` (`internal/domain/dag.go:42`) — the
interpolation pass's call site is this line in `runStep`, immediately
before `string(step.Config)` is handed to `executor.Execute`, NOT inside
a function named `executeStep()` as BE-SOL-003's sketch names it — no
such function exists in the real file; the real function is `runStep`.

## Changes to make

**1. `workflow.proto`** — widen `ExecuteRequest` (exact field number per
the coordination note above):

```protobuf
message ExecuteRequest {
  string template_id = 1;
  string project_id = 2;
  string root_trace_id = 3;
  string request_id = 4;
  google.protobuf.Struct inputs = 5; // NEW — re-check field number against origin_task_id's landing status, see this task's coordination note
}
```

Add `import "google/protobuf/struct.proto";` to `workflow.proto` if not
already present (confirm at implementation time — `project.proto` already
imports it, `workflow.proto` may not).

**2. `internal/usecase/execute.go`** — widen `ExecuteInput`:

```go
type ExecuteInput struct {
	TemplateID  string
	ProjectID   string
	RootTraceID string
	RequestID   string
	Inputs      map[string]any // NEW — from ExecuteRequest.inputs
}
```

`Execute`'s body threads `in.Inputs` through to wherever it persists the
execution row and hands off to `waveDispatcher` — confirm at
implementation time whether `domain.WorkflowExecution` needs its own new
field to persist `Inputs` for later interpolation calls on
resume/recovery (`RecoverExecutions`' boot-time scan), since a
`{{input_field}}` reference inside a step dispatched after a restart still
needs the original inputs available.

**3. `internal/adapter/grpc/server.go`** — `Execute`'s handler
(`:76-87`) currently builds `usecase.ExecuteInput{TemplateID:
req.GetTemplateId(), ProjectID: req.GetProjectId(), RootTraceID:
req.GetRootTraceId(), RequestID: req.GetRequestId()}` — add
`Inputs: req.GetInputs().AsMap()` (proto `Struct`'s standard
`AsMap()` conversion).

**4. `internal/usecase/interpolate.go`** (new):

```go
package usecase

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

var templatePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.]+)\s*\}\}`)

// Interpolate substitutes {{path}} references in raw against inputs (an
// execution's ExecuteRequest.inputs) and outputs (already-completed
// steps' results, keyed by step id). Runs once per step, immediately
// before that step's executor is invoked — NOT once for the whole
// execution up front — since {{outputs.<stepId>.*}} is only resolvable
// once stepId has actually completed; interpolating early would either
// require a second pass per step anyway or force strict topological
// pre-validation that duplicates work BuildWaves already does.
func Interpolate(raw string, inputs map[string]any, outputs map[string]domain.StepResult) (string, error) {
	var firstErr error
	result := templatePattern.ReplaceAllStringFunc(raw, func(match string) string {
		if firstErr != nil {
			return match
		}
		path := templatePattern.FindStringSubmatch(match)[1]
		if rest, ok := strings.CutPrefix(path, "outputs."); ok {
			val, err := resolveStepOutput(outputs, rest)
			if err != nil {
				firstErr = err
				return match
			}
			return val
		}
		val, ok := inputs[path]
		if !ok {
			firstErr = fmt.Errorf("interpolate: unknown reference %q", path)
			return match
		}
		return fmt.Sprintf("%v", val)
	})
	if firstErr != nil {
		return "", firstErr
	}
	return result, nil
}

// resolveStepOutput errors if stepId isn't in outputs yet — a DAG-author
// error (referencing a step that hasn't run, or doesn't exist, in this
// DAG), not a silent empty string.
func resolveStepOutput(outputs map[string]domain.StepResult, path string) (string, error) {
	stepID, field, ok := strings.Cut(path, ".")
	if !ok {
		return "", fmt.Errorf("interpolate: outputs reference %q missing a field after the step id", path)
	}
	res, ok := outputs[stepID]
	if !ok {
		return "", fmt.Errorf("interpolate: step %q has not completed (or does not exist in this DAG)", stepID)
	}
	// res.OutputJSON is StepType-specific JSON — parse and extract `field`
	// via a JSON-path lookup (e.g. gjson or encoding/json + map[string]any
	// walk); exact implementation left open here, matching this codebase's
	// existing "parse StepType-specific JSON only where the structure is
	// actually needed" convention (see domain.WorkflowTemplate's DAGJSON
	// doc comment for the same pattern applied to templates).
	_ = field
	return "", nil // placeholder — implement the field extraction above
}
```

**5. `wave_dispatcher.go`'s `runStep`** — call `Interpolate` on every
string-typed field of `step.Config`'s JSON before calling
`executor.Execute`:

```go
func (d *waveDispatcher) runStep(ctx context.Context, step domain.Step, se *domain.StepExecution) (domain.StepResult, error) {
	executor, err := d.registry.Resolve(step.Type)
	if err != nil {
		se.Fail(err.Error())
		return domain.StepResult{}, err
	}

	interpolatedConfig, err := interpolateStepConfig(string(step.Config), d.inputs, d.completedOutputs) // NEW
	if err != nil {
		se.Fail(err.Error())
		return domain.StepResult{}, err
	}

	result, err := executor.Execute(ctx, interpolatedConfig)
	...
}
```

`waveDispatcher` needs new fields (`inputs map[string]any`,
`completedOutputs map[string]domain.StepResult`, the latter accumulated
across waves as each step's `StepResult` becomes available) — confirm at
implementation time exactly where `completedOutputs` is best accumulated
given `dispatchWave`'s existing per-wave `results` slice
(`wave_dispatcher.go:150-167`) and `dispatchWavesFrom`'s cross-wave loop
(`:75-90`); it must be threaded across waves, not reset per wave, since a
later wave's step may reference an earlier wave's output.

`interpolateStepConfig` is a small helper this task adds that walks the
step config's JSON object, interpolating every string-typed leaf value via
`Interpolate` and re-marshaling — exact JSON-walk implementation left to
the engineer, following whatever JSON-tree helper (if any) this codebase
already uses elsewhere for generic JSON manipulation.

## Test plan

- `{{feature_description}}`-style input interpolates correctly from
  `ExecuteInput.Inputs`.
- A step referencing `{{outputs.stepX.field}}` where `stepX` runs in an
  earlier wave interpolates correctly once that wave completes.
- A step referencing `{{outputs.stepX.field}}` where `stepX` hasn't run
  yet in DAG order → clear validation error, not a runtime empty string.
  Decide whether this is caught at execute-start (DAG-shape static check)
  or at dispatch time (this task's `Interpolate` erroring) — BE-SOL-003
  prefers execute-start; if that's out of scope for this task, the
  dispatch-time error from `resolveStepOutput` above is an acceptable
  fallback but must be flagged as a scope reduction in the PR.
- A step referencing an unknown top-level input key → clear error.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run "TestInterpolate|TestExecute|TestWaveDispatcher" -v
```

Expected: clean build; interpolation unit tests pass in isolation;
`wave_dispatcher` integration tests confirm interpolated values actually
reach the step executor's `Execute` call, not just the raw template
string.
