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
}

type GeneratePlanResult struct {
	RunID string
	// Created is false when a run already in progress answered the call.
	Created bool
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
	now        func() time.Time
	newID      func() string
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
}

func NewGeneratePlan(d GeneratePlanDeps) *GeneratePlan {
	return &GeneratePlan{requests: d.Requests, solutions: d.Solutions, runs: d.Runs, conns: d.Conns, auth: d.Auth, approvals: d.Approvals,
		transition: d.Transition, tx: d.Tx, spawner: d.Spawner, settings: d.Settings.withDefaults(), leaseOwner: d.LeaseOwner, pin: d.Pin,
		now: func() time.Time { return time.Now().UTC() }, newID: uuid.NewString}
}

func (uc *GeneratePlan) Execute(ctx context.Context, in GeneratePlanInput) (GeneratePlanResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return GeneratePlanResult{}, domain.ErrRequestTenantRequired()
	}
	actor, _ := tenant.UserID(ctx)
	if actor == "" {
		return GeneratePlanResult{}, domain.ErrRequestReporterRequired()
	}
	if utf8.RuneCountInString(in.Feedback) > maxFeedbackRunes {
		return GeneratePlanResult{}, domain.ErrPlanFeedbackTooLong()
	}
	req, err := uc.requests.Get(ctx, in.RequestID)
	if err != nil {
		return GeneratePlanResult{}, err
	}
	if err := uc.auth.AuthorizePlan(ctx, req); err != nil {
		return GeneratePlanResult{}, err
	}
	if err := uc.checkPlannable(ctx, req, in.Feedback); err != nil {
		return GeneratePlanResult{}, err
	}
	// Resolve the connection before any write: no dev server means no run row at all.
	conn, err := uc.conns.ResolveForProject(ctx, req.ProjectID)
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
		owner := uc.leaseOwner
		run := domain.AnalysisRun{
			ID: uc.newID(), TenantID: tenantID, RequestID: cur.ID, Kind: domain.RunKindPlan, Mode: mode,
			Status: domain.RunStatusRunning, Attempt: 1, LeaseOwner: &owner, StartedAt: uc.now(), ProjectID: cur.ProjectID, ActorID: actor, Feedback: in.Feedback,
		}
		opts := StartRunOptions{LeaseTTL: uc.settings.LeaseTTL}
		if mode.IsAgent() {
			opts.MaxAgentRuns = uc.settings.MaxAgentRunsPerProject
		}
		res, err = uc.runs.StartRun(ctx, run, domain.Solution{}, opts)
		if err != nil || !res.Created {
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
	return GeneratePlanResult{RunID: res.Run.ID, Created: res.Created}, nil
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
