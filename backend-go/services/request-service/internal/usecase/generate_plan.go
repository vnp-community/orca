package usecase

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type GeneratePlanInput struct {
	RequestID string
	// Feedback asks for another attempt while the previous Plan waits for approval.
	Feedback string
	// IdempotencyKey makes a repeated start return the run it created; only the automatic path sets it.
	IdempotencyKey string
}

type GeneratePlanResult struct {
	RunID string
	// Created is false when a run already in progress answered the call.
	Created bool
	// Status and ErrorCode describe the returned run, so a caller that got a run back by key sees it already failed.
	Status    domain.RunStatus
	ErrorCode string
}

// GeneratePlan is the PROPOSE step: it starts the AI run in the background and returns its id at once, because the
// call can outlast the gateway's WebSocket budget. It writes nothing to task-service (decision D3).
type GeneratePlan struct {
	requests   RequestRepository
	solutions  SolutionStore
	runs       AnalysisRunStore
	conns      AnalysisConnectionResolver
	auth       PlanActorAuthorizer
	approvals  ApprovalCanceller
	transition RequestTransitioner
	tx         TxRunner
	spawner    AnalysisSpawner
	settings   AnalysisSettings
	leaseOwner string
	pin        EnginePinner
	registry   EngineRegistry
	// revisionJobs, when set, gets a job for every change request so the worker commits the revised Plan.
	revisionJobs PlanAutoJobRepository
	now          func() time.Time
	newID        func() string
}

type GeneratePlanDeps struct {
	Requests   RequestRepository
	Solutions  SolutionStore
	Runs       AnalysisRunStore
	Conns      AnalysisConnectionResolver
	Auth       PlanActorAuthorizer
	Approvals  ApprovalCanceller
	Transition RequestTransitioner
	Tx         TxRunner
	Spawner    AnalysisSpawner
	Settings   AnalysisSettings
	LeaseOwner string
	// Pin decides the engine of the Request; nil means every Request uses the native engine.
	Pin EnginePinner
	// Registry, when set, is asked whether the pinned engine exists on this server before a run is started.
	Registry EngineRegistry
}

func NewGeneratePlan(d GeneratePlanDeps) *GeneratePlan {
	return &GeneratePlan{requests: d.Requests, solutions: d.Solutions, runs: d.Runs, conns: d.Conns, auth: d.Auth, approvals: d.Approvals,
		transition: d.Transition, tx: d.Tx, spawner: d.Spawner, settings: d.Settings.withDefaults(), leaseOwner: d.LeaseOwner, pin: d.Pin, registry: d.Registry,
		now: func() time.Time { return time.Now().UTC() }, newID: uuid.NewString}
}

// WithRevisionWorker makes every change request on an existing Plan queue a job that the plan-auto worker commits, and
// swaps in the authorizer that lets the gate's approvers ask for changes: only a server-side commit can finish a Plan
// whose requester is not allowed to commit it. Without it only the reporter and admins revise, and they commit by hand.
func (uc *GeneratePlan) WithRevisionWorker(jobs PlanAutoJobRepository, auth PlanActorAuthorizer) *GeneratePlan {
	if jobs == nil {
		return uc
	}
	uc.revisionJobs, uc.auth = jobs, auth
	return uc
}

// CommitsRevisions reports whether the worker that commits a revised Plan is attached.
func (uc *GeneratePlan) CommitsRevisions() bool { return uc.revisionJobs != nil }

func (uc *GeneratePlan) Execute(ctx context.Context, in GeneratePlanInput) (GeneratePlanResult, error) {
	return uc.run(ctx, in, false)
}

// ExecuteForSystem starts the same run when the Request reaches planning (CR-REQ-044): nobody is calling, so the run
// is attributed to the reporter, whose dev server it needs. Only internal workers call it, never an RPC.
func (uc *GeneratePlan) ExecuteForSystem(ctx context.Context, in GeneratePlanInput) (GeneratePlanResult, error) {
	return uc.run(ctx, in, true)
}

