# TASK-TG-04-06: `buildExecutePrompt` context preamble + `env` map injection in `SimpleExecutor`

**From Solution:** SOL-TG-04
**Priority:** P2
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go`
**Depends on:** TASK-TG-01-04 (`Description`/`AIContext`/`PromptTemplate` fields)
**Status:** [x] DONE — buildExecutePrompt now assembles PromptTemplate-or-generic-opener + Description/AIContext/parent/completedDeps sections, each cleanly omitted when absent; SimpleExecutor gained an edges usecase.EdgeRepository field and resolves parent (via GetAncestors) + completed depends_on targets (via ListFrom + per-dep Get, Status==StatusDone) before dispatch — both best-effort, a lookup failure degrades the prompt rather than failing dispatch. agentExecPromptParams.Env now always carries ORCA_TASK_ID/ORCA_PROJECT_ID. `go test ./services/task-service/internal/adapter/grpcclient/... -run 'TestSimpleExecutor|TestBuildExecutePrompt'` passes (17/17: golden-output cases for each optional section, env-map assertion, and an integration-level completed-deps-thread-into-prompt check); every backend-go service builds clean.

---

## Context

`buildExecutePrompt` today only writes `"Complete the following task.\n\nTask:
<title>"` — no description, no AI context, no parent-task context, no
completed-dependency context, and no `prompt_template` use even though
`SOL-TG-01`/`SOL-TG-02` add it. Separately, `SimpleExecutor`'s own doc
comment already establishes that the real `agent.execPrompt` RPC accepts an
optional `env` map (`simple_executor.go:52-54`) — this field exists on the
live Dev Server Agent contract today and is simply never populated. No
`agent/` change is needed for either fix — both use already-supported
fields.

## Changes to make

Replace `buildExecutePrompt` in
`backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go`:

```go
// buildExecutePrompt assembles the agent.execPrompt prompt from task,
// parent (nil if task is a root task), and completedDeps (this task's
// depends_on targets currently Status == StatusDone). Uses
// task.PromptTemplate verbatim when set — a task with an AI-generated or
// user-edited prompt template should use it as-is — falling back to the
// generic "Complete the following task" opener only when empty.
func buildExecutePrompt(task domain.Task, parent *domain.Task, completedDeps []domain.Task) string {
	var b strings.Builder
	if task.PromptTemplate != "" {
		b.WriteString(task.PromptTemplate)
		b.WriteString("\n\n")
	} else {
		b.WriteString("Complete the following task.\n\n")
	}
	fmt.Fprintf(&b, "Task: %s\n", task.Title)
	if task.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", task.Description)
	}
	if task.AIContext != "" {
		fmt.Fprintf(&b, "Context: %s\n", task.AIContext)
	}
	if parent != nil {
		fmt.Fprintf(&b, "\nParent task: %s\n%s\n", parent.Title, parent.Description)
	}
	if len(completedDeps) > 0 {
		b.WriteString("\nCompleted dependencies:\n")
		for _, d := range completedDeps {
			fmt.Fprintf(&b, "- %s: %s\n", d.Title, d.Description)
		}
	}
	return b.String()
}
```

Add `"fmt"` to this file's imports (`strings` is already there).

`SimpleExecutor.Execute` needs `parent`/`completedDeps` — resolve via
`TaskRepository.GetAncestors` (parent is `ancestors[1]` if
`len(ancestors) > 1`) and `EdgeRepository.ListFrom(...,
EdgeKindDependsOn)` filtered to `Status == StatusDone`, both already-existing
calls, not duplicated logic:

```go
func (s *SimpleExecutor) Execute(ctx context.Context, tenantID, taskID, requestID string) (string, error) {
	task, err := s.tasks.Get(ctx, tenantID, taskID)
	if err != nil {
		return "", fmt.Errorf("simple_executor: load task: %w", err)
	}

	var parent *domain.Task
	if ancestors, err := s.tasks.GetAncestors(ctx, tenantID, taskID, 0); err == nil && len(ancestors) > 1 {
		p := ancestors[1]
		parent = &p
	}
	var completedDeps []domain.Task
	if deps, err := s.edges.ListFrom(ctx, tenantID, taskID, domain.EdgeKindDependsOn); err == nil {
		for _, e := range deps {
			if t, err := s.tasks.Get(ctx, tenantID, e.ToTaskID); err == nil && t.Status == domain.StatusDone {
				completedDeps = append(completedDeps, t)
			}
		}
	}

	connectionID, worktreePath, connected, err := s.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil {
		return "", fmt.Errorf("simple_executor: resolve connection: %w", err)
	}
	if !connected {
		return "", apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_CONNECTION", "task's project has no connected dev server", nil)
	}
	if worktreePath == "" {
		return "", apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_WORKTREE_PATH", "task's connected dev server has no worktree path recorded", nil)
	}

	paramsJSON, err := json.Marshal(agentExecPromptParams{
		Prompt:       buildExecutePrompt(task, parent, completedDeps),
		WorktreePath: worktreePath,
		StepID:       requestID,
		Env:          map[string]string{"ORCA_TASK_ID": task.ID, "ORCA_PROJECT_ID": task.ProjectID},
	})
	// ...unchanged from here: Relay call, result unmarshal, timeout/exit-code checks...
}
```

`SimpleExecutor` needs an `edges usecase.EdgeRepository` field added
alongside its existing `tasks`/`resolver`/`relay` fields — update
`NewSimpleExecutor`'s signature and its one call site in `main.go`.

Widen `agentExecPromptParams` (already has `Prompt`/`WorktreePath`/`StepID`):

```go
type agentExecPromptParams struct {
	Prompt       string            `json:"prompt"`
	WorktreePath string            `json:"worktreePath"`
	StepID       string            `json:"stepId,omitempty"`
	Env          map[string]string `json:"env,omitempty"` // new — already-supported field, simply never populated before
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/adapter/grpcclient/... -run TestSimpleExecutor -v
```

Expected: `buildExecutePrompt` golden-output tests — description/aiContext/
parent/completedDeps each appear when present, are cleanly omitted when
absent (no empty `"Description: "` lines); a set `PromptTemplate` replaces
the generic opener; `env` map always contains `ORCA_TASK_ID`/
`ORCA_PROJECT_ID` in the marshaled `agentExecPromptParams`. Existing
`TestSimpleExecutor_Execute_RelaysAgentExecPrompt` and friends still pass.
