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

const maxFeedbackRunes = 2000

type GenerateSolutionInput struct {
	RequestID      string
	IdempotencyKey string
	// Feedback asks for another attempt while the previous one waits for approval.
	Feedback string
	// Mode overrides the registry's mode; empty follows the registry.
	Mode domain.AnalysisMode
}

type GenerateSolutionResult struct {
	SolutionID string
	RunID      string
	// Created is false when an existing run answered the call.
	Created bool
}

// GenerateSolution starts analysis in the background and returns at once (decision D3): the AI call can outlast the
// 25 second WebSocket budget, so the result is read back through ListSolutions and the solution events.
type GenerateSolution struct {
	requests   RequestRepository
	solutions  SolutionStore
	runs       AnalysisRunStore
	conns      AnalysisConnectionResolver
	auth       SolutionActorAuthorizer
	approvals  ApprovalCanceller
	transition RequestTransitioner
	tx         TxRunner
	spawner    AnalysisSpawner
	settings   AnalysisSettings
	leaseOwner string
	now        func() time.Time
	newID      func() string
	decisions  DecisionSuperseder
}

// WithDecisions retires the Decisions of the proposals a regenerate replaces, in the same transaction.
func (uc *GenerateSolution) WithDecisions(d DecisionSuperseder) *GenerateSolution {
	uc.decisions = d
	return uc
}

type GenerateSolutionDeps struct {
	Requests   RequestRepository
	Solutions  SolutionStore
	Runs       AnalysisRunStore
	Conns      AnalysisConnectionResolver
	Auth       SolutionActorAuthorizer
	Approvals  ApprovalCanceller
	Transition RequestTransitioner
	Tx         TxRunner
	Spawner    AnalysisSpawner
	Settings   AnalysisSettings
	LeaseOwner string
}

func NewGenerateSolution(d GenerateSolutionDeps) *GenerateSolution {
	return &GenerateSolution{
		requests: d.Requests, solutions: d.Solutions, runs: d.Runs, conns: d.Conns, auth: d.Auth, approvals: d.Approvals,
		transition: d.Transition, tx: d.Tx, spawner: d.Spawner, settings: d.Settings.withDefaults(), leaseOwner: d.LeaseOwner,
		now: func() time.Time { return time.Now().UTC() }, newID: uuid.NewString,
	}
}

// modeFor follows the registry: solutions use ai.complete, every other kind reads the repo through the agent.
func modeFor(kind domain.SolutionKind, override domain.AnalysisMode) (domain.AnalysisMode, error) {
	if kind == domain.SolutionKindSolution {
		if override == domain.AnalysisModeAgentReadonly {
			return "", domain.ErrSolutionModeNotAllowed()
		}
		return domain.AnalysisModeComplete, nil
	}
	if override == domain.AnalysisModeComplete {
		return domain.AnalysisModeComplete, nil
	}
	return domain.AnalysisModeAgentReadonly, nil
}

func (uc *GenerateSolution) Execute(ctx context.Context, in GenerateSolutionInput) (GenerateSolutionResult, error) {
	return uc.execute(ctx, in, false)
}

// ExecuteForSystem starts the same run on the system's behalf (a clarification was answered): there is no caller to
// authorise, so the run is attributed to the reporter. Only internal consumers call it, never an RPC.
func (uc *GenerateSolution) ExecuteForSystem(ctx context.Context, in GenerateSolutionInput) (GenerateSolutionResult, error) {
	return uc.execute(ctx, in, true)
}

// PrepareForSystem is ExecuteForSystem without the background spawn: the caller runs the returned func after its
// own transaction commits, because the worker reads the run row straight away.
func (uc *GenerateSolution) PrepareForSystem(ctx context.Context, in GenerateSolutionInput) (GenerateSolutionResult, func(), error) {
	res, run, err := uc.start(ctx, in, true)
	if err != nil || !res.Created {
		return res, nil, err
	}
	return res, func() { uc.spawner.Spawn(run) }, nil
}

func (uc *GenerateSolution) execute(ctx context.Context, in GenerateSolutionInput, system bool) (GenerateSolutionResult, error) {
	res, run, err := uc.start(ctx, in, system)
	if err == nil && res.Created {
		uc.spawner.Spawn(run)
	}
	return res, err
}

