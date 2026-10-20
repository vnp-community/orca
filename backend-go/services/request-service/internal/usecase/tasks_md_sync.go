package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	// syncBatch bounds how many changes one tenant's pass handles.
	syncBatch = 50
	// archiveTimeoutMs bounds `openspec archive`: it moves a directory and merges spec deltas, nothing more.
	archiveTimeoutMs = 60_000
	syncErrorRunes   = 500
)

// SyncObserver counts what the sync and the archive step did. May be absent.
type SyncObserver interface {
	// TasksDrift is one plan region found edited by hand, restored or (when the stored copy is masked) left as it is.
	TasksDrift()
	// SyncFinished is one step that ended: step tasks|archive, result done|failed|gave_up.
	SyncFinished(step, result string)
}

type noopSyncObserver struct{}

func (noopSyncObserver) TasksDrift()                 {}
func (noopSyncObserver) SyncFinished(string, string) {}

// TasksMdSyncer keeps the tasks.md of an OpenSpec change in step with the Plan and archives the change when the Request
// completes. It writes only in the Request's proposal worktree on its local branch and never pushes. What it ticks is
// derived from task state on every pass, so a lost event only delays a tick. A failure here never changes the Request.
type TasksMdSyncer struct {
	Changes   OpenSpecChangeRepository
	Requests  RequestReader
	Tasks     TaskClient
	Outcomes  TaskRunOutcomeRepository
	Runs      AnalysisRunStore
	Workspace ProposalWorkspace
	Exec      DevServerExecutor
	Conns     AnalysisConnectionResolver
	Tenants   TenantLister
	Observer  SyncObserver
	Now       func() time.Time
	Log       *slog.Logger
}

func (s *TasksMdSyncer) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *TasksMdSyncer) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

func (s *TasksMdSyncer) observer() SyncObserver {
	if s.Observer != nil {
		return s.Observer
	}
	return noopSyncObserver{}
}

// OnTaskSucceeded marks the change as behind its Plan; the next pass ticks tasks.md. A Request without a ready change is
// not an OpenSpec Request and is left alone.
func (s *TasksMdSyncer) OnTaskSucceeded(ctx context.Context, requestID string) error {
	for range 3 {
		change, found, err := s.Changes.GetByRequest(ctx, requestID)
		if err != nil || !found || change.Status != domain.ChangeStatusReady || change.SyncState == domain.SyncStatePending {
			return err
		}
		u := syncUpdateOf(change)
		u.State, u.NextRetryAt = domain.SyncStatePending, nil
		if u.Attempts >= domain.MaxSyncAttempts {
			return nil // given up: a new task result does not reopen a change that could not be written
		}
		err = s.Changes.UpdateSync(ctx, change.ID, u, change.Version)
		if !errors.Is(err, domain.ErrOpenSpecChangeVersionConflict) {
			return err
		}
	}
	return nil
}

func syncUpdateOf(c domain.OpenSpecChange) domain.SyncUpdate {
	return domain.SyncUpdate{State: c.SyncState, Digest: c.SyncDigest, Error: c.SyncError, Attempts: c.SyncAttempts, NextRetryAt: c.SyncNextRetryAt}
}

// SyncSummary is what one pass did, for the log.
type SyncSummary struct {
	Tenants, Looked, Ticked, Archived, Failed int
}

// Sweep visits every tenant once. One tenant's trouble never stops the others.
func (s *TasksMdSyncer) Sweep(ctx context.Context) (SyncSummary, error) {
	var sum SyncSummary
	tenants, err := s.Tenants.ListTenants(ctx)
	if err != nil {
		return sum, fmt.Errorf("list tenants: %w", err)
	}
	for _, id := range tenants {
		if ctx.Err() != nil {
			return sum, ctx.Err()
		}
		sum.Tenants++
		tctx := tenant.WithActorType(tenant.WithTenantID(ctx, id), tenant.ActorSystem)
		work, err := s.Changes.ListSyncWork(tctx, syncBatch)
		if err != nil {
			s.log().WarnContext(tctx, "tasks.md sync could not list work", slog.Any("error", err))
			sum.Failed++
			continue
		}
		for _, c := range work {
			sum.Looked++
			out, err := s.Flush(tctx, c.RequestID)
			sum.Ticked += out.Ticked
			if out.Archived {
				sum.Archived++
			}
			if err != nil {
				sum.Failed++
			}
		}
	}
	return sum, nil
}

