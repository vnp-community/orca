# TASK-WF-02-06: Add `{{...}}` variable interpolation + wire into wave dispatch

**From Solution:** SOL-WF-02
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/domain/interpolation.go` (new)
**Depends on:** TASK-WF-02-01, TASK-WF-02-02
**Status:** `[ ]` TODO

---

## Context

BUG-WF-02 finds no variable interpolation at all — step configs cannot
reference `ExecuteRequest.inputs_json` values or earlier steps' outputs.
This adds `domain.Interpolate` and wires it into `waveDispatcher` so each
step's `Config` is interpolated immediately before dispatch.

## Changes to make

Create `backend-go/services/workflow-service/internal/domain/interpolation.go`:

```go
package domain

// ExecutionContext carries everything a step's Config's {{...}} tokens
// can reference — built once per Execute call, threaded through every
// wave.
type ExecutionContext struct {
    Inputs    map[string]any            // from ExecuteRequest.inputs_json
    Outputs   map[string]map[string]any // stepId -> parsed OutputJSON, accumulated wave-by-wave
    ProjectID string
    UserID    string // TriggeredBy
}

var interpolationToken = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

// Interpolate replaces every {{...}} token in configJSON with its
// resolved value, leaving unresolvable tokens untouched (visible, not
// silently dropped) rather than failing the whole step.
func Interpolate(configJSON string, ctx ExecutionContext) (string, error) {
    result := interpolationToken.ReplaceAllStringFunc(configJSON, func(match string) string {
        expr := strings.TrimSpace(interpolationToken.FindStringSubmatch(match)[1])
        val, ok := resolveToken(expr, ctx)
        if !ok {
            return match
        }
        return jsonEscapeAndQuoteIfNeeded(val)
    })
    return result, nil
}

func resolveToken(expr string, ctx ExecutionContext) (any, bool) {
    switch {
    case expr == "now()":
        return time.Now().UTC().Format(time.RFC3339), true
    case expr == "project.id":
        return ctx.ProjectID, true
    case expr == "user.id":
        return ctx.UserID, true
    case strings.HasPrefix(expr, "outputs."):
        parts := strings.SplitN(strings.TrimPrefix(expr, "outputs."), ".", 2)
        if len(parts) != 2 {
            return nil, false
        }
        stepOut, ok := ctx.Outputs[parts[0]]
        if !ok {
            return nil, false
        }
        return digPath(stepOut, parts[1])
    default:
        val, ok := ctx.Inputs[expr]
        return val, ok
    }
}
```

`project.name`/other project metadata are deliberately not resolved in
this pass (would need an extra `project-service` call per step
dispatch) — `project.id` (already known locally) is provided now.

Wire into `wave_dispatcher.go`: add an `execCtx *domain.ExecutionContext`
field (guarded by a `sync.Mutex` for `Outputs` writes — steps within one
wave dispatch concurrently), built once in `Execute.Execute` from
`ExecuteRequest.inputs_json` + `exec.ProjectID` + `exec.TriggeredBy`, and
interpolate before every step dispatch:

```go
func (d *waveDispatcher) dispatchStep(ctx context.Context, step domain.Step, se domain.StepExecution) bool {
    interpolated, err := domain.Interpolate(string(step.Config), *d.execCtx)
    // ... error handling, same shape as today's runStep error path ...
    result, err := d.runStep(ctx, step /* with Config = interpolated */, &se)
    if err == nil && result.Status == domain.ResultStatusCompleted {
        var parsed map[string]any
        json.Unmarshal([]byte(result.OutputJSON), &parsed) // best-effort
        d.execCtxMu.Lock()
        d.execCtx.Outputs[step.ID] = parsed
        d.execCtxMu.Unlock()
    }
    // ...
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/domain/... -run TestInterpolate
go test ./services/workflow-service/internal/usecase/... -run TestWaveDispatcher -race
```

Expected: every token kind resolves (`{{feature_description}}` from
Inputs, `{{outputs.stepA.field}}` from Outputs, `{{project.id}}`,
`{{user.id}}`, `{{now()}}`); unresolvable token left as literal text; a
token embedded inside a larger JSON string value round-trips correctly.
`wave_dispatcher_test.go`: a 2-wave DAG where wave 1's step output is
referenced in wave 2's config — wave 2's executor receives the
interpolated value; concurrent writes to `execCtx.Outputs` within one
wave pass under `-race`.
