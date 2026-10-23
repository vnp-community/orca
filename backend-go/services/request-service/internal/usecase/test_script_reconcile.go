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

// ReconcileTestScripts is the loop of the test-script lane: it starts the lane for ready Requests whose events were lost, resumes a
// pending attempt and asks the dev server about running jobs. It does not touch the execution loop. Timing is the test-spec one.
type ReconcileTestScripts struct {
	Runs     TestScriptRunRepository
	Requests RequestReader
	Start    *StartTestScripts
	Agent    TestScriptAgent
	Resolver AnalysisConnectionResolver
	Timing   TestSpecTiming
	Owner    string
	Now      func() time.Time
	Log      *slog.Logger
}

func (uc *ReconcileTestScripts) log() *slog.Logger {
	if uc.Log != nil {
		return uc.Log
	}
	return slog.Default()
}

func (uc *ReconcileTestScripts) now() time.Time {
	if uc.Now != nil {
		return uc.Now().UTC()
	}
	return time.Now().UTC()
}

func (uc *ReconcileTestScripts) owner() string {
	if uc.Owner == "" {
		return "request-service"
	}
	return uc.Owner
}

// RunReconcileLoop ticks until ctx ends. The first pass waits one interval so a restart does not stampede.
func (uc *ReconcileTestScripts) RunReconcileLoop(ctx context.Context, every time.Duration) {
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
			uc.log().Warn("test scripts: reconcile pass failed", slog.Any("error", err))
		} else if n > 0 {
			uc.log().Info("test scripts: reconciled", slog.Int("count", n))
		}
	}
}

// Execute does one pass and returns how many rows it handled. A failure on one row never stops the others.
func (uc *ReconcileTestScripts) Execute(ctx context.Context) (int, error) {
	timing := uc.Timing.normalized()
	handled := 0
	missing, err := uc.Runs.ListReadyWithoutRun(ctx, timing.Quiet, timing.Batch)
	if err != nil {
		return 0, err
	}
	for _, ref := range missing {
		tctx := tenant.WithTenantID(ctx, ref.TenantID)
		if _, err := uc.Start.Execute(tctx, ref.RequestID); err != nil {
			uc.log().Warn("test scripts: start from reconcile failed", slog.String("request_id", ref.RequestID), slog.Any("error", err))
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
			uc.log().Warn("test scripts: reconcile step failed", slog.String("request_id", ref.RequestID), slog.Any("error", err))
		}
		_ = uc.Runs.Release(tctx, ref.RunID, uc.owner())
		handled++
	}
	return handled, nil
}

func (uc *ReconcileTestScripts) step(ctx context.Context, ref TestScriptRef, timing TestSpecTiming) error {
	run, found, err := uc.Runs.Current(ctx, ref.RequestID)
	if err != nil || !found || run.ID != ref.RunID || !run.State.Active() {
		return err
	}
	req, err := uc.Requests.Get(ctx, ref.RequestID)
	if err != nil {
		return err
	}
	if run.StartedBy != "" {
		ctx = tenant.WithUserID(ctx, run.StartedBy)
	}
	if uc.now().Sub(run.StartedAt) > timing.Deadline {
		uc.endRun(ctx, req, run, domain.TestScriptFailed, domain.TestScriptDeadline, fmt.Sprintf("no result after %s", timing.Deadline), true)
		return nil
	}
	if run.State != domain.TestScriptRunning || run.TSAJobID == "" {
		uc.Start.drive(ctx, req, run)
		return nil
	}
	conn, err := uc.Resolver.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		return uc.touch(ctx, run)
	}
	job, err := uc.Agent.Job(ctx, conn, run.TSAJobID)
	if err != nil {
		var te *tddagent.ToolError
		if errors.As(err, &te) && te.Code == "TSA_JOB_NOT_FOUND" {
			uc.endRun(ctx, req, run, domain.TestScriptFailed, domain.TestScriptJobLost, "the dev server no longer knows the job", false)
			return nil
		}
		return uc.touch(ctx, run)
	}
	switch job.State {
	case TSADone:
		run.Branch, run.WorktreePath, run.CommitSHA = firstNonEmpty(job.Branch, run.Branch), job.Worktree, job.Commit
		run.Summary = job.Raw
		status, code, msg := domain.TestScriptCompleted, "", ""
		if !job.AllGenerated {
			status, code, msg = domain.TestScriptNeedsReview, "REQUEST_TESTSCRIPT_NEEDS_REVIEW", "some files did not pass the spec check; read the report"
		}
		uc.endRun(ctx, req, run, status, code, msg, false)
	case TSAError:
		run.Summary = job.Raw
		status, code := testScriptReasonFromAgent(job.ErrorCode)
		uc.endRun(ctx, req, run, status, code, job.ErrorText, false)
	case TSACancelled:
		uc.endRun(ctx, req, run, domain.TestScriptCancelled, "", job.ErrorText, false)
	default:
		return uc.touch(ctx, run)
	}
	return nil
}

func (uc *ReconcileTestScripts) endRun(ctx context.Context, req domain.Request, run domain.TestScriptRun, status domain.TestScriptStatus, code, msg string, cancelJob bool) {
	if cancelJob && run.TSAJobID != "" {
		uc.Start.cancelBestEffort(ctx, req, run, "deadline")
	}
	t := uc.now()
	run.State, run.ReasonCode, run.ReasonMessage, run.FinishedAt, run.UpdatedAt = status, code, truncateRunes(msg, 500), &t, t
	if err := uc.Runs.Update(ctx, run); err != nil {
		uc.log().Warn("test scripts: save failed", slog.String("run_id", run.ID), slog.Any("error", err))
	}
}

func (uc *ReconcileTestScripts) touch(ctx context.Context, run domain.TestScriptRun) error {
	run.UpdatedAt = uc.now()
	return uc.Runs.Update(ctx, run)
}