// Run sweeps every interval until ctx ends.
func (s *TasksMdSyncer) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sum, err := s.Sweep(ctx)
			switch {
			case err != nil && ctx.Err() == nil:
				s.log().WarnContext(ctx, "tasks.md sync pass failed", slog.Any("error", err))
			case sum.Ticked > 0 || sum.Archived > 0 || sum.Failed > 0:
				s.log().InfoContext(ctx, "tasks.md sync pass", slog.Int("looked", sum.Looked), slog.Int("ticked", sum.Ticked),
					slog.Int("archived", sum.Archived), slog.Int("failed", sum.Failed))
			}
		}
	}
}

// FlushResult says what one Flush did.
type FlushResult struct {
	Ticked   int
	Archived bool
}

// Flush brings one Request's tasks.md up to date and, when the Request is completed, archives its change. It does nothing
// while a plan run writes in the worktree. A failure is recorded with a retry time; the error is returned for the log only.
func (s *TasksMdSyncer) Flush(ctx context.Context, requestID string) (FlushResult, error) {
	var out FlushResult
	change, found, err := s.Changes.GetByRequest(ctx, requestID)
	if err != nil || !found || change.Status != domain.ChangeStatusReady || change.SyncState == domain.SyncStateFailed {
		return out, err
	}
	if due := change.SyncNextRetryAt; due != nil && due.After(s.now()) {
		return out, nil
	}
	req, err := s.Requests.Get(ctx, requestID)
	if err != nil {
		return out, err
	}
	// The sweep has no user, and project-service and git-gateway want one to list the repo and make the worktree: it acts for
	// the Request's reporter, whose project this is, as the plan run that wrote the change did.
	ctx = tenant.WithUserID(ctx, req.ReporterID)
	if active, err := s.planRunActive(ctx, requestID); err != nil || active {
		return out, err
	}
	step, failedBefore := "tasks", change.SyncAttempts // the tick step clears the counter when it succeeds; a failing archive must still count
	ticked, err := s.syncTasks(ctx, &change, req)
	out.Ticked = ticked
	if err == nil && req.Status == domain.RequestStatusCompleted {
		step = "archive"
		err = s.archive(ctx, &change, req)
		out.Archived = err == nil
	}
	if err != nil {
		return out, s.recordFailure(ctx, change, step, failedBefore, err)
	}
	if out.Ticked > 0 || out.Archived {
		s.observer().SyncFinished(step, "done")
	}
	return out, nil
}

func (s *TasksMdSyncer) planRunActive(ctx context.Context, requestID string) (bool, error) {
	runs, err := s.Runs.ListRecent(ctx, requestID, 10)
	if err != nil {
		return false, err
	}
	for _, r := range runs {
		if r.Kind == domain.RunKindPlan && r.Status == domain.RunStatusRunning {
			return true, nil
		}
	}
	return false, nil
}

func (s *TasksMdSyncer) workspace(ctx context.Context, change domain.OpenSpecChange, req domain.Request) (Workspace, error) {
	return s.Workspace.Ensure(ctx, EnsureWorkspaceInput{
		TenantID: req.TenantID, ProjectID: req.ProjectID, RequestID: req.ID, RequestNumber: req.Number, ChangeID: change.ChangeID, BaseRef: req.BaseRef,
	})
}

func tasksMdPath(changeID string) string { return "openspec/changes/" + changeID + "/" + tasksMDName }

