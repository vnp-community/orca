# TASK-TG-005-03: `SimpleExecutor` env injection (`ORCA_TASK_ID`) + richer `buildExecutePrompt`

**From Solution:** BE-SOL-005
**Priority:** P2
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go`
**Depends on:** TASK-TG-001-02/TASK-TG-002-01 (`Task.Description`/`Task.AIContext`/`Task.PromptTemplate` fields the richer prompt reads)
**Status:** `[ ]` TODO

---

## Context

**Zero `agent/`-side change needed for the env-injection half — confirmed,
cite [SOL-AG-TG-001](../../../../../agent/crs/v4/task-graph/solutions/SOL-AG-TG-001-agent-env-contract-assessment.md),
don't re-derive.** That assessment confirms `agent-print-mode-exec.ts`'s
`params.env` is already accepted and merged on top of `buildAgentEnv()`'s
base env, meaning a backend-go-supplied `env.ORCA_TASK_ID` already overrides
the auto-derived (and wrong) `taskId: stepId ?? ''` value `buildAgentEnv`
uses internally. This task is backend-go-only.

Verified directly (`internal/adapter/grpcclient/simple_executor.go:1-197`,
read in full): `agentExecPromptParams` (lines 116-120) today has
`{Prompt, WorktreePath, StepID}` — no `Env` field. `Execute` (lines
132-186) builds `paramsJSON` at lines 161-165 with no `Env` key.
`buildExecutePrompt` (lines 191-197) is the current, minimal
`"Complete the following task.\n\nTask: <title>"` — confirmed exactly 2
lines of content, no `Description`/`AIContext`/dependencies/
`PromptTemplate` interpolation.

`AIDecompose`'s `buildDecomposePrompt` (`ai_decompose.go:79-90`) is this
file's own cited precedent for the plain-text convention — kept, not
switched to JSON: *"this codebase's one existing AI-task-execution
prompt-building convention... plain text... because there is no live agent
contract yet to confirm a structured one against"* (this file's own doc
comment, lines 82-87).

## Changes to make

**1. `agentExecPromptParams`** (lines 116-120) — add `Env`:

```go
type agentExecPromptParams struct {
	Prompt       string            `json:"prompt"`
	WorktreePath string            `json:"worktreePath"`
	StepID       string            `json:"stepId,omitempty"`
	Env          map[string]string `json:"env,omitempty"` // NEW
}
```

**2. `Execute`** (lines 132-186) — set `Env` explicitly using `taskID`, NOT
`requestID`/`StepID`:

```go
paramsJSON, err := json.Marshal(agentExecPromptParams{
	Prompt:       effectivePrompt,
	WorktreePath: worktreePath,
	StepID:       requestID,
	Env:          map[string]string{"ORCA_TASK_ID": taskID, "ORCA_PROJECT_ID": task.ProjectID}, // NEW
})
```

This closes the real bug BE-SOL-005 identifies: without an explicit `Env`,
`buildAgentEnv` (agent-side) derives `taskId` from `stepId`, which is
actually `requestID` here (`SimpleExecutor.Execute`'s own parameter is
named `requestID`, echoed into `StepID` at line 164) — a different,
per-execution-attempt identifier, not the stable task ID. Setting
`Env["ORCA_TASK_ID"]` explicitly overrides that mistaken derivation.

**3. `buildExecutePrompt`** (lines 191-197) — widen to accept dependency
tasks and interpolate the new `Task` fields from TASK-TG-001-02/-002-01:

```go
func buildExecutePrompt(task domain.Task, deps []domain.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Complete the following task.\n\nTask: %s\n", task.Title)
	if task.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", task.Description)
	}
	if task.AIContext != "" {
		fmt.Fprintf(&b, "\nContext:\n%s\n", task.AIContext)
	}
	if len(deps) > 0 {
		fmt.Fprintf(&b, "\nAlready completed (dependencies):\n%s\n", summarize(deps))
	}
	if task.PromptTemplate != "" {
		fmt.Fprintf(&b, "\nSpecific instructions:\n%s\n", task.PromptTemplate)
	}
	return b.String()
}
```

`buildExecutePrompt`'s call site (`Execute`, line 159:
`effectivePrompt = buildExecutePrompt(task)`) needs a `deps []domain.Task`
argument now — `SimpleExecutor` doesn't have `EdgeRepository` access today
(confirmed: `SimpleExecutor` struct, lines 99-103, has only
`tasks/resolver/relay` fields, no `edges`). Add an `edges
usecase.EdgeRepository` field + constructor parameter, and fetch
`depends_on` dependencies via `edges.ListFrom(ctx, tenantID, taskID,
domain.EdgeKindDependsOn)` (the same method `ExecuteTask.isComplex` already
calls, `execute_task.go:94`) before calling `buildExecutePrompt`. Add a
`summarize(deps []domain.Task) string` helper (simple: one line per dep,
title + status).

`buildExecutePrompt` stays plain text per this file's own documented
convention — do not switch to JSON or any new structured format.

## Test plan

- `agentExecPromptParams.Env` contains the real `taskID` (not `requestID`)
  — integration test against a fake agent relay asserting the JSON payload
  (per BE-SOL-005's own test plan item).
- `buildExecutePrompt` snapshot test with `Description`/`AIContext`/
  `PromptTemplate` all populated, and a separate test with all three empty
  (regression against the old minimal 2-line output for a task with none of
  the new fields set).
- `buildExecutePrompt` with 2 dependency tasks includes both in the
  "Already completed" section.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/adapter/grpcclient/... -run "TestSimpleExecutor|TestBuildExecutePrompt" -v
```

Expected: clean build; the `Env` assertion test is the one regression proof
this task exists to add — before this fix, `ORCA_TASK_ID` was never sent at
all; `buildExecutePrompt` snapshot tests pass for both the fully-populated
and empty-fields cases.
