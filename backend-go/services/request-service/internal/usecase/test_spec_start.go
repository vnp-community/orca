package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tddagent"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// StartTestSpecs is the lane that generates test specs while a Request executes. It runs next to StartExecution and shares
// nothing with it: its own consumer, its own table, its own lease; it never calls the execution loop, never moves the
// Request to another status and never holds the execution lock (docs/integrate/TDD-REQ-TESTSPEC-001, ORC-TS-ADR-11).
type StartTestSpecs struct {
	Requests RequestReader
	Runs     TestSpecRunRepository
	Agent    TestSpecAgent
	Resolver AnalysisConnectionResolver
	Context  *TestSpecContextBuilder
	Actors   ExecutionActors
	Settings TestSpecSettings
	// Now and NewID are overridable for tests.
	Now   func() time.Time
	NewID func() string
	Log   *slog.Logger
}

func (uc *StartTestSpecs) now() time.Time {
	if uc.Now != nil {
		return uc.Now().UTC()
	}
	return time.Now().UTC()
}

func (uc *StartTestSpecs) newID() string {
	if uc.NewID != nil {
		return uc.NewID()
	}
	return uuid.NewString()
}

func (uc *StartTestSpecs) log() *slog.Logger {
	if uc.Log != nil {
		return uc.Log
	}
	return slog.Default()
}

// Execute starts the lane for a Request that is executing. It is idempotent: a Request that already has a current attempt is
// left alone, so a repeated event or a reconcile pass changes nothing. started is true when this call created the attempt.
func (uc *StartTestSpecs) Execute(ctx context.Context, requestID string) (started bool, err error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return false, domain.ErrRequestTenantRequired()
	}
	req, err := uc.Requests.Get(ctx, requestID)
	if err != nil {
		return false, err
	}
	if req.Status != domain.RequestStatusExecuting || !uc.Settings.Enabled(ctx, req.ProjectID) {
		return false, nil
	}
	if _, found, err := uc.Runs.Current(ctx, req.ID); err != nil || found {
		return false, err
	}
	run := domain.TestSpecRun{
		ID: uc.newID(), TenantID: tenantID, RequestID: req.ID, Attempt: 1, State: domain.TestSpecPending,
		StartedAt: uc.now(), UpdatedAt: uc.now(),
	}
	created, err := uc.Runs.Create(ctx, run)
	if err != nil || !created {
		return false, err
	}
	// Errors from here on are the lane's own: they land on the row, never on the Request, and never on the caller (a consumer
	// that redelivered would only start the same attempt again).
	uc.drive(ctx, req, run)
	return true, nil
}

// Restart is what the Re-run button does: a new attempt replaces the current one. The run of the previous attempt is
// cancelled on the dev server when it was still going.
func (uc *StartTestSpecs) Restart(ctx context.Context, requestID string) (domain.TestSpecRun, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.TestSpecRun{}, domain.ErrRequestTenantRequired()
	}
	req, err := uc.Requests.Get(ctx, requestID)
	if err != nil {
		return domain.TestSpecRun{}, err
	}
	if req.Status != domain.RequestStatusExecuting && req.Status != domain.RequestStatusCompleted {
		return domain.TestSpecRun{}, domain.ErrTestSpecWrongStatus(req.Status)
	}
	if !uc.Settings.Enabled(ctx, req.ProjectID) {
		return domain.TestSpecRun{}, domain.ErrTestSpecNotConfigured()
	}
	attempt := 1
	prev, found, err := uc.Runs.Current(ctx, req.ID)
	if err != nil {
		return domain.TestSpecRun{}, err
	}
	if found {
		attempt = prev.Attempt + 1
		if prev.State.Active() && prev.TDDRunID != "" {
			uc.cancelBestEffort(ctx, req, prev, "re-run")
		}
	}
	run := domain.TestSpecRun{
		ID: uc.newID(), TenantID: tenantID, RequestID: req.ID, Attempt: attempt, State: domain.TestSpecPending,
		StartedAt: uc.now(), UpdatedAt: uc.now(),
	}
	if err := uc.Runs.Replace(ctx, run); err != nil {
		return domain.TestSpecRun{}, err
	}
	uc.drive(ctx, req, run)
	cur, _, err := uc.Runs.Current(ctx, req.ID)
	return cur, err
}

func (uc *StartTestSpecs) cancelBestEffort(ctx context.Context, req domain.Request, run domain.TestSpecRun, why string) {
	conn, err := uc.Resolver.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		return
	}
	if err := uc.Agent.Cancel(ctx, conn, run.TDDRunID, why); err != nil {
		uc.log().Warn("test specs: cancel on the dev server failed", slog.String("request_id", req.ID), slog.String("tdd_run_id", run.TDDRunID), slog.Any("error", err))
	}
}

