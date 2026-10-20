package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// revisionKit is a Plan that already exists and waits for approval, with the worker's job store attached the way
// wirePlanAuto does it.
type revisionKit struct {
	*planKit
	jobs *memPlanAutoJobs
	req  domain.Request
}

func newRevisionKit(conns AnalysisConnectionResolver, reqType domain.RequestType, withWorker bool) *revisionKit {
	k := newPlanKit(conns)
	jobs := newMemPlanAutoJobs(time.Now)
	k.generate.tx = planAutoTx{w: k.w, proc: newMemProcessedEvents(), jobs: jobs}
	if withWorker {
		k.generate.WithRevisionWorker(jobs, NewPlanActorAuthorizer(allowAll{}, nil, true))
	}
	req := k.w.seedRequest(reqType, domain.RequestStatusAwaitingPlanApproval)
	req.PlanTaskID = "plan-1"
	k.w.requests[req.ID] = req
	k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)
	return &revisionKit{planKit: k, jobs: jobs, req: req}
}

func (k *revisionKit) as(user string) context.Context { return tenant.WithUserID(lcCtx(), user) }

func (k *revisionKit) reporter() context.Context { return k.as(k.req.ReporterID) }

func (k *revisionKit) seedRun(status domain.RunStatus, actor, feedback string, startedAt time.Time) domain.AnalysisRun {
	run := domain.AnalysisRun{ID: uuid.NewString(), TenantID: "t1", RequestID: k.req.ID, Kind: domain.RunKindPlan, Status: status, ActorID: actor, Feedback: feedback, StartedAt: startedAt}
	k.w.runs[run.ID] = run
	return run
}

func TestGeneratePlan_Revise_InsertsRevisionJobInSameTx(t *testing.T) {
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
	res, err := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: k.req.ID, Feedback: "split the migration"})
	if err != nil || !res.Created {
		t.Fatalf("revise: %+v %v", res, err)
	}
	job := k.jobs.get(k.req.ID, domain.RevisionAnchor(res.RunID))
	if job.Origin != domain.PlanAutoOriginRevision || job.State != domain.PlanAutoRunning || job.RunID != res.RunID || job.Attempt != 1 || job.MaxAttempts != 1 ||
		job.RequestedBy != k.req.ReporterID || job.ProjectID != k.req.ProjectID || job.TenantID != "t1" || k.jobs.count() != 1 {
		t.Fatalf("job = %+v (count %d)", job, k.jobs.count())
	}
	if len(k.spawner.runs) != 1 {
		t.Fatalf("spawned %d runs", len(k.spawner.runs))
	}
}

func TestGeneratePlan_Revise_FirstPlanInsertsNoJob(t *testing.T) {
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
	req := k.req
	req.Status, req.PlanTaskID = domain.RequestStatusPlanning, ""
	k.w.requests[req.ID] = req
	if _, err := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: req.ID}); err != nil {
		t.Fatal(err)
	}
	if k.jobs.count() != 0 {
		t.Fatalf("the first Plan is committed by the Analysis event, not by a revision job: %d job(s)", k.jobs.count())
	}
}

func TestGeneratePlan_Revise_ActiveJobConflictIsRevisionInProgress(t *testing.T) {
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
	k.jobs.seed(domain.PlanAutoJob{TenantID: "t1", RequestID: k.req.ID, Anchor: "rev-earlier", Origin: domain.PlanAutoOriginRevision, State: domain.PlanAutoRunning, Attempt: 1, MaxAttempts: 1})
	_, err := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: k.req.ID, Feedback: "again"})
	requireCode(t, err, "REQUEST_PLAN_REVISION_IN_PROGRESS")
	if len(k.w.runs) != 0 || len(k.w.cancelled) != 0 || len(k.spawner.runs) != 0 || k.jobs.count() != 1 {
		t.Fatalf("a refused revision must leave nothing: %d run(s), %d cancel(s), %d spawn(s), %d job(s)", len(k.w.runs), len(k.w.cancelled), len(k.spawner.runs), k.jobs.count())
	}
	if got := k.w.requests[k.req.ID].Status; got != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("status = %s: the Plan must still wait for approval", got)
	}
}

