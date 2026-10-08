package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type ExecuteTaskInput struct {
	TaskID    string
	RequestID string
	// Prompt overrides the executor's own default prompt (built from the
	// task's Title) when non-empty — see docs/backlog/BACKLOG-016 and
	// SimpleExecutor.buildExecutePrompt's doc comment for the default it
	// replaces. Not persisted onto the task.
	Prompt string
	// ResultNonce, when set, selects the contract path for a task that has a spec: the agent must
	// close with an ORCA_RESULT block carrying this nonce. Attempt 0 is derived from RequestID.
	ResultNonce string
	Attempt     int
}

// ExecuteResult replaces the bare execution-ref string Execute used to
// return — Async distinguishes the complex/workflow paths (orchestration-
// service or workflow-service; completion arrives later via
// ReportTaskExecutionResult, TASK-TG-04-05/TASK-FT-002-04) from the simple
// path (SimpleExecutor.Execute blocks until the CLI process exits, so
// completion is written INLINE, same call — TASK-TG-04-03).
type ExecuteResult struct {
	ExecutionRef string
	Async        bool
}

// ExecuteTask is task-service's execution-dispatch usecase (§3.1). The
// branching logic — selectEngine's three-way split — is this usecase's
// real, non-stubbed value: a task with an explicit workflow_template_id
// goes to Engine 3 (WorkflowExecutor/workflow-service); otherwise any
// parent_child (subtask) or depends_on edge FROM it makes it "complex" and
// hands off to Engine 2 (ComplexExecutor/orchestration-service); everything
// else is "simple" and relays directly to Engine 1 (SimpleExecutor/
// infra-fleet-service).
//
// Execute (SOL-TG-04) does the following, in order, BEFORE marking the task
// StatusInProgress: (1) resolves the caller's permission to "execute" the
// task via ResolvePermission — every other mutating RPC does this per
// task-service.md §3, ExecuteTask previously didn't despite dispatching
// real work; (2) determines the engine via selectEngine; (3) resolves the
// project's dev server connection, failing closed (TASK_EXECUTE_NO_CONNECTION)
// before any status write if it's not connected; (4) provisions
// (reuse-or-create) the task's worktree via WorktreeProvisioner. Only once
// all of that succeeds does it write StatusInProgress and dispatch.
//
// If dispatch itself then fails, Execute reverts the status write back to
// whatever it was before dispatch — TASK-TG-04-01's fix for a real bug: a
// task whose dev server is offline (or any other dispatch failure) used to
// be marked in_progress PERMANENTLY on every failed Execute attempt, with no
// RPC to ever clear it.
//
// The simple path additionally closes the "no completion callback" gap
// inline: SimpleExecutor.Execute already blocks synchronously until the Dev
// Server Agent's agent.execPrompt call returns, so by the time it returns
// here Execute already knows the outcome — CompleteExecution records
// StatusReview + actual_hours (measured via Clock) in the same call, no
// second RPC needed. The complex/workflow paths have no such synchronous
// signal (orchestration-service/workflow-service dispatch is async) — they
// return Async: true and leave status at InProgress; TASK-FT-002-04's
// generalized ReportTaskExecutionResult is the eventual completion write for
// both.
// asyncTaskRunner runs fn — real wiring (NewExecuteTask) always spawns it as
// a goroutine, so Execute can return before fn finishes (see
// dispatchDirectAgentAsync's doc comment for why this exists at all). Tests
// substitute a synchronous version (runs fn on the caller's own goroutine)
// via newExecutableExecuteTask's test helper, so assertions right after
// Execute returns observe fn's already-completed side effects
// deterministically — no sleep/poll loop needed in any test.
type asyncTaskRunner func(fn func())