// doneRefs lists the tasks.md lines whose Plan task has a succeeded run. A task the user marked done by hand counts too.
func (s *TasksMdSyncer) doneRefs(ctx context.Context, requestID, planTaskID string) ([]string, error) {
	tasks, err := s.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: []string{requestID}, TaskTypes: executionTaskTypes})
	if err != nil {
		return nil, err
	}
	var refs []string
	// Only the Plan the Request runs now: an earlier Plan of the same Request has tasks with the same labels, done or not.
	for _, t := range domain.BuildExecutionTreeOf(planTaskID, tasks).Leaves {
		ref, ok := domain.SpecRefFromLabels(t.Labels)
		if !ok {
			continue
		}
		done := t.Status == domain.TaskStatusDone
		if !done {
			if done, err = s.Outcomes.Exists(ctx, t.ID, domain.OutcomeSucceeded); err != nil {
				return nil, err
			}
		}
		if done {
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

// syncTasks ticks the done tasks in the proposal's tasks.md and commits only that file. A plan region that no longer
// matches what Orca last wrote was edited by hand: the stored region is put back first, then the ticks are re-applied.
func (s *TasksMdSyncer) syncTasks(ctx context.Context, change *domain.OpenSpecChange, req domain.Request) (int, error) {
	refs, err := s.doneRefs(ctx, req.ID, req.PlanTaskID)
	if err != nil {
		return 0, fmt.Errorf("read task progress: %w", err)
	}
	ws, err := s.workspace(ctx, *change, req)
	if err != nil {
		return 0, fmt.Errorf("prepare the proposal worktree: %w", err)
	}
	path := tasksMdPath(change.ChangeID)
	raw, err := s.Workspace.ReadFile(ctx, ws, path, MaxProposalReadBytes)
	if err != nil {
		return 0, fmt.Errorf("read tasks.md: %w", err)
	}
	md := string(raw)
	region, ok := domain.PlanRegionText(md)
	if !ok {
		return 0, errors.New("tasks.md has no plan region")
	}
	next, drift, err := s.restoreDrift(*change, md, region)
	if err != nil {
		return 0, err
	}
	if drift == driftKept {
		s.observer().TasksDrift()
		s.log().WarnContext(ctx, "tasks.md plan region was edited by hand; kept as is because the stored copy is masked", slog.String("request_id", req.ID))
	}
	next, ticked, err := domain.TickTasks(next, refs)
	if err != nil {
		return 0, err
	}
	if next == md {
		return 0, s.recordInSync(ctx, change, md)
	}
	if drift == driftRestored {
		s.observer().TasksDrift()
		s.log().WarnContext(ctx, "tasks.md plan region was edited by hand; the stored region was restored", slog.String("request_id", req.ID))
	}
	if err := s.Workspace.WriteFile(ctx, ws, path, []byte(next)); err != nil {
		return 0, fmt.Errorf("write tasks.md: %w", err)
	}
	if _, err := s.Workspace.Commit(ctx, ws, "chore(openspec): tick finished tasks of "+change.ChangeID, []string{path}); err != nil {
		_ = s.Workspace.DiscardAll(ctx, ws) // a half-written tick must not be swept into a later commit
		return 0, fmt.Errorf("commit tasks.md: %w", err)
	}
	return len(ticked), s.recordInSync(ctx, change, next)
}

// planRegionDrift says what restoreDrift found: nothing, a hand edit it undid, or one it had to leave in place.
type planRegionDrift int

const (
	driftNone planRegionDrift = iota
	driftRestored
	driftKept
)

// restoreDrift puts the stored plan region back when the one in the worktree differs from what Orca last wrote. When the
// stored copy was masked it is never written back, or tasks.md on the branch would hold the mask: the edited region stays.
func (s *TasksMdSyncer) restoreDrift(change domain.OpenSpecChange, md, region string) (string, planRegionDrift, error) {
	if change.RedactedSecrets > 0 {
		return md, maskedDrift(change, md, region), nil
	}
	want := change.SyncDigest
	stored, hasStored := domain.PlanRegionText(change.Bundle[tasksMDName])
	if want == "" && hasStored {
		want = domain.PlanRegionDigest(stored) // never synced: the authored region is the baseline
	}
	if want == "" || domain.PlanRegionDigest(region) == want || !hasStored {
		return md, driftNone, nil
	}
	restored, err := domain.ReplacePlanRegion(md, stored)
	return restored, driftRestored, err
}

// maskedDrift detects a hand edit without the stored text: before the first sync the baseline is the digest of the whole
// raw file taken when it was authored (TasksMDDigest), after it the digest of the region Orca wrote.
func maskedDrift(change domain.OpenSpecChange, md, region string) planRegionDrift {
	if change.SyncDigest == "" {
		if change.TasksMDDigest != "" && domain.PlanRegionDigest(md) != change.TasksMDDigest {
			return driftKept
		}
		return driftNone
	}
	if domain.PlanRegionDigest(region) != change.SyncDigest {
		return driftKept
	}
	return driftNone
}

func (s *TasksMdSyncer) recordInSync(ctx context.Context, change *domain.OpenSpecChange, md string) error {
	region, _ := domain.PlanRegionText(md)
	u := domain.SyncUpdate{State: domain.SyncStateInSync, Digest: domain.PlanRegionDigest(region)}
	if change.SyncState == domain.SyncStateInSync && change.SyncDigest == u.Digest && change.SyncAttempts == 0 && change.SyncError == "" && change.SyncNextRetryAt == nil {
		return nil
	}
	if err := s.Changes.UpdateSync(ctx, change.ID, u, change.Version); err != nil {
		return err
	}
	change.SyncState, change.SyncDigest, change.SyncAttempts, change.SyncError, change.SyncNextRetryAt = u.State, u.Digest, 0, "", nil
	change.Version++
	return nil
}

// archive runs `openspec archive` in the proposal worktree and commits what it changed, after checking that it changed
// nothing outside openspec/. Anything else is discarded, so the archive can never become a way to edit code.
func (s *TasksMdSyncer) archive(ctx context.Context, change *domain.OpenSpecChange, req domain.Request) error {
	ws, err := s.workspace(ctx, *change, req)
	if err != nil {
		return fmt.Errorf("prepare the proposal worktree: %w", err)
	}
	conn, err := s.Conns.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		return fmt.Errorf("no dev server for the archive: %w", err)
	}
	res, err := s.Exec.Exec(ctx, conn, ExecInput{Binary: "openspec", Args: []string{"archive", change.ChangeID, "--yes"}, Cwd: ws.Path, TimeoutMs: archiveTimeoutMs})
	if err == nil {
		err = archiveExecError(res)
	}
	if err != nil {
		_ = s.Workspace.DiscardAll(ctx, ws)
		return err
	}
	changed, err := s.Workspace.ChangedPaths(ctx, ws)
	if err != nil {
		return fmt.Errorf("read what the archive changed: %w", err)
	}
	paths := changedPathNames(changed)
	if bad := domain.ArchiveScopeViolations(paths); len(bad) > 0 {
		_ = s.Workspace.DiscardAll(ctx, ws)
		return fmt.Errorf("the archive changed paths outside openspec/: %s", strings.Join(firstN(bad, 5), ", "))
	}
	if len(paths) == 0 {
		return errors.New("the archive changed nothing")
	}
	sha, err := s.Workspace.Commit(ctx, ws, "chore(openspec): archive "+change.ChangeID+" (completed)", paths)
	if err != nil {
		_ = s.Workspace.DiscardAll(ctx, ws)
		return fmt.Errorf("commit the archive: %w", err)
	}
	if err := change.MarkArchived(sha); err != nil {
		return err
	}
	change.SyncState, change.SyncAttempts, change.SyncError, change.SyncNextRetryAt = domain.SyncStateInSync, 0, "", nil
	if _, err := s.Changes.Upsert(ctx, *change); err != nil {
		return fmt.Errorf("record the archive: %w", err)
	}
	return nil
}

func archiveExecError(res ExecResult) error {
	switch {
	case res.SpawnError != "":
		return fmt.Errorf("openspec could not be started: %s", truncateRunes(res.SpawnError, 200))
	case res.TimedOut:
		return errors.New("openspec archive timed out")
	case res.ExitCode != 0:
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(res.Stdout)
		}
		return fmt.Errorf("openspec archive exited with code %d: %s", res.ExitCode, truncateRunes(msg, 300))
	}
	return nil
}

