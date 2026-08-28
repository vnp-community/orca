# TASK-WF-02-06: Add `{{...}}` variable interpolation + wire into wave dispatch

**From Solution:** SOL-WF-02
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/domain/interpolation.go` (new)
**Depends on:** TASK-WF-02-01, TASK-WF-02-02
**Status:** `[x]` DONE — new `domain/interpolation.go`: `ExecutionContext`, `Interpolate`, `resolveToken`, `digPath`, `jsonEscapeAndQuoteIfNeeded` (string values unquoted-and-escaped for mid-string embedding; non-string values rendered as raw JSON). Wired into `wave_dispatcher.go` via a new `executionContext` (ExecutionContext + its own `sync.Mutex`) threaded EXPLICITLY through `dispatchWaves`/`dispatchWavesFrom`/`dispatchWave`/`dispatchStep` as a parameter — deliberately NOT a `waveDispatcher` struct field, since `waveDispatcher` is one shared instance across every execution this process runs (constructed once in `NewExecute`/`NewExecuteAdHocStep`/`NewRecoverExecutions`); a struct field would let concurrent executions corrupt each other's Inputs/Outputs. `dispatchStep` also enriches `ctx` with `tenant.WithUserID`/`WithProjectID` (added in TASK-WF-02-05) so `AgentExecutor`'s `ProviderResolver` call now genuinely receives the triggering user/project — closing that task's documented gap. `Execute.Execute` gained `ExecuteInput.InputsJSON` (wired from the proto's `inputs_json`, TASK-WF-02-01) and fails synchronously on malformed JSON; `RecoverExecutions` reconstructs `Outputs` from completed steps' persisted `OutputJSON` (Inputs is NOT recoverable post-crash — `inputs_json` was never persisted — documented as a known gap). `ExecuteAdHocStep` intentionally untouched: it calls `runStep` directly, bypassing `dispatchStep`, so ad hoc steps get no interpolation (no template-level inputs_json exists for them). New tests: 8 `domain/interpolation_test.go` cases (all token kinds, unresolvable-left-literal, special-char escaping, numeric standalone value, multi-token) + 4 `wave_dispatcher_test.go` cases (cross-wave output interpolation, project/user tokens, concurrent output writes under `-race`, unresolvable-token-literal). Found and fixed a genuine pre-existing bug while adding the concurrent-writes test: `fakeStepExecutor` (shared across concurrent goroutines in wave-dispatch tests) had no mutex — this is the exact root cause of the data race TASK-WF-02-04 had flagged as pre-existing in `TestWaveDispatcher_DispatchWave_BoundsConcurrency`; fixed by adding a mutex to the fake. `go build/vet/test ./... -race` now green across the whole workflow-service package (previously only non-race was green).

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
