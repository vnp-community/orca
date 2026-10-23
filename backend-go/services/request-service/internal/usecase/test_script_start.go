package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tddagent"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// StartTestScripts is the lane that generates test scripts from the TCIR of the Request's test specs, in a worktree of its own, beside
// the coding agent. It starts when execution may start: the Request is executing and its test specs completed (both orders of
// those two events reach it, ORC-TSC-ADR-02). It never calls the execution loop, never moves the Request and never holds the
// execution lock (docs/integrate/TDD-REQ-TESTSCRIPT-001, ORC-TSC-ADR-11).
type StartTestScripts struct {
	Requests RequestReader
	Runs     TestScriptRunRepository
	// SpecRuns is read only: the TCIR the scripts come from is the current completed test-spec attempt.
	SpecRuns TestSpecRunRepository
	Agent    TestScriptAgent
	Resolver AnalysisConnectionResolver
	Actors   ExecutionActors
	Settings TestScriptSettings
	Options  TestScriptOptions
	Now      func() time.Time
	NewID    func() string
	Log      *slog.Logger
}

func (uc *StartTestScripts) now() time.Time {
	if uc.Now != nil {
		return uc.Now().UTC()
	}
	return time.Now().UTC()
}

func (uc *StartTestScripts) newID() string {
	if uc.NewID != nil {
		return uc.NewID()
	}
	return uuid.NewString()
}

func (uc *StartTestScripts) log() *slog.Logger {
	if uc.Log != nil {
		return uc.Log
	}
	return slog.Default()
}

// Execute starts the lane for an executing Request whose test specs finished. Idempotent: a Request with a current attempt for
// the same spec version is left alone. A Request whose test specs are still going is left alone too (the test-spec lane calls
// again when it finishes). started is true when this call created the attempt.
func (uc *StartTestScripts) Execute(ctx context.Context, requestID string) (started bool, err error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return false, domain.ErrRequestTenantRequired()
	}
	req, err := uc.Requests.Get(ctx, requestID)
	if err != nil {
		return false, err
	}
	if !domain.TestScriptLaneStatus(req.Status) || !uc.Settings.Enabled(ctx, req.ProjectID) {
		return false, nil
	}
	spec, specFound, err := uc.SpecRuns.Current(ctx, req.ID)
	if err != nil {
		return false, err
	}
	if specFound && spec.State.Active() {
		return false, nil
	}
	cur, found, err := uc.Runs.Current(ctx, req.ID)
	if err != nil {
		return false, err
	}
	if found && (cur.TestSpecRunID == spec.ID || !specFound) {
		return false, nil
	}
	run := uc.newRun(tenantID, req, 1)
	if found {
		// The test specs ran again since the scripts were generated: a new attempt on a new branch, the old one stays.
		run.Attempt = cur.Attempt + 1
		if err := uc.Runs.Replace(ctx, run); err != nil {
			return false, err
		}
	} else {
		created, err := uc.Runs.Create(ctx, run)
		if err != nil || !created {
			return false, err
		}
	}
	uc.drive(ctx, req, run)
	return true, nil
}

// Restart is what the Re-run button does: a new attempt replaces the current one; a job still going is cancelled first.
func (uc *StartTestScripts) Restart(ctx context.Context, requestID string) (domain.TestScriptRun, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.TestScriptRun{}, domain.ErrRequestTenantRequired()
	}
	req, err := uc.Requests.Get(ctx, requestID)
	if err != nil {
		return domain.TestScriptRun{}, err
	}
	if !domain.TestScriptLaneStatus(req.Status) && req.Status != domain.RequestStatusCompleted {
		return domain.TestScriptRun{}, domain.ErrTestScriptWrongStatus(req.Status)
	}
	if !uc.Settings.Enabled(ctx, req.ProjectID) {
		return domain.TestScriptRun{}, domain.ErrTestScriptNotConfigured()
	}
	attempt := 1
	prev, found, err := uc.Runs.Current(ctx, req.ID)
	if err != nil {
		return domain.TestScriptRun{}, err
	}
	if found {
		attempt = prev.Attempt + 1
		if prev.State.Active() && prev.TSAJobID != "" {
			uc.cancelBestEffort(ctx, req, prev, "re-run")
		}
	}
	run := uc.newRun(tenantID, req, attempt)
	if err := uc.Runs.Replace(ctx, run); err != nil {
		return domain.TestScriptRun{}, err
	}
	uc.drive(ctx, req, run)
	cur, _, err := uc.Runs.Current(ctx, req.ID)
	return cur, err
}

