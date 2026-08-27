package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// SimpleExecutor implements usecase.SimpleExecutor for real (TASK-224),
// replacing the prior StubSimpleExecutor — dispatches Execute's simple path
// to infra-fleet-service's Relay RPC. See task-service.md §3.1 and
// TASK-224's Context note for why task-service goes through
// infra-fleet-service rather than dialing the Dev Server Agent itself
// (task-service.md §2/§3.1's "only two Go services that talk to the
// execution plane" rule — infra-fleet-service and git-gateway-service —
// task-service isn't one of them).
//
// TASK-224 Gap 1, closed: relays via "agent.execPrompt", not "agent.exec".
// Read both RPC handlers in full in agent/src/relay/agent-rpc-dispatch.ts
// before making this switch:
//   - case 'agent.exec' (agent-rpc-dispatch.ts:913-982): a generic
//     "run this literal binary" primitive — params
//     {binary(required), args?, cwd?, stdin?, env?, timeoutMs?}, result
//     {stdout, stderr, exitCode, timedOut}. Its own doc comment says its
//     real callers are "StepExecutors.executeAgent() via
//     relay.call('agent.exec', ...)" and "ProfileAwareAgentSpawner" — but
//     that comment is stale: reading the real TS source
//     (backend/src/main/workflow/StepExecutors.ts:1-19,114-124) shows it
//     was rewritten to call 'agent.execPrompt' instead, specifically
//     because "agent.exec IS a real agent RPC — just a different one: a
//     generic {binary,args,cwd,stdin,env,timeoutMs} process-exec call with
//     no prompt/model/trustPreset concept — sending this step's
//     domain-shaped payload to it always failed with InvalidParams."
//   - case 'agent.execPrompt' (agent-rpc-dispatch.ts:992-1000) delegates to
//     handleAgentExecPrompt (agent-print-mode-exec.ts:33-178), which is the
//     real "dispatch an AI-driven task" handler: required params
//     `prompt` (string) and `worktreePath` (string, becomes the spawned
//     CLI's cwd) — see agent-print-mode-exec.ts:39-73's destructuring +
//     validation. Optional: `stepId` (echoed back, used for tracing),
//     `trustPreset` ("full" appends the YOLO flag, anything else is a
//     no-op — agent-print-mode-exec.ts:44,97-99), `model` (defaults to
//     "claude" when absent or empty — agent-print-mode-exec.ts:45; only
//     "claude"-prefixed models are supported for one-shot exec today,
//     agent-print-mode-exec.ts:76-94), `accountId` (agent-print-mode-exec.ts:46,
//     109-116 — buildAgentEnv errors if set but no resolvedApiKey is
//     forwarded, which no live caller does yet; omitted here relies on the
//     CLI's own already-authenticated state, same as every other caller),
//     `env`, `timeoutMs`. Result:
//     {stdout, stderr, exitCode, timedOut, stepId} — no `executionRef`
//     field exists on EITHER method's real result shape (the prior
//     "agent.exec" wiring's `{executionRef}` unmarshal target was already a
//     fabrication, not something either RPC actually returns).
//   - The real, already-proven param-construction pattern is
//     StepExecutors.executeAgent() (backend/src/main/workflow/StepExecutors.ts:101-131):
//     `relay.call('agent.execPrompt', { stepId, prompt, worktreePath,
//     trustPreset: step.config['trustPreset'] ?? 'default', traceId,
//     ...(resolved ? { accountId: resolved.accountId, model: resolved.model } : {}) })`
//     — accountId/model are omitted ENTIRELY (not even as undefined keys)
//     "when no override AND no scope match ... the dev server's agent.exec
//     handler then falls back to its own pre-fix default account,
//     preserving current behavior for workflows that never pin a
//     provider" (StepExecutors.ts:120-122). SimpleExecutor follows that
//     same omit-when-unresolved convention below: task-service has no
//     per-task AI-provider-account pin today (unlike AIDecompose, this
//     path never calls AIProviderContextResolver), so accountId/model are
//     left out of the params entirely rather than guessed at.
//
// worktreePath resolution: infra-fleet-service's ResolveConnectionResponse
// already carries a repo_path field for exactly this purpose (see
// usecase.ProjectExecutionResolver's doc comment and
// ProjectExecutionResolver.ResolveConnection's implementation, which reads
// resp.GetRepoPath() — the same field git-gateway-service's
// ConnectionResolver reads for its own GitExecutor's cwd equivalent,
// internal/usecase/ports.go:26-37 in that service).
//
// prompt construction: this codebase's one existing AI-task-execution
// prompt-building convention is AIDecompose's buildDecomposePrompt
// (ai_decompose.go) — plain text naming the task's title, no structured
// (JSON) request format, because there is no live agent contract yet to
// confirm one against. buildExecutePrompt below follows that same
// plain-text convention for consistency, not a new one invented here.
//
// Honest limit carried over unchanged, not solved by this task:
// task-service still has no execution-completion callback. Unlike
// agent.exec's original (never-real) `executionRef` sketch,
// agent.execPrompt actually blocks until the CLI process exits (or times
// out, up to 15 minutes — agent-print-mode-exec.ts's MAX_TIMEOUT_MS) and
// returns its exit status synchronously in the same RPC response. This
// method therefore synthesizes its own executionRef from taskID+requestID
// (no ID of that kind exists in agent.execPrompt's result to reuse) and
// surfaces a non-zero exit / timeout as a real error rather than an
// "executionRef" pointing at a run that already finished.
type SimpleExecutor struct {
	tasks    usecase.TaskRepository
	edges    usecase.EdgeRepository
	resolver usecase.ProjectExecutionResolver
	relay    infrafleetv1.InfraFleetServiceClient
	profiles usecase.ProfileResolver        // NEW
	projects usecase.ProjectContextResolver // NEW
}