func (uc *GeneratePlan) run(ctx context.Context, in GeneratePlanInput, system bool) (GeneratePlanResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return GeneratePlanResult{}, domain.ErrRequestTenantRequired()
	}
	actor, _ := tenant.UserID(ctx)
	if actor == "" && !system {
		return GeneratePlanResult{}, domain.ErrRequestReporterRequired()
	}
	if utf8.RuneCountInString(in.Feedback) > maxFeedbackRunes {
		return GeneratePlanResult{}, domain.ErrPlanFeedbackTooLong()
	}
	if utf8.RuneCountInString(in.IdempotencyKey) > domain.PlanAutoKeyMaxLen {
		return GeneratePlanResult{}, domain.ErrPlanIdempotencyKeyInvalid()
	}
	req, err := uc.requests.Get(ctx, in.RequestID)
	if err != nil {
		return GeneratePlanResult{}, err
	}
	if system {
		// project-service rejects tenant-only calls, so the dev-server lookup below runs as the reporter.
		if actor == "" {
			actor = req.ReporterID
		}
		if actor == "" {
			return GeneratePlanResult{}, domain.ErrRequestReporterRequired()
		}
		ctx = tenant.WithUserID(ctx, actor)
	} else if err := uc.authorize(ctx, req, in.Feedback); err != nil {
		return GeneratePlanResult{}, err
	}
	if err := checkPlanRevisable(req, in.Feedback, system); err != nil {
		return GeneratePlanResult{}, err
	}
	if err := uc.checkPlannable(ctx, req, in.Feedback); err != nil {
		return GeneratePlanResult{}, err
	}
	// The dev server is bound to the reporter, so an approver or admin who revises the Plan resolves it as the reporter;
	// the run stays attributed to the caller.
	connCtx := ctx
	if req.ReporterID != "" && actor != req.ReporterID {
		connCtx = tenant.WithUserID(ctx, req.ReporterID)
	}
	// Resolve the connection before any write: no dev server means no run row at all.
	conn, err := uc.conns.ResolveForProject(connCtx, req.ProjectID)
	if err != nil {
		if errors.Is(err, ErrNoDevServer) {
			return GeneratePlanResult{}, domain.ErrPlanAIUnavailable()
		}
		return GeneratePlanResult{}, err
	}
	mode, err := uc.modeFor(ctx, req.ID, req.ProjectID, conn)
	if err != nil {
		return GeneratePlanResult{}, err
	}

	var res StartRunResult
	err = uc.tx.InTx(ctx, func(ctx context.Context) error {
		cur, err := uc.requests.Get(ctx, req.ID)
		if err != nil {
			return err
		}
		if err := checkPlanState(cur, in.Feedback); err != nil {
			return err
		}
		feedback := in.Feedback
		if feedback == "" && cur.Status == domain.RequestStatusPlanning && !system {
			if feedback, err = inheritedPlanFeedback(ctx, uc.runs, cur.ID); err != nil {
				return err
			}
		}
		owner := uc.leaseOwner
		run := domain.AnalysisRun{
			ID: uc.newID(), TenantID: tenantID, RequestID: cur.ID, Kind: domain.RunKindPlan, Mode: mode,
			Status: domain.RunStatusRunning, Attempt: 1, LeaseOwner: &owner, StartedAt: uc.now(), ProjectID: cur.ProjectID, ActorID: actor, Feedback: feedback,
		}
		if in.IdempotencyKey != "" {
			key := in.IdempotencyKey
			run.IdempotencyKey = &key
		}
		opts := StartRunOptions{LeaseTTL: uc.settings.LeaseTTL}
		if mode.IsAgent() {
			opts.MaxAgentRuns = uc.settings.MaxAgentRunsPerProject
		}
		res, err = uc.runs.StartRun(ctx, run, domain.Solution{}, opts)
		if err != nil {
			return err
		}
		if !res.Created {
			return refuseSecondPlanRevision(in, res.Run)
		}
		if err := uc.queueRevisionCommit(ctx, cur, run, actor); err != nil {
			return err
		}
		if cur.Status != domain.RequestStatusAwaitingPlanApproval {
			return nil
		}
		// Regenerate with feedback: the pending approval goes away in the same transaction as the new run.
		if err := uc.approvals.CancelPending(ctx, cur.ID, "revision_requested"); err != nil {
			return err
		}
		from := cur.Status
		_, err = uc.transition.Execute(ctx, TransitionInput{
			RequestID: cur.ID, Trigger: domain.TriggerPlanRevision, ExpectedFrom: &from, ActorID: actor, ActorKind: domain.ActorKindUser,
		})
		return err
	})
	if err != nil {
		return GeneratePlanResult{}, err
	}
	if res.Created {
		uc.spawner.Spawn(res.Run)
	}
	return GeneratePlanResult{RunID: res.Run.ID, Created: res.Created, Status: res.Run.Status, ErrorCode: derefStr(res.Run.ErrorCode)}, nil
}