func TestGeneratePlan_Revise_NoJobRepository_NoJobAndApproverForbidden(t *testing.T) {
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, false)
	if k.generate.CommitsRevisions() {
		t.Fatal("no worker is attached")
	}
	if _, err := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: k.req.ID, Feedback: "x"}); err != nil {
		t.Fatal(err)
	}
	if k.jobs.count() != 0 {
		t.Fatal("without the worker nobody commits a job, so none is queued")
	}

	// The same deployment with the real authorizer: an approver is turned away because nobody could commit for them.
	f := newReviseFixture(domain.SubjectPlan, domain.Principal{Kind: domain.PrincipalKindUser, ID: "u-approver"})
	off := NewPlanActorAuthorizer(f.authorizer(), fakeRuns{k.w}, false)
	k2 := newRevisionKit(nil, domain.RequestTypeChangeRequest, false)
	k2.generate.auth = off
	_, err := k2.generate.Execute(reviseActor("u-approver", "member"), GeneratePlanInput{RequestID: k2.req.ID, Feedback: "x"})
	requireCode(t, err, "REQUEST_PLAN_FORBIDDEN")

	k2.generate.WithRevisionWorker(k2.jobs, NewPlanActorAuthorizer(f.authorizer(), fakeRuns{k2.w}, true))
	if !k2.generate.CommitsRevisions() {
		t.Fatal("the worker is attached")
	}
}

func TestGeneratePlan_Revise_InheritedFeedbackInsertsJob(t *testing.T) {
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
	req := k.req
	req.Status = domain.RequestStatusPlanning
	k.w.requests[req.ID] = req
	k.seedRun(domain.RunStatusFailed, req.ReporterID, "split the migration", time.Now().Add(-time.Minute))

	res, err := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: req.ID})
	if err != nil || !res.Created {
		t.Fatalf("retry: %+v %v", res, err)
	}
	if got := k.w.runs[res.RunID].Feedback; got != "split the migration" {
		t.Fatalf("feedback = %q, want the failed run's ask", got)
	}
	if job := k.jobs.get(req.ID, domain.RevisionAnchor(res.RunID)); job.Origin != domain.PlanAutoOriginRevision || job.RunID != res.RunID {
		t.Fatalf("a retry without text is still a revision and needs its job: %+v", job)
	}
}

func TestGeneratePlan_IdempotentDoubleClick(t *testing.T) {
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
	in := GeneratePlanInput{RequestID: k.req.ID, Feedback: "split", IdempotencyKey: "click-1"}
	first, err := k.generate.Execute(k.reporter(), in)
	if err != nil || !first.Created {
		t.Fatalf("first: %+v %v", first, err)
	}
	second, err := k.generate.Execute(k.reporter(), in)
	if err != nil || second.Created || second.RunID != first.RunID {
		t.Fatalf("second click: %+v %v", second, err)
	}
	if len(k.spawner.runs) != 1 || len(k.w.runs) != 1 || k.jobs.count() != 1 || len(k.w.cancelled) != 1 {
		t.Fatalf("a double click must change nothing twice: %d spawn(s), %d run(s), %d job(s), %d cancel(s)", len(k.spawner.runs), len(k.w.runs), k.jobs.count(), len(k.w.cancelled))
	}
	if key := k.w.runs[first.RunID].IdempotencyKey; key == nil || *key != "click-1" {
		t.Fatalf("the key must be stored on the run: %v", key)
	}
}

func TestGeneratePlan_SameKeyReturnsRunningRun(t *testing.T) {
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
	first, err := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: k.req.ID, Feedback: "split", IdempotencyKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	// A retried HTTP call arrives from another connection of the same person.
	again, err := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: k.req.ID, Feedback: "split", IdempotencyKey: "k"})
	if err != nil || again.RunID != first.RunID || again.Created {
		t.Fatalf("same key: %+v %v", again, err)
	}
}

func TestGeneratePlan_SecondReviserRefused(t *testing.T) {
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
	first, err := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: k.req.ID, Feedback: "split", IdempotencyKey: "a"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = k.generate.Execute(k.as("u-admin"), GeneratePlanInput{RequestID: k.req.ID, Feedback: "something else", IdempotencyKey: "b"})
	requireCode(t, err, "REQUEST_PLAN_REVISION_IN_PROGRESS")
	_, err = k.generate.Execute(k.as("u-admin"), GeneratePlanInput{RequestID: k.req.ID, Feedback: "something else"})
	requireCode(t, err, "REQUEST_PLAN_REVISION_IN_PROGRESS")
	if len(k.w.runs) != 1 || k.w.runs[first.RunID].Feedback != "split" || k.jobs.count() != 1 {
		t.Fatalf("the first person's ask must stay the only run: %d run(s), %d job(s)", len(k.w.runs), k.jobs.count())
	}
}

func TestGeneratePlan_PlainRegenerateKeepsTheQuietReuseOfARunningRun(t *testing.T) {
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
	req := k.req
	req.Status, req.PlanTaskID = domain.RequestStatusPlanning, ""
	k.w.requests[req.ID] = req
	first, _ := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: req.ID})
	second, err := k.generate.Execute(k.as("u-admin"), GeneratePlanInput{RequestID: req.ID})
	if err != nil || second.Created || second.RunID != first.RunID {
		t.Fatalf("a start without feedback has nothing to refuse: %+v %v", second, err)
	}
}