func changedPathNames(changed []ChangedPath) []string {
	var out []string
	for _, c := range changed {
		out = append(out, c.Path)
		if c.OldPath != "" {
			out = append(out, c.OldPath)
		}
	}
	return out
}

func firstN(in []string, n int) []string {
	if len(in) > n {
		return in[:n]
	}
	return in
}

// recordFailure keeps the change pending with a back-off, and leaves it failed after MaxSyncAttempts. The Request is untouched.
func (s *TasksMdSyncer) recordFailure(ctx context.Context, change domain.OpenSpecChange, step string, failedBefore int, cause error) error {
	attempts := failedBefore + 1
	u := domain.SyncUpdate{State: domain.SyncStatePending, Digest: change.SyncDigest, Error: truncateRunes(cause.Error(), syncErrorRunes), Attempts: attempts}
	result := "failed"
	if attempts >= domain.MaxSyncAttempts {
		u.State, result = domain.SyncStateFailed, "gave_up"
	} else {
		at := s.now().Add(domain.SyncBackoff(attempts))
		u.NextRetryAt = &at
	}
	s.observer().SyncFinished(step, result)
	s.log().WarnContext(ctx, "tasks.md sync step failed", slog.String("request_id", change.RequestID), slog.String("step", step),
		slog.Int("attempt", attempts), slog.String("result", result), slog.Any("error", cause))
	if err := s.Changes.UpdateSync(ctx, change.ID, u, change.Version); err != nil && !errors.Is(err, domain.ErrOpenSpecChangeVersionConflict) {
		return errors.Join(cause, err)
	}
	return cause
}
