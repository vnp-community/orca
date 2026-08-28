# TASK-PRF-04-08: Inject profile env/model into `task-service`'s `SimpleExecutor`

**From Solution:** SOL-PRF-04
**Priority:** P0
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go`
**Depends on:** TASK-PRF-04-04, TASK-PRF-04-07
**Status:** `[ ]` TODO

---

## Context

`SimpleExecutor` already relays via the correct `"agent.execPrompt"` method
(confirmed by reading the file — no method-name fix needed here, unlike
`workflow-service`'s TASK-PRF-04-06) but its `agentExecPromptParams` only
sends `prompt`/`worktreePath`/`stepId` — no `trustPreset`/`model`/`env`. This
task adds all three, resolving the executing user's profile the same way
TASK-PRF-04-06 does for `workflow-service`. **Open question, flagged for the
implementer**: `task.AssigneeID` is assumed to exist on `domain.Task` per
SOL-PRF-04's Design section — confirm against the real `domain.Task` shape
before wiring; if tasks have no per-task assignee field, fall back to
whichever user id triggered task execution via the request context
(`common/tenant.UserID(ctx)`) instead.

## Changes to make

In `backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go`:

```go
type SimpleExecutor struct {
	tasks    usecase.TaskRepository
	resolver usecase.ProjectExecutionResolver
	relay    infrafleetv1.InfraFleetServiceClient
	profiles usecase.ProfileResolver        // NEW
	projects usecase.ProjectContextResolver // NEW
}

func NewSimpleExecutor(tasks usecase.TaskRepository, resolver usecase.ProjectExecutionResolver, relay infrafleetv1.InfraFleetServiceClient, profiles usecase.ProfileResolver, projects usecase.ProjectContextResolver) *SimpleExecutor {
	return &SimpleExecutor{tasks: tasks, resolver: resolver, relay: relay, profiles: profiles, projects: projects}
}

// agentExecPromptParams gains TrustPreset/Model/Env/InitFile — all
// omitempty, same "omit when unresolved" convention this file's doc comment
// already established for accountId/model.
type agentExecPromptParams struct {
	Prompt       string            `json:"prompt"`
	WorktreePath string            `json:"worktreePath"`
	StepID       string            `json:"stepId,omitempty"`
	TrustPreset  string            `json:"trustPreset,omitempty"` // NEW
	Model        string            `json:"model,omitempty"`       // NEW
	Env          map[string]string `json:"env,omitempty"`         // NEW
	InitFile     string            `json:"initFile,omitempty"`    // NEW — see workflow-service's agent_step_executor.go for the same field-name caveat
}

func (s *SimpleExecutor) Execute(ctx context.Context, tenantID, taskID, requestID string) (string, error) {
	task, err := s.tasks.Get(ctx, tenantID, taskID)
	if err != nil {
		return "", fmt.Errorf("simple_executor: load task: %w", err)
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

	params := agentExecPromptParams{Prompt: buildExecutePrompt(task), WorktreePath: worktreePath, StepID: requestID}

	// userID: task.AssigneeID if domain.Task carries one — CONFIRM against
	// the real domain.Task shape (open question, see this task's Context) —
	// else fall back to the request-context caller.
	userID := task.AssigneeID
	if userID == "" {
		userID, _ = tenant.UserID(ctx)
	}
	if userID != "" {
		resolved, err := s.profiles.GetResolvedProfile(ctx, userID)
		if err == nil { // best-effort — a profile-resolve failure degrades to the legacy bare passthrough, never blocks task execution
			if agent, ok := resolved["agent"].(map[string]any); ok {
				if model, ok := agent["preferredModel"].(string); ok {
					params.Model = model
				}
			}
			env := domain.BuildAgentEnv(resolved, userID, task.ProjectID, "", "")
			if task.ProjectID != "" {
				if pctx, err := s.projects.GetProjectContext(ctx, task.ProjectID); err == nil {
					env["ORCA_PROJECT_NAME"] = pctx.ProjectName
					params.InitFile = domain.BuildProjectContext(domain.PreambleInput{
						ProjectName: pctx.ProjectName, Description: pctx.Description, RepoURL: pctx.RepoURL,
						WorktreePath: worktreePath, DevServerHostname: pctx.DevServerHostname,
					})
				}
			}
			params.Env = env
		}
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("simple_executor: marshal params: %w", err)
	}
	resp, err := s.relay.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: connectionID, Method: "agent.execPrompt", ParamsJson: string(paramsJSON),
	})
	if err != nil {
		return "", fmt.Errorf("simple_executor: relay agent.execPrompt: %w", err)
	}
	var result agentExecPromptResult
	if err := json.Unmarshal([]byte(resp.GetResultJson()), &result); err != nil {
		return "", fmt.Errorf("simple_executor: unmarshal agent.execPrompt result: %w", err)
	}
	if result.TimedOut {
		return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_TIMED_OUT", "agent.execPrompt timed out before the task finished", nil)
	}
	if result.ExitCode == nil || *result.ExitCode != 0 {
		return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_FAILED", fmt.Sprintf("agent.execPrompt exited non-zero: %s", result.Stderr), nil)
	}
	return fmt.Sprintf("task-exec:%s:%s", taskID, requestID), nil
}
```

Add `"github.com/stablyai/orca-go/common/tenant"` and
`"github.com/stablyai/orca-go/services/task-service/internal/domain"` to the
file's imports.

Update the `NewSimpleExecutor(tasks, resolver, relay)` call site in
`cmd/server/main.go` to pass the two new resolvers constructed in
TASK-PRF-04-07: `NewSimpleExecutor(tasks, resolver, relay, profileResolver,
projectContextResolver)`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
```

Update `simple_executor_test.go` per SOL-PRF-04's Test plan (same coverage
shape as `workflow-service`'s `agent_step_executor_test.go`): confirm the
relay call's method string stays `"agent.execPrompt"` (no regression — this
service was already correct); a resolvable user id populates `env`/`model`;
a `ProfileResolver` error degrades to the legacy bare passthrough rather
than failing task execution; `ProjectContextResolver` failure -> spawn still
proceeds with `InitFile == ""`.

```bash
go test ./services/task-service/internal/adapter/grpcclient/... -v
go test ./backend-go/services/workflow-service/... ./backend-go/services/task-service/... ./backend-go/services/project-service/... -v
```

Expected: full green across all three services this SOL touches — this is
the last task in the PRF-04 set.