// queueRevisionCommit stores the job that commits the revised Plan, in the transaction that starts the run: a run nobody
// would commit must not exist. The first Plan has no job here; the event of the approved Analysis queues its own.
func (uc *GeneratePlan) queueRevisionCommit(ctx context.Context, req domain.Request, run domain.AnalysisRun, actor string) error {
	if uc.revisionJobs == nil || req.PlanTaskID == "" || run.Feedback == "" {
		return nil
	}
	inserted, err := uc.revisionJobs.Insert(ctx, domain.PlanAutoJob{
		TenantID: run.TenantID, RequestID: req.ID, Anchor: domain.RevisionAnchor(run.ID), Origin: domain.PlanAutoOriginRevision,
		ProjectID: req.ProjectID, State: domain.PlanAutoRunning, Attempt: 1, MaxAttempts: 1, RunID: run.ID, RequestedBy: actor,
	})
	if err != nil {
		return err
	}
	if !inserted {
		// A job of an earlier attempt is still active: the worker has not yet seen that run end.
		return domain.ErrPlanRevisionInProgress()
	}
	return nil
}

// checkRevisable refuses a change request on a type whose single fix task comes from its diagnosis (a hotfix).
func checkPlanRevisable(req domain.Request, feedback string, system bool) error {
	if system || !isPlanRevision(req, feedback) {
		return nil
	}
	flow, err := domain.FlowFor(req.Type)
	if err == nil && flow.PlanKind == domain.PlanSingleTask {
		return domain.ErrPlanRevisionNotApplicable()
	}
	return nil
}

// modeFor picks how the run executes: the pinned engine decides, and only the openspec engine needs an agent in the repo.
// Without a pinner every Request keeps the original ai.complete path.
func (uc *GeneratePlan) modeFor(ctx context.Context, requestID, projectID string, conn AnalysisConnection) (domain.AnalysisMode, error) {
	if uc.pin == nil {
		return domain.AnalysisModeComplete, nil
	}
	engine, err := uc.pin.Execute(ctx, requestID)
	if err != nil {
		return "", err
	}
	if engine != domain.EngineOpenSpec {
		return domain.AnalysisModeComplete, nil
	}
	if uc.registry != nil {
		if _, err := uc.registry.For(engine); err != nil {
			return "", domain.ErrEngineUnavailable
		}
	}
	if conn.RepoPath == "" {
		return "", domain.ErrAnalysisNoRepoPath()
	}
	// The gate row serialises the per-project cap on agent runs; it must exist before StartRun counts.
	if err := uc.runs.EnsureProjectGate(ctx, projectID); err != nil {
		return "", err
	}
	return domain.AnalysisModeAgentProposal, nil
}

// checkPlannable runs the checks that need no transaction: the type has a Plan, and an approved decision exists
// where the type requires one.
func (uc *GeneratePlan) checkPlannable(ctx context.Context, req domain.Request, feedback string) error {
	if err := checkPlanState(req, feedback); err != nil {
		return err
	}
	flow, err := domain.FlowFor(req.Type)
	if err != nil {
		return domain.ErrPlanNotApplicable(req.Type)
	}
	if domain.PlanShapeFor(flow, req.Size) == domain.ShapeNone {
		return domain.ErrPlanNotApplicable(req.Type)
	}
	if flow.AnalysisKind != domain.AnalysisNone {
		if _, ok := latestApprovedAnalysis(ctx, uc.solutions, req.ID); !ok {
			return domain.ErrPlanSolutionNotApproved(req.ID)
		}
	}
	return nil
}

// checkPlanState allows planning, or awaiting_plan_approval with feedback (the regenerate path).
func checkPlanState(r domain.Request, feedback string) error {
	switch {
	case r.Status == domain.RequestStatusPlanning:
		return nil
	case r.Status == domain.RequestStatusAwaitingPlanApproval && feedback != "":
		return nil
	}
	return domain.ErrPlanWrongState(r.Status)
}