type ExecuteTask struct {
	repo              TaskRepository
	edges             EdgeRepository
	simple            SimpleExecutor
	complex           ComplexExecutor
	workflow          WorkflowExecutor
	resolvePermission *ResolvePermission
	worktrees         WorktreeProvisioner
	resolver          ProjectExecutionResolver
	clock             Clock
	// links records one execution_links row per Execute call that reaches
	// dispatch, across all three engines (BE-SOL-001/CR-FLOW-TASK-001) — see
	// ExecutionLinkRepository's doc comment.
	links ExecutionLinkRepository
	// runAsync backs the direct_agent engine's async dispatch — see
	// dispatchDirectAgentAsync's doc comment.
	runAsync asyncTaskRunner

	// leases, when set, makes direct_agent runs recoverable after a crash —
	// see ExecutionLeaseRepository. nil keeps the previous behavior.
	claimer        TaskExecutionClaimer
	leases         ExecutionLeaseRepository
	leaseOwner     string
	leaseTTL       time.Duration
	leaseHeartbeat time.Duration

	// sync re-derives the parent plan/phase whenever this task's status changes.
	sync *SyncContainerStatus

	// releaser reverts a failed dispatch together with its outbox event; outbox is
	// the non-atomic fallback for failures before the active link exists.
	releaser TaskExecutionReleaser
	outbox   OutboxWriter

	// contract/specs back the spec-task path, see WithContract.
	contract ContractAgentExecutor
	specs    TaskSpecLookup
}

// WithRunEvents lets a failed dispatch revert through the link-guarded release
// (atomic with its statuschanged event). Without it, request-owned tasks revert
// silently as before.
func (uc *ExecuteTask) WithRunEvents(releaser TaskExecutionReleaser, outbox OutboxWriter) *ExecuteTask {
	uc.releaser = releaser
	uc.outbox = outbox
	return uc
}

// revertDispatch puts a claimed task back to previous after a failed dispatch.
// With an active link the revert is a CAS that also writes the event in one
// transaction; before the link exists no CAS key is available, so the event goes
// out best-effort (reconcile covers a lost one).
func (uc *ExecuteTask) revertDispatch(ctx context.Context, tenantID string, task domain.Task, linkID string, previous domain.Status, engine domain.ExecutionEngine, cause error) {
	uc.revertDispatchWithOutcome(ctx, tenantID, task, linkID, previous, engine, cause, runOutcome{})
}

// revertDispatchWithOutcome is revertDispatch plus the contract run's failure class and record id.
func (uc *ExecuteTask) revertDispatchWithOutcome(ctx context.Context, tenantID string, task domain.Task, linkID string, previous domain.Status, engine domain.ExecutionEngine, cause error, outcome runOutcome) {
	events := runEventsWithOutcome(task, domain.StatusInProgress, previous, CauseExecutionFailed, linkID, engine, cause.Error(), uc.clock.Now(), outcome)
	if linkID != "" && len(events) > 0 && uc.releaser != nil {
		released, err := uc.releaser.ReleaseExecution(ctx, tenantID, task.ID, linkID, previous, events)
		if err == nil {
			// !released: recovery or a newer dispatch already owns the task.
			if released {
				syncContainerParent(ctx, uc.sync, task.ID)
			}
			return
		}
		slog.WarnContext(ctx, "task: link-guarded revert failed, falling back to plain status write",
			slog.String("task_id", task.ID), slog.Any("error", err))
	}
	_ = uc.repo.UpdateStatus(ctx, tenantID, task.ID, previous)
	syncContainerParent(ctx, uc.sync, task.ID)
	if len(events) > 0 && uc.outbox != nil {
		ev := events[0]
		if err := uc.outbox.InsertOutboxEvent(ctx, ev.ID, tenantID, ev.Subject, ev.PayloadJSON); err != nil {
			slog.WarnContext(ctx, "task: could not record revert event", slog.String("task_id", task.ID), slog.Any("error", err))
		}
	}
}

// WithContainerSync enables plan/phase status derivation; nil keeps the old behavior.
func (uc *ExecuteTask) WithContainerSync(s *SyncContainerStatus) *ExecuteTask {
	uc.sync = s
	return uc
}

