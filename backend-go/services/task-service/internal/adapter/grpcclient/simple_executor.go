package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/agentrelay"
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
// The agent.execPrompt params and result are built and decoded by common/agentrelay (golden wire files in
// agentrelay/testdata/wire; simple_executor_wire_test.go proves this executor sends the same JSON as before). What stays
// here: the `{{outputs.*}}` prompt interpolation, the profile/project env and init file, the full-trust policy, the
// contract-run options and the absence of an RPC deadline (agentrelay.WithoutCallDeadline).
//
// worktreePath resolution: infra-fleet-service's ResolveConnectionResponse
// already carries a repo_path field for exactly this purpose (see
// usecase.ProjectExecutionResolver's doc comment and
// ProjectExecutionResolver.ResolveConnection's implementation, which reads
// resp.GetRepoPath() — the same field git-gateway-service's
// ConnectionResolver reads for its own GitExecutor's cwd equivalent,
// internal/usecase/ports.go:26-37 in that service).
//
// prompt construction: the default prompt is plain text naming the task's
// title (backend-go/prompts/task_execution), the same convention as the
// decompose prompt, with no structured (JSON) request format. The mapping
// from a task to that prompt is execute_prompt_mapping.go.
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
	// agent builds and sends agent.execPrompt; the tenant metadata is stamped by runAgentPrompt because the same
	// context also feeds the live-output stream.
	agent    *agentrelay.Relayer
	profiles usecase.ProfileResolver        // NEW
	projects usecase.ProjectContextResolver // NEW
	// records stores contract-run outcomes; nil disables ExecuteWithContract (see WithExecutionRecords).
	records usecase.TaskExecutionRecordRepository
	// outbox/streamer back TASK-AG-FLOWTASK-003's throttled mid-run
	// republish — see publishThrottledOutput's doc comment. Both are
	// best-effort: neither ever changes Execute's own success/failure
	// outcome, only whether task.activity's agent_output_partial frames
	// show up for this run.
	outbox   usecase.OutboxWriter
	streamer usecase.AgentExecOutputStreamer
	// throttleInterval defaults to execOutputThrottleInterval in
	// NewSimpleExecutor — a struct field (not just the const directly) so
	// tests can shrink it instead of sleeping multiple real seconds per
	// assertion; same same-package-direct-field-set convention this repo's
	// other tests use (e.g. SystemClock's fake in execute_task_test.go).
	throttleInterval time.Duration
}

func NewSimpleExecutor(tasks usecase.TaskRepository, edges usecase.EdgeRepository, resolver usecase.ProjectExecutionResolver, relay infrafleetv1.InfraFleetServiceClient, profiles usecase.ProfileResolver, projects usecase.ProjectContextResolver, outbox usecase.OutboxWriter, streamer usecase.AgentExecOutputStreamer) *SimpleExecutor {
	return &SimpleExecutor{
		tasks: tasks, edges: edges, resolver: resolver, profiles: profiles, projects: projects,
		// The run is bounded by the agent's own timeout and the caller's context, as before; no RPC deadline of ours.
		agent:  agentrelay.New(relay, agentrelay.WithoutCallDeadline()),
		outbox: outbox, streamer: streamer, throttleInterval: execOutputThrottleInterval,
	}
}

// execOutputThrottleInterval is SOL-AG-FLOWTASK-001 §2.3's throttle option
// (a) — a fixed 2s interval, the only viable option for this path (option
// (b), OSC-133 milestones, is not viable: `claude --print` doesn't run
// inside a PTY, so no OSC sequences are ever emitted here — see
// agent-print-mode-exec.ts and SOL-AG-FLOWTASK-001 §2.3's own note). No
// real traffic data exists yet to tune this against (TASK-AG-FLOWTASK-003
// Open Question 1) — 2s is a starting point, not a measured value.
const execOutputThrottleInterval = 2 * time.Second

// agentOutputPartialSubject is task-service's first published NATS
// subject (TASK-AG-FLOWTASK-003) — api-gateway's wscompat
// channels_task_activity.go subscribes to it and republishes onto
// task.activity:{taskId} with Engine "direct_agent", EventType
// "agent_output_partial".
const agentOutputPartialSubject = "orca.task.agent_output_partial"

