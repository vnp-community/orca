# TASK-PRF-04-06: Fix `agent.exec` -> `agent.execPrompt` and inject profile env into `workflow-service`'s `AgentExecutor`

**From Solution:** SOL-PRF-04
**Priority:** P0 — this is BUG-PRF-04's core fix; everything else in this set exists to feed this
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`
**Depends on:** TASK-PRF-04-03, TASK-PRF-04-05
**Status:** `[x]` DONE — agent.exec -> agent.execPrompt fixed + profile-aware env injection wired; AgentStepConfig gains UserID/ProjectID; main.go wired (tenant-service+project-service dials); full workflow-service suite green

---

## Context

Confirmed by reading the real file: `AgentExecutor` today relays via
`agentExecMethod = "agent.exec"` — its own doc comment already flags this as
wrong (`"agent.exec" IS a real agent RPC — just a different one: a generic
{binary,args,cwd,stdin,env,timeoutMs} process-exec call with no
prompt/model/trustPreset concept`), citing exactly the fix
`task-service`'s `SimpleExecutor` already made
(`"agent.execPrompt"`, confirmed correct by reading `simple_executor.go`).
This task is a **prerequisite fix** for env injection, not an optional
cleanup: sending `env`/`model` to `"agent.exec"` would hit a handler with no
concept of either field. `AgentStepConfig` also gains `UserID`/`ProjectID`
so a step config carries what the profile-aware path needs.

## Changes to make

In `backend-go/services/workflow-service/internal/domain/step.go`, extend
`AgentStepConfig`:

```go
type AgentStepConfig struct {
	ConnectionID string `json:"connectionId"`
	Prompt       string `json:"prompt"`
	WorktreePath string `json:"worktreePath,omitempty"`
	TrustPreset  string `json:"trustPreset,omitempty"`
	UserID       string `json:"userId,omitempty"`    // NEW — whose profile to resolve; empty = legacy passthrough, see below
	ProjectID    string `json:"projectId,omitempty"` // NEW — for GetProjectContext + ORCA_PROJECT_*
}
```

Replace `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`
in full:

```go
package infrafleetclient

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// agentExecPromptMethod replaces the prior, wrong "agent.exec" — see this
// file's Context. Regression-tested directly: agent_step_executor_test.go
// asserts the relay call's method string unconditionally.
const agentExecPromptMethod = "agent.execPrompt"

type agentExecPromptParams struct {
	Prompt       string            `json:"prompt"`
	WorktreePath string            `json:"worktreePath"`
	TrustPreset  string            `json:"trustPreset,omitempty"`
	Model        string            `json:"model,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	InitFile     string            `json:"initFile,omitempty"` // project-context preamble — field name UNVERIFIED against the real agent handler, see this task's Verify section
}

// AgentExecutor is the real Agent step executor — relays a prompt-driven
// agent invocation to infra-fleet-service's Relay RPC, method
// "agent.execPrompt". Profile-aware: when the step config carries a
// UserID, resolves the caller's ResolvedProfile and injects it into the
// spawned process's env — see domain/agent_environment.go's BuildAgentEnv.
type AgentExecutor struct {
	client   infrafleetv1.InfraFleetServiceClient
	profiles usecase.ProfileResolver
	projects usecase.ProjectContextResolver
}

func NewAgentExecutor(client infrafleetv1.InfraFleetServiceClient, profiles usecase.ProfileResolver, projects usecase.ProjectContextResolver) *AgentExecutor {
	return &AgentExecutor{client: client, profiles: profiles, projects: projects}
}

var _ domain.StepExecutor = (*AgentExecutor)(nil)

func (e *AgentExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg domain.AgentStepConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: invalid step config JSON: %w", err)
	}

	params := agentExecPromptParams{Prompt: cfg.Prompt, WorktreePath: cfg.WorktreePath, TrustPreset: cfg.TrustPreset}

	// Profile-aware path only when UserID is present — a step authored
	// before this migration (no UserID in its config JSON) degrades to a
	// bare passthrough rather than failing outright. An expand/contract
	// compatibility shim, not a permanent branch.
	if cfg.UserID != "" {
		resolved, err := e.profiles.GetResolvedProfile(ctx, cfg.UserID)
		if err != nil {
			return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: resolve profile: %w", err)
		}
		// agent.execPrompt's model field wants the raw model NAME, not a
		// binary path — the agent-side handler resolves its own binary from
		// the name. domain.ResolveAgentBinary/AgentBinaryMap are NOT called
		// here; they exist only for a future richer spawn RPC (see
		// agent_environment.go's package doc comment).
		if agent, ok := resolved["agent"].(map[string]any); ok {
			if model, ok := agent["preferredModel"].(string); ok {
				params.Model = model
			}
		}

		existingPath := "" // this service has no visibility into the target host's existing PATH — env.PATH is additive-only from pathAdditions
		env := domain.BuildAgentEnv(resolved, cfg.UserID, cfg.ProjectID, "", existingPath)

		if cfg.ProjectID != "" {
			pctx, err := e.projects.GetProjectContext(ctx, cfg.ProjectID)
			if err == nil { // best-effort — a preamble-build failure must never block the agent spawn itself
				env["ORCA_PROJECT_NAME"] = pctx.ProjectName
				params.InitFile = domain.BuildProjectContext(domain.PreambleInput{
					ProjectName: pctx.ProjectName, Description: pctx.Description, RepoURL: pctx.RepoURL,
					WorktreePath: cfg.WorktreePath, DevServerHostname: pctx.DevServerHostname,
				})
			}
		}
		params.Env = env
	}

	var result execResult
	if err := relay(ctx, e.client, cfg.ConnectionID, agentExecPromptMethod, params, &result); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: %w", err)
	}
	return toStepResult(result)
}
```

Update the `NewAgentExecutor(client)` call site in `cmd/server/main.go` to
pass the two new resolvers constructed in TASK-PRF-04-05:
`NewAgentExecutor(infraFleetClient, profileResolver, projectContextResolver)`.

Optional, out of this task's strict scope but flagged as a needed follow-up:
add `WorkflowExecutionChecker`-style health-gate before the relay call,
reusing SOL-PRF-03's `DevServerHealthChecker` port, per SOL-PRF-04's
"server-unavailability" design note — not required for the core env-fix,
tracked separately.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
```

Add/update `agent_step_executor_test.go` per SOL-PRF-04's Test plan: fake
`ProfileResolver`/`ProjectContextResolver` — `cfg.UserID` empty -> legacy
passthrough (`agent.execPrompt` called with no `env`/`model`); assert the
relay call's method string is `"agent.execPrompt"` **unconditionally** —
this is the hard regression test for the stale `"agent.exec"` bug this task
fixes. `cfg.UserID` set -> `ProfileResolver` called with the right userID,
`env` populated, `params.Model` equals the raw resolved model string (not a
binary name). `ProjectContextResolver` failure -> spawn still proceeds
(best-effort `InitFile`), relay call still happens with `InitFile == ""`.

```bash
go test ./services/workflow-service/internal/adapter/infrafleetclient/... -v
```

**Open verification item, not resolved by this task**: re-read
`agent-print-mode-exec.ts`'s real handler in full to confirm it has an
`initFile`-equivalent field for the preamble (`env`/`model`/`trustPreset`/
`stepId` are already confirmed by `task-service`'s `simple_executor.go` doc
comment; the preamble field name is not). If it doesn't accept one, the
workaround is prepending the preamble into `prompt` itself instead of a
separate `InitFile` field — flag to whoever picks this up.