func (uc *StartTestScripts) newRun(tenantID string, req domain.Request, attempt int) domain.TestScriptRun {
	return domain.TestScriptRun{
		ID: uc.newID(), TenantID: tenantID, RequestID: req.ID, Attempt: attempt, State: domain.TestScriptPending,
		BaseRef: req.BaseRef, StartedAt: uc.now(), UpdatedAt: uc.now(),
	}
}

func (uc *StartTestScripts) cancelBestEffort(ctx context.Context, req domain.Request, run domain.TestScriptRun, why string) {
	conn, err := uc.Resolver.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		return
	}
	if err := uc.Agent.Cancel(ctx, conn, run.TSAJobID, why); err != nil {
		uc.log().Warn("test scripts: cancel on the dev server failed", slog.String("request_id", req.ID), slog.String("job_id", run.TSAJobID), slog.Any("error", err))
	}
}

// drive walks one attempt from pending to running (or to the state that explains why not). It also resumes an attempt a crash
// left pending.
func (uc *StartTestScripts) drive(ctx context.Context, req domain.Request, run domain.TestScriptRun) {
	finish := func(status domain.TestScriptStatus, code, msg string) {
		run.State, run.ReasonCode, run.ReasonMessage = status, code, truncateRunes(msg, 500)
		if status.Finished() {
			t := uc.now()
			run.FinishedAt = &t
		}
		uc.save(ctx, &run)
	}
	spec, found, err := uc.SpecRuns.Current(ctx, req.ID)
	if err != nil {
		uc.retryLater(ctx, &run, "read the test specs: "+err.Error())
		return
	}
	if !found || spec.State != domain.TestSpecCompleted || spec.Feature == "" {
		msg := "the Request has no test specs"
		if found {
			msg = "the test specs of the Request are " + string(spec.State)
			if spec.ReasonCode != "" {
				msg += " (" + spec.ReasonCode + ")"
			}
		}
		run.TestSpecRunID = spec.ID
		finish(domain.TestScriptSkipped, domain.TestScriptNoTestSpecs, msg)
		return
	}
	run.TestSpecRunID, run.Feature, run.SpecVersion = spec.ID, spec.Feature, specVersion(spec.Summary)
	run.Branch = domain.TestScriptBranch(req.Number, req.ID, run.Feature, run.SpecVersion)
	// A consumer or the reconcile loop has no signed-in user: every call runs as the person who may run this Request's execution.
	actor := run.StartedBy
	if actor == "" {
		var aerr error
		actor, aerr = uc.Actors.ActorFor(ctx, req, "")
		if aerr != nil || actor == "" {
			msg := "nobody is allowed to run the Request's execution"
			if aerr != nil {
				msg = aerr.Error()
			}
			finish(domain.TestScriptBlocked, domain.TestScriptActorNotAllowed, msg)
			return
		}
	}
	run.StartedBy = actor
	ctx = tenant.WithUserID(ctx, actor)
	conn, err := uc.Resolver.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		if errors.Is(err, ErrNoDevServer) {
			finish(domain.TestScriptSkipped, domain.TestScriptNoDevServer, "the project has no dev server that can run it")
			return
		}
		uc.retryLater(ctx, &run, "resolve the dev server: "+err.Error())
		return
	}
	if conn.RepoPath == "" {
		finish(domain.TestScriptSkipped, domain.TestScriptNoDevServer, "the project has no repository path on its dev server")
		return
	}
	in := TestScriptStartInput{
		Feature: run.Feature, Branch: run.Branch, BaseRef: req.BaseRef, TestsPaths: uc.Options.TestsPaths, TestRepo: uc.Options.TestRepo,
		Generator: uc.Options.Generator, Model: uc.Options.Model, ExecutionID: req.ID, StepID: domain.TestScriptStepID, Attempt: run.Attempt,
	}
	if run.SpecVersion > 0 {
		in.SpecVersion = run.SpecVersion
	}
	started, err := uc.Agent.Start(ctx, conn, in)
	if err != nil {
		uc.agentFailure(ctx, &run, err, finish)
		return
	}
	run.TSAJobID = started.JobID
	run.State = domain.TestScriptRunning
	run.ReasonCode, run.ReasonMessage = "", ""
	uc.save(ctx, &run)
	uc.log().Info("test scripts started", slog.String("request_id", req.ID), slog.String("job_id", started.JobID), slog.String("branch", run.Branch),
		slog.Bool("reused", started.Reused))
}