// drive walks one attempt from pending to running (or to the state that explains why not) and writes every step to the row.
// It also resumes an attempt a crash left in pending or starting.
func (uc *StartTestSpecs) drive(ctx context.Context, req domain.Request, run domain.TestSpecRun) {
	finish := func(status domain.TestSpecStatus, code, msg string) {
		run.State, run.ReasonCode, run.ReasonMessage = status, code, truncateRunes(msg, 500)
		if status.Finished() {
			t := uc.now()
			run.FinishedAt = &t
		}
		uc.save(ctx, &run)
	}
	// A consumer or the reconcile loop has no signed-in user, and project-service refuses the dev server lookup without one:
	// every call below runs as the person who may run this Request's execution.
	actor := run.StartedBy
	if actor == "" {
		var aerr error
		actor, aerr = uc.Actors.ActorFor(ctx, req, "")
		if aerr != nil || actor == "" {
			msg := "nobody is allowed to run the Request's execution"
			if aerr != nil {
				msg = aerr.Error()
			}
			finish(domain.TestSpecBlocked, domain.TestSpecActorNotAllowed, msg)
			return
		}
	}
	ctx = tenant.WithUserID(ctx, actor)
	conn, err := uc.Resolver.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		if errors.Is(err, ErrNoDevServer) {
			finish(domain.TestSpecSkipped, domain.TestSpecNoDevServer, "the project has no dev server that can run it")
			return
		}
		uc.retryLater(ctx, &run, "resolve the dev server: "+err.Error())
		return
	}
	if conn.RepoPath == "" {
		finish(domain.TestSpecSkipped, domain.TestSpecNoDevServer, "the project has no repository path on its dev server")
		return
	}
	cfg, err := uc.Agent.Config(ctx, conn)
	if err != nil {
		uc.agentFailure(ctx, &run, err, finish)
		return
	}
	if !cfg.Enabled {
		finish(domain.TestSpecSkipped, domain.TestSpecDisabledOnDev, "tdd-agent has [testspec] enabled = false on the dev server")
		return
	}
	if cfg.ContextMode != "solution" && cfg.ContextMode != "plan" {
		finish(domain.TestSpecSkipped, domain.TestSpecModeNone, "tdd-agent has [testspec] context_mode = "+quote(cfg.ContextMode)+" on the dev server")
		return
	}
	built, err := uc.Context.Build(ctx, req, cfg.ContextMode)
	if err != nil {
		uc.retryLater(ctx, &run, "build the document: "+err.Error())
		return
	}
	if built.Skip != "" {
		finish(domain.TestSpecSkipped, built.Skip, built.SkipText)
		return
	}
	run.StartedBy, run.ContextMode, run.ContextRef = actor, built.Kind, built.Ref
	run.State = domain.TestSpecStarting
	uc.save(ctx, &run)
	started, err := uc.Agent.Start(ctx, conn, TestSpecStartInput{
		Kind: built.Kind, Ref: built.Ref, Document: built.Document, Files: built.Files,
		ExecutionID: req.ID, StepID: "testspec", UserID: actor,
	})
	if err != nil {
		uc.agentFailure(ctx, &run, err, finish)
		return
	}
	run.TDDRunID, run.ContextSHA256 = started.RunID, started.SHA256
	if started.Mode != "" {
		run.ContextMode = started.Mode
	}
	uc.save(ctx, &run)
	if err := uc.Agent.Dispatch(ctx, conn, started.RunID); err != nil {
		uc.agentFailure(ctx, &run, err, finish)
		return
	}
	run.State = domain.TestSpecRunning
	run.ReasonCode, run.ReasonMessage = "", ""
	uc.save(ctx, &run)
	uc.log().Info("test specs started", slog.String("request_id", req.ID), slog.String("tdd_run_id", started.RunID), slog.String("mode", run.ContextMode))
}

// agentFailure turns an error of the dev server's tdd-agent into the state of the row. A dev server that cannot be reached is
// not a verdict: the row stays pending and the reconcile pass tries again until the deadline.
func (uc *StartTestSpecs) agentFailure(ctx context.Context, run *domain.TestSpecRun, err error, finish func(domain.TestSpecStatus, string, string)) {
	var te *tddagent.ToolError
	var be *tddagent.BridgeError
	switch {
	case errors.Is(err, tddagent.ErrTransport):
		uc.retryLater(ctx, run, err.Error())
	case errors.As(err, &te):
		status, code := testSpecReasonFromAgent(te.Code)
		finish(status, code, te.Message)
	case errors.As(err, &be):
		status, code := testSpecReasonFromAgent(be.Code)
		finish(status, code, be.Message)
	default:
		finish(domain.TestSpecFailed, domain.TestSpecUnreachable, err.Error())
	}
}

// retryLater keeps the attempt pending and records why; ReconcileTestSpecs picks it up again.
func (uc *StartTestSpecs) retryLater(ctx context.Context, run *domain.TestSpecRun, msg string) {
	run.State = domain.TestSpecPending
	run.ReasonCode, run.ReasonMessage = domain.TestSpecUnreachable, truncateRunes(msg, 500)
	uc.save(ctx, run)
}

func (uc *StartTestSpecs) save(ctx context.Context, run *domain.TestSpecRun) {
	run.UpdatedAt = uc.now()
	if err := uc.Runs.Update(ctx, *run); err != nil {
		uc.log().Warn("test specs: save failed", slog.String("run_id", run.ID), slog.String("request_id", run.RequestID), slog.Any("error", err))
	}
}