func NewSimpleExecutor(tasks usecase.TaskRepository, edges usecase.EdgeRepository, resolver usecase.ProjectExecutionResolver, relay infrafleetv1.InfraFleetServiceClient, profiles usecase.ProfileResolver, projects usecase.ProjectContextResolver) *SimpleExecutor {
	return &SimpleExecutor{tasks: tasks, edges: edges, resolver: resolver, relay: relay, profiles: profiles, projects: projects}
}

// agentExecPromptParams mirrors agent-print-mode-exec.ts's handled fields —
// see this file's doc comment for the exact source citation. stepId/
// trustPreset/model/accountId are `omitempty`: SimpleExecutor has no value
// to put in the latter two (see the doc comment's "omit when unresolved"
// note), and an empty stepId/trustPreset should vanish from the JSON the
// same way StepExecutors.ts's spread omits accountId/model, not be sent as
// an explicit empty string.
//
// TrustPreset/Model/Env/InitFile are NEW (TASK-PRF-04-08) — same
// profile-aware env injection as workflow-service's AgentExecutor; see
// domain/agent_environment.go's BuildAgentEnv/BuildProjectContext.
type agentExecPromptParams struct {
	Prompt       string            `json:"prompt"`
	WorktreePath string            `json:"worktreePath"`
	StepID       string            `json:"stepId,omitempty"`
	TrustPreset  string            `json:"trustPreset,omitempty"` // NEW
	Model        string            `json:"model,omitempty"`       // NEW
	Env          map[string]string `json:"env,omitempty"`         // NEW / TASK-TG-04-06 — already-supported field, simply never populated before
	InitFile     string            `json:"initFile,omitempty"`    // NEW — see workflow-service's agent_step_executor.go for the same field-name caveat
}

// agentExecPromptResult mirrors agent-print-mode-exec.ts's real
// PrintModeExecResult shape (agent-print-mode-exec.ts:25-31,177) — no
// executionRef field exists on the real RPC; see this file's doc comment.
type agentExecPromptResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode *int   `json:"exitCode"`
	TimedOut bool   `json:"timedOut"`
}