// agentOutputPartialPayload is this subject's payload shape — origin_task_id
// is the field channels_task_activity.go's translateToTaskActivity filters
// every subject's payload on (see that file), so it MUST be present and
// correct on every publish, same as every other orca.orchestration.*/
// orca.workflow.* payload already does.
type agentOutputPartialPayload struct {
	OriginTaskID string `json:"origin_task_id"`
	RequestID    string `json:"request_id"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
}

// publishThrottledOutput drains chunks (from streamer.StreamExecOutput,
// keyed by the same stepId=requestID Execute already sends
// agent.execPrompt), accumulating stdout/stderr separately, and flushes an
// outbox event at most once per execOutputThrottleInterval. Each flush
// carries the FULL buffer accumulated so far (cumulative, not a delta) —
// SOL-AG-FLOWTASK-001 §2.3's own wording ("publish one event carrying the
// buffer accumulated so far"), the simplest possible contract for whatever
// eventually reads these frames (no client-side reassembly needed). Runs
// until ctx is cancelled (Execute cancels it right after its own Relay
// call returns, success or failure) or chunks closes on its own.
func (s *SimpleExecutor) publishThrottledOutput(ctx context.Context, tenantID, taskID, requestID string, chunks <-chan usecase.AgentExecOutputChunk) (received int) {
	var stdout, stderr strings.Builder
	dirty := false
	ticker := time.NewTicker(s.throttleInterval)
	defer ticker.Stop()

	flush := func() {
		if !dirty {
			return
		}
		dirty = false
		s.publishAgentOutputPartial(tenantID, taskID, requestID, stdout.String(), stderr.String())
	}
	accumulate := func(chunk usecase.AgentExecOutputChunk) {
		received++
		if chunk.Stream == "stderr" {
			stderr.WriteString(chunk.Data)
		} else {
			stdout.WriteString(chunk.Data)
		}
		dirty = true
	}

	for {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				flush() // final flush — the run ended right after a chunk, don't lose the last partial buffer
				return
			}
			accumulate(chunk)
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			// ctx.Done() firing (Execute's own Relay call just returned)
			// races the select above against chunks already sitting in its
			// buffer — Go picks randomly between two simultaneously-ready
			// cases, so without this drain a burst of output delivered
			// right as the run ends could be silently lost instead of
			// reaching the final flush. Non-blocking: stops the moment
			// chunks has nothing immediately available, it does not wait
			// for the producer goroutine (AgentExecOutputRelay's own ctx is
			// the same one that was just cancelled, so it is also winding
			// down and will close chunks shortly regardless).
			for {
				select {
				case chunk, ok := <-chunks:
					if !ok {
						flush()
						return
					}
					accumulate(chunk)
				default:
					flush()
					return
				}
			}
		}
	}
}

// publishAgentOutputPartial marshals and inserts one outbox row. Best-effort
// (see this file's SimpleExecutor doc comment and
// postgres.Repository.InsertOutboxEvent's doc comment for the shared
// posture): a lost partial-progress update must never surface as
// SimpleExecutor.Execute's own error, but it is logged (ids only) so the
// loss is visible.
func (s *SimpleExecutor) publishAgentOutputPartial(tenantID, taskID, requestID, stdout, stderr string) {
	payload, err := json.Marshal(agentOutputPartialPayload{
		OriginTaskID: taskID, RequestID: requestID, Stdout: boundPartialStream(stdout), Stderr: boundPartialStream(stderr),
	})
	if err != nil {
		slog.Warn("task: agent output frame not encoded", slog.String("task_id", taskID), slog.Any("error", err))
		return
	}
	// context.Background(): this publish happens from publishThrottledOutput's
	// goroutine, whose own ctx may already be cancelled (e.g. the final
	// ctx.Done() flush) by the time this call runs — an outbox insert
	// should not be aborted just because the streaming subscription itself
	// is winding down.
	if err := s.outbox.InsertOutboxEvent(context.Background(), uuid.NewString(), tenantID, agentOutputPartialSubject, payload); err != nil {
		slog.Warn("task: agent output frame not stored", slog.String("task_id", taskID), slog.String("request_id", requestID), slog.Any("error", err))
	}
}

func (s *SimpleExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, worktreePath, prompt string) (string, error) {
	result, err := s.runAgentPrompt(ctx, tenantID, taskID, requestID, worktreePath, prompt, nil)
	if err != nil {
		return "", err
	}
	if result.TimedOut {
		return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_TIMED_OUT", "agent.execPrompt timed out before the task finished", nil)
	}
	if result.ExitCode == nil || *result.ExitCode != 0 {
		return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_FAILED", fmt.Sprintf("agent.execPrompt exited non-zero: %s", result.Stderr), nil)
	}
	// TASK-TG-04-07: persist this run's stdout so a LATER run
	// wave's renderExecutePrompt can resolve `{{outputs.<taskId>.*}}`
	// against this task once it's a completed dependency — best-effort,
	// a write failure here must not fail an otherwise-successful run.
	_ = s.tasks.UpdateLastExecutionOutput(ctx, tenantID, taskID, result.Stdout)
	return fmt.Sprintf("task-exec:%s:%s", taskID, requestID), nil
}

// runAgentPrompt prepares and relays one agent.execPrompt call and returns the raw result;
// Execute and ExecuteWithContract differ only in what they do with it. opts is nil for a plain run.
func (s *SimpleExecutor) runAgentPrompt(ctx context.Context, tenantID, taskID, requestID, worktreePath, prompt string, opts *contractRunOptions) (agentrelay.ExecResult, error) {
	task, err := s.tasks.Get(ctx, tenantID, taskID)
	if err != nil {
		return agentrelay.ExecResult{}, fmt.Errorf("simple_executor: load task: %w", err)
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

	// The resolver's own worktreePath return (2nd value) is deliberately
	// discarded (`_`) — BUG-028: ResolveConnection only ever knows about
	// the project's REPO, never a specific worktree, so its path guess is
	// wrong for any task reusing an existing worktree. worktreePath is now
	// an explicit parameter, resolved correctly by ExecuteTask's own
	// WorktreeProvisioner.EnsureWorktree before this call. connectionID/
	// devServerID/connected are still exactly what this call is for —
	// unchanged from BUG-026's fix.
	connectionID, repoRoot, _, devServerID, connected, err := s.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil {
		return agentrelay.ExecResult{}, fmt.Errorf("simple_executor: resolve connection: %w", err)
	}
	if !connected {
		// Per git-gateway-service.md §8's precedent: a resolve failure or
		// not-connected connectionId is a real error, never a silent local
		// fallback — task-service has no local agent.execPrompt equivalent
		// of its own (unlike git-gateway-service's §2 step 3, there is no
		// "this service's own host" case for task execution).
		return agentrelay.ExecResult{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_CONNECTION", "task's project has no connected dev server", nil)
	}
	if worktreePath == "" {
		// agent.execPrompt requires worktreePath (agent-print-mode-exec.ts:62-73
		// rejects a missing one with InvalidParams) — a connected connection
		// with no repo_path recorded is a distinct, real error, not a
		// silent no-op dispatch.
		return agentrelay.ExecResult{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_WORKTREE_PATH", "task's connected dev server has no worktree path recorded", nil)
	}

	// TASK-TG-04-06's context preamble (parent/completedDeps) and
	// TASK-PRF-04-08's profile-aware env/model/init-file resolution are
	// independent enhancements to the same dispatched prompt — composed
	// here rather than picking one: the prompt always carries the task's
	// dependency context, and the env always at least carries the base
	// ORCA_TASK_ID/ORCA_PROJECT_ID pair (TASK-TG-04-06) even when no
	// profile resolves. An explicit prompt argument (docs/backlog/BACKLOG-016)
	// overrides the built default entirely when non-empty.
	effectivePrompt := prompt
	if effectivePrompt == "" {
		built, err := renderDefaultExecutePrompt(task, parent, completedDeps, opts, repoRoot)
		if err != nil {
			return agentrelay.ExecResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_PROMPT_BUILD_FAILED", "failed to build the task execution prompt", err)
		}
		effectivePrompt = built
	}
	run := agentrelay.Run{
		Prompt:       effectivePrompt,
		WorktreePath: worktreePath,
		StepID:       requestID,
		Env:          map[string]string{"ORCA_TASK_ID": task.ID, "ORCA_PROJECT_ID": task.ProjectID},
		// TrustPreset "full" (see this file's doc comment: "full" appends
		// the YOLO flag, anything else is a no-op — agent-print-mode-exec.ts:44,97-99).
		// task.execute is a one-shot, non-interactive dispatch — there is no
		// human present in this call path to answer the CLI's normal
		// per-write/per-command approval prompts. Without this, live-
		// confirmed: the agent reports success (exit 0) having made NO
		// actual file changes, e.g. "the save to specs/generated/
		// TASK-1-spec.md was blocked because it needs your permission... approve
		// the write when it comes up again" — impossible advice for a
		// headless run, so every direct_agent dispatch that needs to write
		// anything silently no-ops without this. Deliberate policy decision
		// (not a default this port should invent quietly): every
		// task.execute dispatch now runs with full, unattended write/git
		// trust, confirmed with the user given the security-posture
		// implication.
		TrustPreset: agentrelay.TrustFull,
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
					run.Model = model
				}
			}
			env := domain.BuildAgentEnv(resolved, userID, task.ProjectID, "", "")
			env["ORCA_TASK_ID"] = task.ID // preserve TASK-TG-04-06's task-id env var alongside the profile-resolved env
			if task.ProjectID != "" {
				if pctx, err := s.projects.GetProjectContext(userCtx, task.ProjectID); err == nil {
					env["ORCA_PROJECT_NAME"] = pctx.ProjectName
					run.InitFile = domain.BuildProjectContext(domain.PreambleInput{
						ProjectName: pctx.ProjectName, Description: pctx.Description, RepoURL: pctx.RepoURL,
						WorktreePath: worktreePath, DevServerHostname: pctx.DevServerHostname,
					})
				}
			}
			run.Env = env
		}
	}

	params := opts.execParams(run, repoRoot)

	// relayCtx carries tenant id on the OUTGOING metadata for both
	// infra-fleet-service calls below — infra-fleet-service's own RPCs
	// reject a call with no tenant in context (INFRA_NO_TENANT), and the
	// plain incoming ctx only carries tenant as an inbound-extracted Go
	// value, never restamped onto an outbound gRPC call. Live-confirmed:
	// every task.execute dispatch failed with TASK_EXECUTE_FAILED, and
	// infra-fleet-service's own log showed the real cause,
	// "INFRA_NO_TENANT: no tenant in request context", on the Relay RPC —
	// this call was the one gap left un-fixed after BUG-026 (whose 3
	// earlier fixes were all in project-service call sites, not this one).
	relayCtx, err := withTenantMetadata(ctx)
	if err != nil {
		return agentrelay.ExecResult{}, fmt.Errorf("simple_executor: tenant metadata: %w", err)
	}

	// TASK-AG-FLOWTASK-003: subscribe to this run's mid-run output
	// CONCURRENTLY with the unary Relay('agent.execPrompt') call below —
	// same stepId (requestID) both calls share, so infra-fleet-service's
	// execOutputSubs demux (TASK-AG-FLOWTASK-002) correlates them. streamCtx
	// is scoped to just this Execute call: cancelled (and its consumer
	// goroutine drained via wg.Wait, in that order — see the two defers
	// below) right after Relay returns, success or failure, so no
	// subscription/goroutine outlives one Execute call.
	streamCtx, cancelStream := context.WithCancel(relayCtx)
	var streamWG sync.WaitGroup
	streamWG.Add(1)
	go func() {
		defer streamWG.Done()
		var streamFailed atomic.Bool
		// Live output is best-effort, but losing it silently is how a wrong connection id went unnoticed:
		// say so, with ids only (never output content).
		target := usecase.AgentExecOutputTarget{ConnectionID: connectionID, DevServerID: devServerID, StepID: requestID}
		chunks := s.streamer.StreamExecOutput(streamCtx, target, func(err error) {
			streamFailed.Store(true)
			slog.WarnContext(streamCtx, "task: agent output stream failed; the run continues without live output",
				slog.String("task_id", taskID), slog.String("request_id", requestID),
				slog.String("dev_server_id", devServerID), slog.String("grpc_code", status.Code(err).String()), slog.Any("error", err))
		})
		if received := s.publishThrottledOutput(streamCtx, tenantID, taskID, requestID, chunks); received == 0 && !streamFailed.Load() {
			slog.InfoContext(streamCtx, "task: agent run produced no streamed output",
				slog.String("task_id", taskID), slog.String("request_id", requestID))
		}
	}()
	// Deliberately two separate defers, not one combined func — LIFO order
	// makes cancelStream() run BEFORE streamWG.Wait() (the later defer
	// statement runs first), so the goroutine above is actually asked to
	// stop before Execute blocks waiting for it to.
	defer streamWG.Wait()
	defer cancelStream()

	// devServerID is only set when ResolveConnection resolved via its
	// no-infra.connections-row fallback (resolveViaDefaultRepo): that path has no
	// real connectionID, so agentrelay sends it through RelayByDevServer.
	result, err := s.agent.ExecPrompt(relayCtx, agentrelay.Target{DevServerID: devServerID, ConnectionID: connectionID}, params)
	if err != nil {
		return agentrelay.ExecResult{}, execPromptError(err)
	}
	return result, nil
}
