package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/tddagent"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// TestSpecSettingsDefaults are the knobs of the test-spec lane.
type TestSpecTiming struct {
	// Quiet is how long a row waits between two looks; Lease outlasts one look; Deadline ends a run that never finishes.
	Quiet    time.Duration
	Lease    time.Duration
	Deadline time.Duration
	Batch    int
}

func (t TestSpecTiming) normalized() TestSpecTiming {
	if t.Quiet <= 0 {
		t.Quiet = 20 * time.Second
	}
	if t.Lease <= 0 {
		t.Lease = 2 * time.Minute
	}
	if t.Deadline <= 0 {
		t.Deadline = 60 * time.Minute
	}
	if t.Batch <= 0 {
		t.Batch = 20
	}
	return t
}

// ReconcileTestSpecs is the loop of the test-spec lane: it starts the lane for executing Requests whose event was lost, resumes an
// attempt a crash left half way, and looks at the runs that are going on the dev server. It does not touch the execution loop.
type ReconcileTestSpecs struct {
	Runs     TestSpecRunRepository
	Requests RequestReader
	Start    *StartTestSpecs
	Agent    TestSpecAgent
	Resolver AnalysisConnectionResolver
	Timing   TestSpecTiming
	Owner    string
	Now      func() time.Time
	Log      *slog.Logger
}

func (uc *ReconcileTestSpecs) log() *slog.Logger {
	if uc.Log != nil {
		return uc.Log
	}
	return slog.Default()
}

func (uc *ReconcileTestSpecs) now() time.Time {
	if uc.Now != nil {
		return uc.Now().UTC()
	}
	return time.Now().UTC()
}

// RunReconcileLoop ticks until ctx ends. The first pass waits one interval so a restart does not stampede.
func (uc *ReconcileTestSpecs) RunReconcileLoop(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = 30 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if n, err := uc.Execute(ctx); err != nil {
			uc.log().Warn("test specs: reconcile pass failed", slog.Any("error", err))
		} else if n > 0 {
			uc.log().Info("test specs: reconciled", slog.Int("count", n))
		}
	}
}

// Execute does one pass and returns how many rows it handled. A failure on one row never stops the others.
func (uc *ReconcileTestSpecs) Execute(ctx context.Context) (int, error) {
	timing := uc.Timing.normalized()
	handled := 0
	missing, err := uc.Runs.ListExecutingWithoutRun(ctx, timing.Quiet, timing.Batch)
	if err != nil {
		return 0, err
	}
	for _, ref := range missing {
		tctx := tenant.WithTenantID(ctx, ref.TenantID)
		if _, err := uc.Start.Execute(tctx, ref.RequestID); err != nil {
			uc.log().Warn("test specs: start from reconcile failed", slog.String("request_id", ref.RequestID), slog.Any("error", err))
			continue
		}
		handled++
	}
	due, err := uc.Runs.ListDue(ctx, timing.Quiet, timing.Batch)
	if err != nil {
		return handled, err
	}
	for _, ref := range due {
		tctx := tenant.WithTenantID(ctx, ref.TenantID)
		ok, err := uc.Runs.Claim(tctx, ref.RunID, uc.owner(), timing.Lease)
		if err != nil || !ok {
			continue
		}
		if err := uc.step(tctx, ref, timing); err != nil {
			uc.log().Warn("test specs: reconcile step failed", slog.String("request_id", ref.RequestID), slog.Any("error", err))
		}
		_ = uc.Runs.Release(tctx, ref.RunID, uc.owner())
		handled++
	}
	return handled, nil
}

func (uc *ReconcileTestSpecs) owner() string {
	if uc.Owner == "" {
		return "request-service"
	}
	return uc.Owner
}