// specVersion reads the version of the test specs from the test-spec lane's summary (testspec_get(feature)); 0 when unknown.
func specVersion(summary json.RawMessage) int {
	var s domain.TestSpecSummary
	if len(summary) == 0 || json.Unmarshal(summary, &s) != nil {
		return 0
	}
	return s.Version
}

func (uc *StartTestScripts) agentFailure(ctx context.Context, run *domain.TestScriptRun, err error, finish func(domain.TestScriptStatus, string, string)) {
	var te *tddagent.ToolError
	var be *tddagent.BridgeError
	switch {
	case errors.Is(err, tddagent.ErrTransport):
		uc.retryLater(ctx, run, err.Error())
	case errors.As(err, &te):
		status, code := testScriptReasonFromAgent(te.Code)
		finish(status, code, te.Message)
	case errors.As(err, &be):
		status, code := testScriptReasonFromAgent(be.Code)
		finish(status, code, be.Message)
	default:
		finish(domain.TestScriptFailed, domain.TestScriptUnreachable, err.Error())
	}
}

// testScriptReasonFromAgent maps an error code of the bridge or of the Test Script Agent (tsa error codes) to the row.
func testScriptReasonFromAgent(code string) (domain.TestScriptStatus, string) {
	switch code {
	case "MCP_TOOL_NOT_ALLOWED", "MCP_SERVER_NOT_ALLOWED", "MCP_SERVER_NOT_INSTALLED", "TOOL_NOT_FOUND":
		return domain.TestScriptBlocked, domain.TestScriptToolMissing
	case "FRAMEWORK_UNSUPPORTED":
		return domain.TestScriptBlocked, domain.TestScriptFrameworkUnsupported
	case "GENERATOR":
		return domain.TestScriptBlocked, domain.TestScriptGenerator
	case "TCIR_SOURCE":
		return domain.TestScriptBlocked, domain.TestScriptTCIRSource
	case "SPEC_VERSION_CHANGED":
		return domain.TestScriptFailed, domain.TestScriptSpecVersionChanged
	case "WORKTREE":
		return domain.TestScriptFailed, domain.TestScriptWorktree
	case "TSA_JOB_LOST":
		return domain.TestScriptFailed, domain.TestScriptJobLost
	case "":
		return domain.TestScriptFailed, domain.TestScriptFailedCode
	}
	return domain.TestScriptFailed, code
}

// retryLater keeps the attempt pending and records why; ReconcileTestScripts picks it up again.
func (uc *StartTestScripts) retryLater(ctx context.Context, run *domain.TestScriptRun, msg string) {
	run.State = domain.TestScriptPending
	run.ReasonCode, run.ReasonMessage = domain.TestScriptUnreachable, truncateRunes(msg, 500)
	uc.save(ctx, run)
}

func (uc *StartTestScripts) save(ctx context.Context, run *domain.TestScriptRun) {
	run.UpdatedAt = uc.now()
	if err := uc.Runs.Update(ctx, *run); err != nil {
		uc.log().Warn("test scripts: save failed", slog.String("run_id", run.ID), slog.String("request_id", run.RequestID), slog.Any("error", err))
	}
}