func NewExecuteTask(repo TaskRepository, edges EdgeRepository, simple SimpleExecutor, complex ComplexExecutor, workflow WorkflowExecutor, resolvePermission *ResolvePermission, worktrees WorktreeProvisioner, resolver ProjectExecutionResolver, clock Clock, links ExecutionLinkRepository) *ExecuteTask {
	return &ExecuteTask{
		repo: repo, edges: edges, simple: simple, complex: complex, workflow: workflow,
		resolvePermission: resolvePermission, worktrees: worktrees, resolver: resolver, clock: clock,
		links:    links,
		runAsync: func(fn func()) { go fn() },
	}
}

func (uc *ExecuteTask) Execute(ctx context.Context, in ExecuteTaskInput) (ExecuteResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.TaskID == "" {
		return ExecuteResult{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_EXECUTE_INVALID", "task_id is required", nil)
	}

	// Pre-check 1: permission — every mutating RPC calls ResolvePermission
	// internally first per task-service.md §3; ExecuteTask never has,
	// despite dispatching real work. Closed here, BEFORE any status write.
	userID, _ := tenant.UserID(ctx)
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: userID, Action: "execute"}); err != nil {
		return ExecuteResult{}, err // PermissionDenied, no status write happened yet
	}

	task, err := uc.repo.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}

	// Plan/phase are derived containers: one worktree and coordinator per phase would contradict
	// the v6 design, so they are never dispatched (lift this guard only with a phase coordinator).
	if domain.IsContainerType(task.Type) {
		return ExecuteResult{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_CONTAINER_NOT_EXECUTABLE", "plan and phase tasks cannot be executed; execute their tasks instead", nil)
	}

	// Pre-check 0: reject a re-dispatch while one is already running.
	//
	// Why this exists: dispatchDirectAgentAsync (this bug's own async
	// redesign) made Execute return in milliseconds instead of blocking for
	// the whole agent run — which means NOTHING any longer naturally
	// prevents a second click (or a third, or a tenth) from firing another
	// concurrent Execute call for the SAME task while the first is still
	// running in the background. Live-confirmed real consequence, not
	// theoretical: a user repeatedly clicking "Run with Agent" (because the
	// first click gave no visible feedback) fired several concurrent
	// dispatches against the SAME dev server connection, which crashed it
	// ("devserveragent: connection lost: EOF") — and worse, EACH concurrent
	// dispatch captures its OWN `previousStatus` snapshot for its
	// on-failure revert; the second dispatch's snapshot is already
	// "in_progress" (set by the first dispatch moments earlier), so its
	// failure "reverts" to in_progress — a no-op — leaving the task stuck
	// at in_progress FOREVER once every concurrent attempt has failed, with
	// no RPC able to clear it (the exact TASK-TG-04-01 failure mode this
	// usecase already fixed once, reintroduced by concurrent re-entrancy).
	//
	// This check-then-write still has a narrow TOCTOU race (two Execute
	// calls landing within microseconds of each other could both read a
	// pre-dispatch status before either writes in_progress) — closing that
	// fully would need a conditional/CAS-style UpdateStatus, a bigger
	// interface change not justified here: this guard's real job is
	// collapsing the "user clicks 10 times over several seconds because the
	// first click gave no feedback" scenario (the one actually observed
	// live) down from "guaranteed pile-up" to "not practically reachable",
	// not achieving perfect mutual exclusion.
	if task.Status == domain.StatusInProgress {
		return ExecuteResult{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_ALREADY_IN_PROGRESS", "task already has a dispatch in progress", nil)
	}
	previousStatus := task.Status
	if in.ResultNonce != "" && in.Attempt == 0 {
		in.Attempt = parseAttempt(in.RequestID)
	}

	engine, err := uc.selectEngine(ctx, tenantID, task) // computed BEFORE any status write, same as the old isComplex was
	if err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_EDGE_LOOKUP_FAILED", "failed to determine execution engine", err)
	}

	// Only the direct_agent engine can honor a prompt override. The spec ->
	// approve -> code loop (BL-TG-05) sends one per phase; on a task with
	// subtasks the coordinator would ignore it and run the implementation,
	// silently skipping the approval step. Refuse instead of doing that.
	if in.Prompt != "" && engine != domain.EngineDirectAgent {
		return ExecuteResult{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_PROMPT_UNSUPPORTED",
			"a prompt override is only supported for tasks without subtasks, dependencies or an attached workflow", nil)
	}

	// Pre-check 2: dev-server-online — resolved once here, BEFORE the
	// in_progress write, so a disconnected project fails before any status
	// mutation at all.
	_, resolvedPath, _, _, connected, err := uc.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil || !connected {
		// BUG-026 diagnostic: apperrors.ToGRPCStatus only forwards
		// Kind+Code+Message to the client, never the wrapped cause — logging
		// it explicitly here is the only way to see WHY the resolver
		// reported not-connected (a real RPC error inside the fallback vs.
		// a genuine "no reachable dev server" result) without re-deriving
		// it from scratch every time this fires.
		slog.WarnContext(ctx, "task: execute blocked, project has no connected dev server",
			slog.String("task_id", in.TaskID), slog.String("project_id", task.ProjectID), slog.Any("resolver_error", err))
		return ExecuteResult{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_CONNECTION", "task's project has no connected dev server", err)
	}

	// Worktree reuse-or-create (SOL-TG-04's "IF task.worktreeId exists: use
	// existing worktree ELSE: create one").
	worktreeID, worktreePath, err := uc.worktrees.EnsureWorktree(ctx, tenantID, task)
	if err != nil {
		// BUG-026-style diagnostic: same reasoning as the connection-check
		// log above — apperrors.ToGRPCStatus never forwards this wrapped
		// cause to the client, and the "rpc failed" log line only has the
		// client-facing message, not this real error.
		slog.WarnContext(ctx, "task: execute failed to provision worktree",
			slog.String("task_id", in.TaskID), slog.String("project_id", task.ProjectID), slog.Any("worktree_error", err))
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_WORKTREE_FAILED", "failed to provision worktree", err)
	}
	if worktreePath == "" {
		// Defensive only, not the normal path since BUG-028: EnsureWorktree
		// now resolves a real path on both the create AND reuse branches
		// (failing closed on error instead of returning ""), so this only
		// fires if EnsureWorktree somehow violates that contract — falling
		// back to the project's repo-root path here is still better than
		// crashing, but should never actually happen in practice.
		worktreePath = resolvedPath
	}
	if worktreeID != task.WorktreeID {
		if err := uc.repo.UpdateWorktreeID(ctx, tenantID, in.TaskID, worktreeID); err != nil {
			return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_WORKTREE_PERSIST_FAILED", "failed to persist worktree id", err)
		}
	}

	task.WorktreeID = worktreeID // events carry the worktree this run uses
	if uc.claimer != nil {
		// Atomic claim: only the Execute call that still sees previousStatus wins.
		claimEvents := runEvents(task, previousStatus, domain.StatusInProgress, CauseExecuteClaim, "", engine, "", uc.clock.Now())
		claimed, err := uc.claimer.ClaimForExecution(ctx, tenantID, in.TaskID, previousStatus, claimEvents)
		if err != nil {
			return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_STATUS_UPDATE_FAILED", "failed to mark task in_progress", err)
		}
		if !claimed {
			return ExecuteResult{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_ALREADY_IN_PROGRESS", "task already has a dispatch in progress", nil)
		}
	} else if err := uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, domain.StatusInProgress); err != nil {
		// No claimer means no shared transaction, so no claim event; production always wires the claimer.
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_STATUS_UPDATE_FAILED", "failed to mark task in_progress", err)
	}
	syncContainerParent(ctx, uc.sync, in.TaskID)
	dispatchStart := uc.clock.Now()

	// One execution_links row per Execute call that reaches dispatch, across
	// all three engines — CR-FLOW-TASK-001's acceptance criteria, so
	// CR-FLOW-TASK-003's Activity Feed (BE-SOL-003) has a history row even
	// for the synchronous Engine 1 path. A failed link write must not
	// silently proceed with an unaccounted-for dispatch: fail closed and
	// revert the in_progress write already made, same posture as the other
	// pre-dispatch failures above.
	link, linkErr := uc.links.CreateExecutionLink(ctx, tenantID, in.TaskID, engine, "")
	if linkErr != nil {
		uc.revertDispatch(ctx, tenantID, task, "", previousStatus, engine, linkErr)
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_LINK_CREATE_FAILED", "failed to record execution link", linkErr)
	}
	// Record this link as the task's active dispatch BEFORE calling out to
	// the engine — ReportTaskExecutionResult (TASK-FT-002-04) validates an
	// inbound callback's execution_ref/engine against this pointer, and a
	// fast-completing async engine could call back before this Execute call
	// returns. A failed write here must fail closed like the link create
	// above: without it, the eventual completion callback can never match
	// and would be silently dropped forever.
	if err := uc.repo.SetActiveExecutionLink(ctx, tenantID, in.TaskID, link.ID); err != nil {
		_ = uc.links.Complete(ctx, tenantID, link.ID, "failed")
		uc.revertDispatch(ctx, tenantID, task, "", previousStatus, engine, err)
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_ACTIVE_LINK_PERSIST_FAILED", "failed to record active execution link", err)
	}

	// Lease the run BEFORE dispatching, so a crash between here and the
	// goroutine starting is still recoverable. Best-effort on purpose: if the
	// lease cannot be written (e.g. migration 0013 not applied yet during a
	// rolling deploy), degrade to the old un-recoverable behavior instead of
	// failing every direct_agent run.
	if uc.leases != nil {
		if engine == domain.EngineDirectAgent {
			if err := uc.leases.StartLease(ctx, tenantID, link.ID, uc.leaseOwner, string(previousStatus), uc.leaseTTL); err != nil {
				slog.WarnContext(ctx, "task: could not lease execution, run will not be recoverable after a crash",
					slog.String("task_id", in.TaskID), slog.String("link_id", link.ID), slog.Any("error", err))
			}
		} else if err := uc.leases.SetPreviousStatus(ctx, tenantID, link.ID, string(previousStatus)); err != nil {
			// Engines 2/3 have no lease (they report back); this only lets a
			// failure report restore the task. Best-effort for the same reason.
			slog.WarnContext(ctx, "task: could not record previous status, a failed run will restore to open",
				slog.String("task_id", in.TaskID), slog.String("link_id", link.ID), slog.Any("error", err))
		}
	}

	// in.Prompt (docs/backlog/BACKLOG-016) overrides SimpleExecutor's own
	// default prompt when non-empty; worktreeID threads the reuse-or-create
	// result above into the complex path, same as before selectEngine
	// generalized this switch (TASK-TG-04-04).
	//
	// EngineDirectAgent dispatches via dispatchDirectAgentAsync instead of
	// inline below — see that method's doc comment for why (redesigned to
	// async, explicit user ask after a live timeout: "thiết kế lại đi. phải
	// theo async"). EngineOrchestration/EngineWorkflow are unchanged: their
	// own .Execute() calls are already fast (they just kick off a request to
	// another service and return a ref), so they keep reporting a synchronous
	// dispatch error to the caller — only SimpleExecutor.Execute blocks for
	// up to 15 minutes, so only its path needed to move off this RPC's own
	// lifetime.
	if engine == domain.EngineDirectAgent {
		return uc.dispatchDirectAgentAsync(ctx, tenantID, userID, in, task, worktreePath, link.ID, previousStatus, dispatchStart)
	}

	var ref string
	switch engine {
	case domain.EngineOrchestration:
		ref, err = uc.complex.Execute(ctx, tenantID, in.TaskID, in.RequestID, worktreeID)
	case domain.EngineWorkflow:
		ref, err = uc.workflow.Execute(ctx, tenantID, in.TaskID, in.RequestID, task.WorkflowTemplateID)
	}

	if err != nil {
		// The fix: revert the in_progress write instead of leaving the task
		// stuck — a dispatch failure must never leave permanently-false
		// "running" state, since there is no other RPC to clear it.
		_ = uc.links.Complete(ctx, tenantID, link.ID, "failed") // best-effort, same posture as this codebase's other non-critical bookkeeping writes
		uc.revertDispatch(ctx, tenantID, task, link.ID, previousStatus, engine, err)
		slog.WarnContext(ctx, "task: execute dispatch failed",
			slog.String("task_id", in.TaskID), slog.String("project_id", task.ProjectID), slog.String("engine", string(engine)), slog.Any("dispatch_error", err))
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_FAILED", "execution dispatch failed", err)
	}
	if ref != "" {
		_ = uc.links.SetExternalRef(ctx, tenantID, link.ID, ref) // best-effort: a failed backfill doesn't invalidate a dispatch that already succeeded
	}

	// No further status write here — StatusReview/Done arrives later via
	// ReportTaskExecutionResult (TASK-TG-04-05). The link's "completed"
	// mark is only task-service's own initial bookkeeping for the async
	// engines — see ExecutionLinkRepository.Complete's doc comment.
	_ = uc.links.Complete(ctx, tenantID, link.ID, "completed")
	return ExecuteResult{ExecutionRef: ref, Async: true}, nil
}

// dispatchDirectAgentAsync dispatches SimpleExecutor.Execute (and its
// completion bookkeeping) via uc.runAsync instead of inline on Execute's own
// call stack.
//
// Why: SimpleExecutor.Execute blocks synchronously on the spawned CLI
// process for up to 15 minutes (agent-print-mode-exec.ts's MAX_TIMEOUT_MS).
// Running that inline used to hold this Execute call's own client-facing
// RPC open for the same duration — live-confirmed broken: the frontend's
// own WS connection liveness watchdog (REMOTE_RUNTIME_SOCKET_LIVENESS_TIMEOUT_MS
// / HEARTBEAT_IDLE_MS, 25s) tore the connection down mid-run, canceling this
// call's ctx, which cascaded down through infra-fleet-service as "context
// canceled" and surfaced to the user as a raw DeadlineExceeded RPC error.
// Explicit user ask after hitting this live: "thiết kế lại đi. phải theo
// async" (redesign it, it must be async).
//
// This mirrors the orchestration/workflow engines' own long-standing async
// posture (dispatch, return immediately, completion arrives later) rather
// than inventing a new mechanism — the difference is where "later" runs:
// those two engines get a completion signal from a REAL later network call
// (ReportTaskExecutionResult, TASK-TG-04-05, since orchestration-service/
// workflow-service are separate processes); SimpleExecutor.Execute runs
// in-process, so an in-process goroutine plays the same role, with no
// external callback RPC needed.
//
// dispatchCtx is deliberately NOT ctx: ctx dies with this RPC (the very
// problem being fixed here). dispatchCtx rebuilds only the identity values
// SimpleExecutor.Execute and its downstream resolvers actually read from
// context (tenant id, user id) onto a fresh context.Background() — no
// deadline, no tie to the client connection, the same shape a real
// completion callback's own fresh inbound request would carry.
func (uc *ExecuteTask) dispatchDirectAgentAsync(ctx context.Context, tenantID, userID string, in ExecuteTaskInput, task domain.Task, worktreePath, linkID string, previousStatus domain.Status, dispatchStart time.Time) (ExecuteResult, error) {
	dispatchCtx := tenant.WithTenantID(context.Background(), tenantID)
	if userID != "" {
		dispatchCtx = tenant.WithUserID(dispatchCtx, userID)
	}
	uc.runAsync(func() {
		stopHeartbeat := uc.startHeartbeat(dispatchCtx, tenantID, linkID)
		defer stopHeartbeat()
		if in.ResultNonce != "" && uc.contract != nil {
			uc.runContractAgent(dispatchCtx, tenantID, in, task, worktreePath, linkID, previousStatus, dispatchStart)
			return
		}
		ref, err := uc.simple.Execute(dispatchCtx, tenantID, in.TaskID, in.RequestID, worktreePath, in.Prompt)
		if err != nil {
			// Same fix as the orchestration/workflow branch above: revert
			// the in_progress write instead of leaving the task stuck.
			_ = uc.links.Complete(dispatchCtx, tenantID, linkID, "failed")
			uc.revertDispatch(dispatchCtx, tenantID, task, linkID, previousStatus, domain.EngineDirectAgent, err)
			slog.WarnContext(dispatchCtx, "task: execute dispatch failed",
				slog.String("task_id", in.TaskID), slog.String("project_id", task.ProjectID), slog.String("engine", string(domain.EngineDirectAgent)), slog.Any("dispatch_error", err))
			return
		}
		if ref != "" {
			_ = uc.links.SetExternalRef(dispatchCtx, tenantID, linkID, ref)
		}
		actualHours := uc.clock.Now().Sub(dispatchStart).Hours()
		doneEvents := runEvents(task, domain.StatusInProgress, domain.StatusReview, CauseExecutionCompleted, linkID, domain.EngineDirectAgent, "", uc.clock.Now())
		if err := uc.repo.CompleteExecution(dispatchCtx, tenantID, in.TaskID, string(domain.StatusReview), actualHours, doneEvents); err != nil {
			_ = uc.links.Complete(dispatchCtx, tenantID, linkID, "failed")
			slog.WarnContext(dispatchCtx, "task: execute completion write failed",
				slog.String("task_id", in.TaskID), slog.Any("completion_error", err))
			return
		}
		syncContainerParent(dispatchCtx, uc.sync, in.TaskID)
		_ = uc.links.Complete(dispatchCtx, tenantID, linkID, "completed")
	})
	return ExecuteResult{Async: true}, nil
}

// selectEngine implements task-service.md §3.1's three-way dispatch branch
// (CR-FLOW-TASK-001) — same two ListFrom calls isComplex used to make, plus
// the workflow_template_id priority check.
func (uc *ExecuteTask) selectEngine(ctx context.Context, tenantID string, task domain.Task) (domain.ExecutionEngine, error) {
	// Priority: an explicitly-set workflow_template_id always wins over the
	// automatic subtree/dependency check below — CR-FLOW-TASK-001's own
	// stated rule (avoid ambiguity when a task has both a subtask and an
	// attached workflow).
	if task.WorkflowTemplateID != "" {
		return domain.EngineWorkflow, nil
	}

	children, err := uc.edges.ListFrom(ctx, tenantID, task.ID, domain.EdgeKindParentChild)
	if err != nil {
		return "", err
	}
	if len(children) > 0 {
		return domain.EngineOrchestration, nil
	}
	// A task with a spec carries a prompt override, which only Engine 1 honors; ordering between
	// spec tasks is AdvanceExecution's job, so depends_on must not push it to the coordinator.
	if uc.specs != nil && task.RequestID != "" {
		has, err := uc.specs.HasSpec(ctx, tenantID, task.ID)
		if err != nil {
			return "", err
		}
		if has {
			return domain.EngineDirectAgent, nil
		}
	}
	deps, err := uc.edges.ListFrom(ctx, tenantID, task.ID, domain.EdgeKindDependsOn)
	if err != nil {
		return "", err
	}
	if len(deps) > 0 {
		return domain.EngineOrchestration, nil
	}
	return domain.EngineDirectAgent, nil
}