func TestInheritedPlanFeedback_Boundaries(t *testing.T) {
	cases := []struct {
		name string
		runs func(k *revisionKit)
		want string
	}{
		{"no run at all", func(*revisionKit) {}, ""},
		{"latest failed", func(k *revisionKit) { k.seedRun(domain.RunStatusFailed, "u", "ask", time.Now()) }, "ask"},
		{"latest succeeded", func(k *revisionKit) { k.seedRun(domain.RunStatusSucceeded, "u", "ask", time.Now()) }, ""},
		{"latest running", func(k *revisionKit) { k.seedRun(domain.RunStatusRunning, "u", "ask", time.Now()) }, ""},
		{"an older failure behind a newer success", func(k *revisionKit) {
			k.seedRun(domain.RunStatusFailed, "u", "old", time.Now().Add(-time.Hour))
			k.seedRun(domain.RunStatusSucceeded, "u", "", time.Now())
		}, ""},
		{"a solution run in between is not a plan run", func(k *revisionKit) {
			k.seedRun(domain.RunStatusFailed, "u", "ask", time.Now().Add(-time.Hour))
			k.w.runs["sol"] = domain.AnalysisRun{ID: "sol", RequestID: k.req.ID, Kind: domain.RunKindSolution, Status: domain.RunStatusSucceeded, StartedAt: time.Now()}
		}, "ask"},
	}
	for _, c := range cases {
		k := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
		c.runs(k)
		got, err := inheritedPlanFeedback(context.Background(), fakeRuns{k.w}, k.req.ID)
		if err != nil || got != c.want {
			t.Errorf("%s: got %q err %v, want %q", c.name, got, err, c.want)
		}
	}
}

func TestGeneratePlan_FailedRevisionKeepsAsk(t *testing.T) {
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
	first, _ := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: k.req.ID, Feedback: "split the migration"})
	failed := k.w.runs[first.RunID]
	code := "REQUEST_OPENSPEC_TIMEOUT"
	failed.Status, failed.ErrorCode = domain.RunStatusFailed, &code
	k.w.runs[first.RunID] = failed
	// The worker has given the first job up; its job is final.
	job := k.jobs.get(k.req.ID, domain.RevisionAnchor(first.RunID))
	job.State = domain.PlanAutoGaveUp
	k.jobs.seed(job)

	// "Regenerate" with no text runs again with the same ask and its own job.
	again, err := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: k.req.ID})
	if err != nil || !again.Created || again.RunID == first.RunID {
		t.Fatalf("retry: %+v %v", again, err)
	}
	if got := k.w.runs[again.RunID].Feedback; got != "split the migration" {
		t.Fatalf("feedback = %q", got)
	}
	if k.jobs.get(k.req.ID, domain.RevisionAnchor(again.RunID)).State != domain.PlanAutoRunning {
		t.Fatal("the retry needs a job of its own to be committed")
	}
}

func TestGeneratePlan_NewAskReplacesInherited(t *testing.T) {
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
	req := k.req
	req.Status = domain.RequestStatusPlanning
	k.w.requests[req.ID] = req
	k.seedRun(domain.RunStatusFailed, req.ReporterID, "old ask", time.Now().Add(-time.Minute))
	res, err := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: req.ID, Feedback: "new ask"})
	if err != nil || k.w.runs[res.RunID].Feedback != "new ask" {
		t.Fatalf("new ask: %+v %v", k.w.runs[res.RunID], err)
	}
}

func TestGeneratePlan_HotfixRevisionNotApplicable(t *testing.T) {
	var asked int
	k := newRevisionKit(countingConns{n: &asked}, domain.RequestTypeHotfix, true)
	_, err := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: k.req.ID, Feedback: "change the fix"})
	requireCode(t, err, "REQUEST_PLAN_REVISION_NOT_APPLICABLE")
	if asked != 0 || len(k.w.runs) != 0 || len(k.w.cancelled) != 0 || k.jobs.count() != 0 {
		t.Fatalf("a refused revision must not even look for the dev server: asked=%d runs=%d cancels=%d jobs=%d", asked, len(k.w.runs), len(k.w.cancelled), k.jobs.count())
	}
	if got := k.w.requests[k.req.ID].Status; got != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("status = %s", got)
	}
}

type countingConns struct{ n *int }

func (c countingConns) ResolveForProject(context.Context, string) (AnalysisConnection, error) {
	*c.n++
	return AnalysisConnection{}, nil
}

