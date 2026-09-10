package usecase

import (
	"context"

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
}

func NewExecuteTask(repo TaskRepository, edges EdgeRepository, simple SimpleExecutor, complex ComplexExecutor, workflow WorkflowExecutor, resolvePermission *ResolvePermission, worktrees WorktreeProvisioner, resolver ProjectExecutionResolver, clock Clock, links ExecutionLinkRepository) *ExecuteTask {
	return &ExecuteTask{
		repo: repo, edges: edges, simple: simple, complex: complex, workflow: workflow,
		resolvePermission: resolvePermission, worktrees: worktrees, resolver: resolver, clock: clock,
		links: links,
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
	previousStatus := task.Status

	engine, err := uc.selectEngine(ctx, tenantID, task) // computed BEFORE any status write, same as the old isComplex was
	if err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_EDGE_LOOKUP_FAILED", "failed to determine execution engine", err)
	}

	// Pre-check 2: dev-server-online — resolved once here, BEFORE the
	// in_progress write, so a disconnected project fails before any status
	// mutation at all.
	_, resolvedPath, _, connected, err := uc.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil || !connected {
		return ExecuteResult{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_CONNECTION", "task's project has no connected dev server", err)
	}

	// Worktree reuse-or-create (SOL-TG-04's "IF task.worktreeId exists: use
	// existing worktree ELSE: create one").
	worktreeID, worktreePath, err := uc.worktrees.EnsureWorktree(ctx, tenantID, task)
	if err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_WORKTREE_FAILED", "failed to provision worktree", err)
	}
	if worktreePath == "" {
		worktreePath = resolvedPath // reuse branch: EnsureWorktree returns "" for path on reuse — see WorktreeProvisioner's doc comment
	}
	_ = worktreePath // resolved for parity with SOL-TG-04's design; SimpleExecutor/ComplexExecutor resolve their own worktree path today (TASK-TG-04-06 threads this through as a context preamble)
	if worktreeID != task.WorktreeID {
		if err := uc.repo.UpdateWorktreeID(ctx, tenantID, in.TaskID, worktreeID); err != nil {
			return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_WORKTREE_PERSIST_FAILED", "failed to persist worktree id", err)
		}
	}

	if err := uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, domain.StatusInProgress); err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_STATUS_UPDATE_FAILED", "failed to mark task in_progress", err)
	}
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
		_ = uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, previousStatus)
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
		_ = uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, previousStatus)
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_ACTIVE_LINK_PERSIST_FAILED", "failed to record active execution link", err)
	}

	// in.Prompt (docs/backlog/BACKLOG-016) overrides SimpleExecutor's own
	// default prompt when non-empty; worktreeID threads the reuse-or-create
	// result above into the complex path, same as before selectEngine
	// generalized this switch (TASK-TG-04-04).
	var ref string
	switch engine {
	case domain.EngineOrchestration:
		ref, err = uc.complex.Execute(ctx, tenantID, in.TaskID, in.RequestID, worktreeID)
	case domain.EngineWorkflow:
		ref, err = uc.workflow.Execute(ctx, tenantID, in.TaskID, in.RequestID, task.WorkflowTemplateID)
	default: // domain.EngineDirectAgent
		ref, err = uc.simple.Execute(ctx, tenantID, in.TaskID, in.RequestID, in.Prompt)
	}

	if err != nil {
		// The fix: revert the in_progress write instead of leaving the task
		// stuck — a dispatch failure must never leave permanently-false
		// "running" state, since there is no other RPC to clear it.
		_ = uc.links.Complete(ctx, tenantID, link.ID, "failed") // best-effort, same posture as this codebase's other non-critical bookkeeping writes
		_ = uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, previousStatus)
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_FAILED", "execution dispatch failed", err)
	}
	if ref != "" {
		_ = uc.links.SetExternalRef(ctx, tenantID, link.ID, ref) // best-effort: a failed backfill doesn't invalidate a dispatch that already succeeded
	}

	if engine != domain.EngineDirectAgent {
		// No further status write here — StatusReview/Done arrives later via
		// ReportTaskExecutionResult (TASK-TG-04-05). The link's "completed"
		// mark is only task-service's own initial bookkeeping for the async
		// engines — see ExecutionLinkRepository.Complete's doc comment.
		_ = uc.links.Complete(ctx, tenantID, link.ID, "completed")
		return ExecuteResult{ExecutionRef: ref, Async: true}, nil
	}

	// Simple path: SimpleExecutor.Execute blocks until the CLI process
	// exits — the completion transition happens INLINE, same call, no
	// separate completion RPC needed (see this usecase's doc comment).
	actualHours := uc.clock.Now().Sub(dispatchStart).Hours()
	if err := uc.repo.CompleteExecution(ctx, tenantID, in.TaskID, domain.StatusReview, actualHours); err != nil {
		_ = uc.links.Complete(ctx, tenantID, link.ID, "failed")
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_COMPLETION_WRITE_FAILED", "failed to persist execution completion", err)
	}
	_ = uc.links.Complete(ctx, tenantID, link.ID, "completed")
	return ExecuteResult{ExecutionRef: ref, Async: false}, nil
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
	deps, err := uc.edges.ListFrom(ctx, tenantID, task.ID, domain.EdgeKindDependsOn)
	if err != nil {
		return "", err
	}
	if len(deps) > 0 {
		return domain.EngineOrchestration, nil
	}
	return domain.EngineDirectAgent, nil
}