// step looks at one current row: a pending or starting one is driven again, a running one is asked about.
func (uc *ReconcileTestSpecs) step(ctx context.Context, ref TestSpecRef, timing TestSpecTiming) error {
	run, found, err := uc.Runs.Current(ctx, ref.RequestID)
	if err != nil || !found || run.ID != ref.RunID || !run.State.Active() {
		return err
	}
	req, err := uc.Requests.Get(ctx, ref.RequestID)
	if err != nil {
		return err
	}
	if run.StartedBy != "" {
		ctx = tenant.WithUserID(ctx, run.StartedBy) // project-service refuses the dev server lookup without a user
	}
	if uc.now().Sub(run.StartedAt) > timing.Deadline {
		uc.endRun(ctx, req, run, domain.TestSpecFailed, domain.TestSpecDeadline, fmt.Sprintf("no result after %s", timing.Deadline), true)
		return nil
	}
	if run.State != domain.TestSpecRunning || run.TDDRunID == "" {
		uc.Start.drive(ctx, req, run)
		return nil
	}
	conn, err := uc.Resolver.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		return uc.touch(ctx, run)
	}
	state, err := uc.Agent.Run(ctx, conn, run.TDDRunID)
	if err != nil {
		var te *tddagent.ToolError
		if errors.As(err, &te) && (te.Code == "RUN_NOT_FOUND" || te.Code == "NOT_FOUND") {
			uc.endRun(ctx, req, run, domain.TestSpecFailed, domain.TestSpecRunLost, "the dev server no longer knows the run", false)
			return nil
		}
		return uc.touch(ctx, run) // not reachable now: look again after the quiet time
	}
	switch state.Status {
	case tddagent.RunCompleted:
		uc.complete(ctx, conn, run)
	case tddagent.RunFailed:
		uc.endRun(ctx, req, run, domain.TestSpecFailed, "TESTSPEC_FAILED", blockersText(state.Blockers), false)
	case tddagent.RunBlocked:
		if state.ProcessAlive {
			return uc.touch(ctx, run)
		}
		status, code := testSpecBlockedReason(state.Blockers)
		uc.endRun(ctx, req, run, status, code, blockersText(state.Blockers), false)
	default:
		return uc.touch(ctx, run)
	}
	return nil
}

// complete reads the summary of the finished run and stores it on the row.
func (uc *ReconcileTestSpecs) complete(ctx context.Context, conn AnalysisConnection, run domain.TestSpecRun) {
	feature := run.Feature
	if feature == "" {
		feature = uc.featureOf(ctx, conn, run)
	}
	run.Feature = feature
	if feature != "" {
		if raw, err := uc.Agent.Summary(ctx, conn, feature); err == nil {
			run.Summary = raw
		} else {
			uc.log().Warn("test specs: summary unavailable", slog.String("request_id", run.RequestID), slog.Any("error", err))
		}
	}
	t := uc.now()
	run.State, run.FinishedAt, run.ReasonCode, run.ReasonMessage = domain.TestSpecCompleted, &t, "", ""
	run.UpdatedAt = t
	if err := uc.Runs.Update(ctx, run); err != nil {
		uc.log().Warn("test specs: save failed", slog.String("run_id", run.ID), slog.Any("error", err))
	}
}

// featureOf is the feature that holds the Request's test specs: the first one the dev server lists (a project has one TDD today).
func (uc *ReconcileTestSpecs) featureOf(ctx context.Context, conn AnalysisConnection, _ domain.TestSpecRun) string {
	features, err := uc.Agent.Features(ctx, conn)
	if err != nil || len(features) == 0 {
		return ""
	}
	return features[0]
}

func (uc *ReconcileTestSpecs) endRun(ctx context.Context, req domain.Request, run domain.TestSpecRun, status domain.TestSpecStatus, code, msg string, cancelRun bool) {
	if cancelRun && run.TDDRunID != "" {
		uc.Start.cancelBestEffort(ctx, req, run, "deadline")
	}
	t := uc.now()
	run.State, run.ReasonCode, run.ReasonMessage, run.FinishedAt, run.UpdatedAt = status, code, truncateRunes(msg, 500), &t, t
	if err := uc.Runs.Update(ctx, run); err != nil {
		uc.log().Warn("test specs: save failed", slog.String("run_id", run.ID), slog.Any("error", err))
	}
}

// touch moves updated_at so the row waits its quiet time before the next look.
func (uc *ReconcileTestSpecs) touch(ctx context.Context, run domain.TestSpecRun) error {
	run.UpdatedAt = uc.now()
	return uc.Runs.Update(ctx, run)
}

func blockersText(b []string) string {
	if len(b) == 0 {
		return "tdd-agent stopped the run"
	}
	out := b[0]
	for _, x := range b[1:] {
		out += ", " + x
	}
	return out
}

// testSpecBlockedReason maps the blockers of a stopped tdd-agent run.
func testSpecBlockedReason(blockers []string) (domain.TestSpecStatus, string) {
	for _, b := range blockers {
		if status, code := testSpecReasonFromAgent(b); status != domain.TestSpecFailed {
			return status, code
		}
	}
	return domain.TestSpecFailed, "TESTSPEC_FAILED"
}