func TestGeneratePlan_NoDevServerWritesNothing(t *testing.T) {
	k := newRevisionKit(fakeConns{err: ErrNoDevServer}, domain.RequestTypeChangeRequest, true)
	_, err := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: k.req.ID, Feedback: "x"})
	requireCode(t, err, "REQUEST_PLAN_AI_UNAVAILABLE")
	if len(k.w.runs) != 0 || len(k.w.cancelled) != 0 || k.jobs.count() != 0 || len(k.spawner.runs) != 0 {
		t.Fatalf("a refused revision must leave nothing: %d run(s), %d cancel(s), %d job(s)", len(k.w.runs), len(k.w.cancelled), k.jobs.count())
	}
}

func TestGeneratePlan_ApproverResolvesDevServerAsReporter(t *testing.T) {
	var seen string
	k := newRevisionKit(userRecordingConns{seen: &seen}, domain.RequestTypeChangeRequest, true)
	approver := uuid.NewString()
	if _, err := k.generate.Execute(k.as(approver), GeneratePlanInput{RequestID: k.req.ID, Feedback: "x"}); err != nil {
		t.Fatal(err)
	}
	if seen != k.req.ReporterID {
		t.Fatalf("the dev-server lookup ran as %q, want the reporter %q", seen, k.req.ReporterID)
	}
}

func TestGeneratePlan_RunAttributedToCaller(t *testing.T) {
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
	approver := uuid.NewString()
	res, err := k.generate.Execute(k.as(approver), GeneratePlanInput{RequestID: k.req.ID, Feedback: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if got := k.w.runs[res.RunID].ActorID; got != approver {
		t.Fatalf("the run belongs to %q, want the approver %q", got, approver)
	}
	if job := k.jobs.get(k.req.ID, domain.RevisionAnchor(res.RunID)); job.RequestedBy != approver {
		t.Fatalf("job requested_by = %q", job.RequestedBy)
	}
}

func TestGeneratePlan_IdempotencyKeyTooLong(t *testing.T) {
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
	ok := strings.Repeat("k", domain.PlanAutoKeyMaxLen)
	if _, err := k.generate.Execute(k.reporter(), GeneratePlanInput{RequestID: k.req.ID, Feedback: "x", IdempotencyKey: ok}); err != nil {
		t.Fatalf("a key of exactly the limit is fine: %v", err)
	}
	k2 := newRevisionKit(nil, domain.RequestTypeChangeRequest, true)
	_, err := k2.generate.Execute(k2.reporter(), GeneratePlanInput{RequestID: k2.req.ID, Feedback: "x", IdempotencyKey: ok + "k"})
	requireCode(t, err, "REQUEST_PLAN_IDEMPOTENCY_KEY_INVALID")
	if len(k2.w.runs) != 0 {
		t.Fatal("a refused key must not start a run")
	}
}

func TestGeneratePlan_ApproverMayRepeatTheirOwnRunningRevision(t *testing.T) {
	// The approval is gone after the first click, so the second one passes only through the caller's own run.
	f := newReviseFixture(domain.SubjectPlan, domain.Principal{Kind: domain.PrincipalKindUser, ID: "u-approver"})
	k := newRevisionKit(nil, domain.RequestTypeChangeRequest, false)
	jobs := k.jobs
	k.generate.WithRevisionWorker(jobs, NewPlanActorAuthorizer(f.authorizer(), fakeRuns{k.w}, true))
	// reviseFixture owns the approvals: the approver is the pending approver of the Plan gate only for the first click.
	in := GeneratePlanInput{RequestID: k.req.ID, Feedback: "x", IdempotencyKey: "same"}
	first, err := k.generate.Execute(reviseActor("u-approver", "member"), in)
	if err != nil {
		t.Fatal(err)
	}
	f.pending.bySubject = map[domain.SubjectType][]domain.Approval{}
	second, err := k.generate.Execute(reviseActor("u-approver", "member"), in)
	if err != nil || second.RunID != first.RunID || second.Created {
		t.Fatalf("repeat: %+v %v", second, err)
	}
	_, err = k.generate.Execute(reviseActor("u-approver", "member"), GeneratePlanInput{RequestID: k.req.ID, Feedback: "y", IdempotencyKey: "other"})
	requireCode(t, err, "REQUEST_PLAN_REVISION_IN_PROGRESS")
	_, err = k.generate.Execute(reviseActor("u-stranger", "member"), GeneratePlanInput{RequestID: k.req.ID, Feedback: "y"})
	requireCode(t, err, "REQUEST_PLAN_FORBIDDEN")
}