func (s *SimpleExecutor) Execute(ctx context.Context, tenantID, taskID, requestID string) (string, error) {
	task, err := s.tasks.Get(ctx, tenantID, taskID)
	if err != nil {
		return "", fmt.Errorf("simple_executor: load task: %w", err)
	}

	// Context preamble inputs (TASK-TG-04-06): parent is ancestors[1] (the
	// immediate parent, nil for a root task — ancestors[0] is task itself,
	// GetAncestors' own convention); completedDeps is this task's
	// depends_on targets currently Status == StatusDone. Both are
	// best-effort — a lookup failure here degrades the prompt, it must
	// never fail the whole dispatch.
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

	connectionID, worktreePath, _, connected, err := s.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil {
		return "", fmt.Errorf("simple_executor: resolve connection: %w", err)
	}
	if !connected {
		// Per git-gateway-service.md §8's precedent: a resolve failure or
		// not-connected connectionId is a real error, never a silent local
		// fallback — task-service has no local agent.execPrompt equivalent
		// of its own (unlike git-gateway-service's §2 step 3, there is no
		// "this service's own host" case for task execution).
		return "", apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_CONNECTION", "task's project has no connected dev server", nil)
	}
	if worktreePath == "" {
		// agent.execPrompt requires worktreePath (agent-print-mode-exec.ts:62-73
		// rejects a missing one with InvalidParams) — a connected connection
		// with no repo_path recorded is a distinct, real error, not a
		// silent no-op dispatch.
		return "", apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_WORKTREE_PATH", "task's connected dev server has no worktree path recorded", nil)
	}

	// TASK-TG-04-06's context preamble (parent/completedDeps) and
	// TASK-PRF-04-08's profile-aware env/model/init-file resolution are
	// independent enhancements to the same dispatched prompt — composed
	// here rather than picking one: the prompt always carries the task's
	// dependency context, and the env always at least carries the base
	// ORCA_TASK_ID/ORCA_PROJECT_ID pair (TASK-TG-04-06) even when no
	// profile resolves.
	params := agentExecPromptParams{
		Prompt:       buildExecutePrompt(task, parent, completedDeps),
		WorktreePath: worktreePath,
		StepID:       requestID,
		Env:          map[string]string{"ORCA_TASK_ID": task.ID, "ORCA_PROJECT_ID": task.ProjectID},
	}

	// userID: task-service's domain.Task carries no per-task assignee field
	// (confirmed against the real domain.Task shape — see this task's
	// Context open question) — fall back to the request-context caller.
	userID, _ := tenant.UserID(ctx)
	if userID != "" {
		userCtx := tenant.WithUserID(ctx, userID) // explicit, for the resolvers' outbound-metadata forwarding below
		resolved, err := s.profiles.GetResolvedProfile(userCtx, userID)
		if err == nil { // best-effort — a profile-resolve failure degrades to the legacy bare passthrough (still carrying TG-04-06's base env above), never blocks task execution
			if agent, ok := resolved["agent"].(map[string]any); ok {
				if model, ok := agent["preferredModel"].(string); ok {
					params.Model = model
				}
			}
			env := domain.BuildAgentEnv(resolved, userID, task.ProjectID, "", "")
			env["ORCA_TASK_ID"] = task.ID // preserve TASK-TG-04-06's task-id env var alongside the profile-resolved env
			if task.ProjectID != "" {
				if pctx, err := s.projects.GetProjectContext(userCtx, task.ProjectID); err == nil {
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
	// TASK-TG-04-07: persist this run's stdout so a LATER ExecuteBatch
	// wave's buildExecutePrompt can resolve `{{outputs.<taskId>.*}}`
	// against this task once it's a completed dependency — best-effort,
	// a write failure here must not fail an otherwise-successful run.
	_ = s.tasks.UpdateLastExecutionOutput(ctx, tenantID, taskID, result.Stdout)
	return fmt.Sprintf("task-exec:%s:%s", taskID, requestID), nil
}

// buildExecutePrompt assembles the agent.execPrompt prompt from task,
// parent (nil if task is a root task), and completedDeps (this task's
// depends_on targets currently Status == StatusDone) — see this file's doc
// comment for why this follows ai_decompose.go's buildDecomposePrompt
// plain-text convention rather than a new one. Uses task.PromptTemplate
// verbatim when set — a task with an AI-generated or user-edited prompt
// template should use it as-is (SOL-TG-01/SOL-TG-02) — falling back to the
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
	return interpolateOutputs(b.String(), completedDeps)
}

// interpolateOutputs resolves `{{outputs.<taskId>.*}}` and
// `{{outputs.<taskId>.stdout}}` tokens (SOL-TG-04's batch-wave prompt
// interpolation, TASK-TG-04-07) against completedDeps' LastExecutionOutput
// (persisted by a PRIOR ExecuteBatch wave's SimpleExecutor.Execute) — the
// only field currently captured is stdout, so both the wildcard and the
// explicit `.stdout` token resolve to the same value. A token naming a
// task not in completedDeps (never ran, not yet finished, or not this
// task's dependency at all) is left verbatim rather than silently
// stripped, so a broken reference is visible in the dispatched prompt
// instead of vanishing.
func interpolateOutputs(prompt string, completedDeps []domain.Task) string {
	if len(completedDeps) == 0 {
		return prompt
	}
	for _, d := range completedDeps {
		prompt = strings.ReplaceAll(prompt, fmt.Sprintf("{{outputs.%s.stdout}}", d.ID), d.LastExecutionOutput)
		prompt = strings.ReplaceAll(prompt, fmt.Sprintf("{{outputs.%s.*}}", d.ID), d.LastExecutionOutput)
	}
	return prompt
}