func (uc *GenerateSolution) start(ctx context.Context, in GenerateSolutionInput, system bool) (GenerateSolutionResult, domain.AnalysisRun, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return GenerateSolutionResult{}, domain.AnalysisRun{}, domain.ErrRequestTenantRequired()
	}
	actor, _ := tenant.UserID(ctx)
	if actor == "" && !system {
		return GenerateSolutionResult{}, domain.AnalysisRun{}, domain.ErrRequestReporterRequired()
	}
	if utf8.RuneCountInString(in.Feedback) > maxFeedbackRunes {
		return GenerateSolutionResult{}, domain.AnalysisRun{}, domain.ErrSolutionFeedbackTooLong()
	}

	req, kind, mode, err := uc.prepare(ctx, in, system)
	if err != nil {
		return GenerateSolutionResult{}, domain.AnalysisRun{}, err
	}
	// Resolve the connection before any write: no dev server means no run row at all.
	conn, err := uc.conns.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		if errors.Is(err, ErrNoDevServer) {
			if mode == domain.AnalysisModeAgentReadonly {
				return GenerateSolutionResult{}, domain.AnalysisRun{}, domain.ErrAnalysisNoConnection()
			}
			return GenerateSolutionResult{}, domain.AnalysisRun{}, domain.ErrSolutionNoConnection()
		}
		return GenerateSolutionResult{}, domain.AnalysisRun{}, err
	}
	if mode == domain.AnalysisModeAgentReadonly && conn.RepoPath == "" {
		return GenerateSolutionResult{}, domain.AnalysisRun{}, domain.ErrAnalysisNoRepoPath()
	}

	if mode == domain.AnalysisModeAgentReadonly {
		if err := uc.runs.EnsureProjectGate(ctx, req.ProjectID); err != nil {
			return GenerateSolutionResult{}, domain.AnalysisRun{}, err
		}
	}
	var res StartRunResult
	err = uc.tx.InTx(ctx, func(ctx context.Context) error {
		cur, err := uc.requests.Get(ctx, req.ID)
		if err != nil {
			return err
		}
		if err := checkAnalysisState(cur, in.Feedback); err != nil {
			return err
		}
		if actor == "" {
			actor = cur.ReporterID
		}
		now := uc.now()
		owner := uc.leaseOwner
		runID := uc.newID()
		var idem *string
		if in.IdempotencyKey != "" {
			k := in.IdempotencyKey
			idem = &k
		}
		run := domain.AnalysisRun{
			ID: runID, TenantID: tenantID, RequestID: cur.ID, Kind: domain.RunKind(kind), Mode: mode, Status: domain.RunStatusRunning,
			IdempotencyKey: idem, Attempt: 1, LeaseOwner: &owner, StartedAt: now,
			SolutionID: uc.newID(), ProjectID: cur.ProjectID, ActorID: actor, Feedback: in.Feedback,
		}
		draft := domain.Solution{
			ID: run.SolutionID, TenantID: tenantID, RequestID: cur.ID, Kind: kind, Status: domain.SolutionStatusDraft,
			OptionsJSON: []byte(`{}`), GenerationRunID: run.ID, Version: 1, CreatedAt: now, UpdatedAt: now,
			InputRequestRevision: maxInt(cur.ContentRevision, 1),
		}
		res, err = uc.runs.StartRun(ctx, run, draft, StartRunOptions{LeaseTTL: uc.settings.LeaseTTL, MaxAgentRuns: uc.settings.MaxAgentRunsPerProject})
		if err != nil || !res.Created {
			return err
		}
		if cur.Status != domain.RequestStatusAwaitingAnalysisApproval {
			return nil
		}
		// Regenerate with feedback: the pending approval and the old proposal go away in the same transaction as the new run.
		if err := uc.approvals.CancelPending(ctx, cur.ID, "revision_requested"); err != nil {
			return err
		}
		if uc.decisions != nil {
			if err := uc.decisions.SupersedeDecisions(ctx, openSolutionIDs(ctx, uc.solutions, cur.ID, kind, draft.ID)); err != nil {
				return err
			}
		}
		if _, err := uc.solutions.SupersedeOpen(ctx, cur.ID, kind, draft.ID); err != nil {
			return err
		}
		from := cur.Status
		_, err = uc.transition.Execute(ctx, TransitionInput{
			RequestID: cur.ID, Trigger: domain.TriggerAnalysisRevision, ExpectedFrom: &from, ActorID: actor, ActorKind: domain.ActorKindUser,
		})
		return err
	})
	if err != nil {
		return GenerateSolutionResult{}, domain.AnalysisRun{}, err
	}
	return GenerateSolutionResult{SolutionID: res.Run.SolutionID, RunID: res.Run.ID, Created: res.Created}, res.Run, nil
}

// prepare runs the checks that need no transaction: existence, permission, type and mode.
func (uc *GenerateSolution) prepare(ctx context.Context, in GenerateSolutionInput, system bool) (domain.Request, domain.SolutionKind, domain.AnalysisMode, error) {
	req, err := uc.requests.Get(ctx, in.RequestID)
	if err != nil {
		if isNotFound(err) {
			return domain.Request{}, "", "", domain.ErrSolutionRequestNotFound(in.RequestID)
		}
		return domain.Request{}, "", "", err
	}
	if !system {
		if err := uc.auth.AuthorizeGenerate(ctx, req); err != nil {
			return domain.Request{}, "", "", err
		}
	}
	if err := checkAnalysisState(req, in.Feedback); err != nil {
		return domain.Request{}, "", "", err
	}
	flow, ferr := domain.FlowFor(req.Type)
	if ferr != nil {
		return domain.Request{}, "", "", domain.ErrSolutionKindNotAllowed(req.Type)
	}
	kind, ok := domain.KindForAnalysis(flow.AnalysisKind)
	if !ok {
		return domain.Request{}, "", "", domain.ErrSolutionKindNotAllowed(req.Type)
	}
	mode, err := modeFor(kind, in.Mode)
	if err != nil {
		return domain.Request{}, "", "", err
	}
	return req, kind, mode, nil
}

// checkAnalysisState allows analyzing, or awaiting_analysis_approval with feedback (the regenerate path).
func checkAnalysisState(r domain.Request, feedback string) error {
	switch {
	case r.Status == domain.RequestStatusAnalyzing:
		return nil
	case r.Status == domain.RequestStatusAwaitingAnalysisApproval && feedback != "":
		return nil
	}
	return domain.ErrSolutionWrongState(r.Status)
}
