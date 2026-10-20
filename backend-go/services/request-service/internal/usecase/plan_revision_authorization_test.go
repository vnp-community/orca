package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

var gateSubjectsForPlanRevise = []domain.SubjectType{domain.SubjectPlan, domain.SubjectTaskList, domain.SubjectPreDeploy}

var planReviseActors = []struct {
	name       string
	user, role string
	baseOnly   bool // passes through the reporter/admin rule, so also with approversMayRevise off
	approver   bool
}{
	{"reporter", "u-reporter", "member", true, false},
	{"admin", "u-admin", "admin", true, false},
	{"lead", "u-lead", "lead", true, false},
	{"owner", "u-owner", "owner", true, false},
	{"approver by user", "u-approver", "member", false, true},
	{"approver by team", "u-team", "member", false, true},
	{"approver by role", "u-qa", "qa", false, true},
	{"member who is no approver", "u-member", "member", false, false},
	{"read-only user", "u-viewer", "viewer", false, false},
	{"stranger", "u-stranger", "member", false, false},
}

func planReviseAuthorizer(f *reviseFixture, runs AnalysisRunStore, mayRevise bool) PlanActorAuthorizer {
	return NewPlanActorAuthorizer(f.authorizer(), runs, mayRevise)
}

func TestAuthorizePlanRevise_PermissionTable(t *testing.T) {
	principals := []domain.Principal{
		{Kind: domain.PrincipalKindUser, ID: "u-approver"},
		{Kind: domain.PrincipalKindTeam, ID: "team-sec"},
		{Kind: domain.PrincipalKindRole, ID: "qa"},
	}
	states := []struct {
		status domain.RequestStatus
	}{{domain.RequestStatusAwaitingPlanApproval}, {domain.RequestStatusPlanning}}
	for _, subject := range gateSubjectsForPlanRevise {
		for _, st := range states {
			for _, mayRevise := range []bool{true, false} {
				for _, a := range planReviseActors {
					name := string(subject) + "/" + string(st.status) + "/" + a.name
					if !mayRevise {
						name += "/approvers_off"
					}
					t.Run(name, func(t *testing.T) {
						f := newReviseFixture(subject, principals...)
						req := domain.Request{ID: "req", ReporterID: "u-reporter", Status: st.status, PlanTaskID: "plan-1"}
						err := planReviseAuthorizer(f, fakeRuns{newSolWorld()}, mayRevise).AuthorizePlanRevise(reviseActor(a.user, a.role), req)
						want := a.baseOnly || (a.approver && mayRevise)
						if want && err != nil {
							t.Fatalf("want allowed, got %v", err)
						}
						if !want {
							requireCode(t, err, "REQUEST_PLAN_FORBIDDEN")
						}
					})
				}
			}
		}
	}
}

func TestAuthorizePlanRevise_PendingApprovalOfAnalysisGateDoesNotCount(t *testing.T) {
	f := newReviseFixture(domain.SubjectSolution, domain.Principal{Kind: domain.PrincipalKindUser, ID: "u-approver"})
	req := domain.Request{ID: "req", ReporterID: "u-reporter", Status: domain.RequestStatusAwaitingPlanApproval, PlanTaskID: "plan-1"}
	err := planReviseAuthorizer(f, fakeRuns{newSolWorld()}, true).AuthorizePlanRevise(reviseActor("u-approver", "member"), req)
	requireCode(t, err, "REQUEST_PLAN_FORBIDDEN")
}

func TestAuthorizePlanRevise_InfraErrorNotMaskedAsForbidden(t *testing.T) {
	boom := errors.New("approvals database down")
	f := newReviseFixture(domain.SubjectPlan, domain.Principal{Kind: domain.PrincipalKindUser, ID: "u-approver"})
	f.pending.failFor[domain.SubjectPlan] = boom
	req := domain.Request{ID: "req", ReporterID: "u-reporter", Status: domain.RequestStatusAwaitingPlanApproval, PlanTaskID: "plan-1"}
	err := planReviseAuthorizer(f, fakeRuns{newSolWorld()}, true).AuthorizePlanRevise(reviseActor("u-approver", "member"), req)
	if !errors.Is(err, boom) {
		t.Fatalf("want the infrastructure error, got %v", err)
	}
}

func TestAuthorizePlanRevise_RetryOfOwnFailedRevision(t *testing.T) {
	failed := func(actor string, status domain.RunStatus, feedback string) domain.AnalysisRun {
		return domain.AnalysisRun{ID: "run-1", RequestID: "req", Kind: domain.RunKindPlan, Status: status, ActorID: actor, Feedback: feedback, StartedAt: time.Unix(100, 0)}
	}
	cases := []struct {
		name   string
		runs   []domain.AnalysisRun
		status domain.RequestStatus
		plan   string
		allow  bool
	}{
		{"own failed run with feedback", []domain.AnalysisRun{failed("u-approver", domain.RunStatusFailed, "chia nhỏ")}, domain.RequestStatusPlanning, "plan-1", true},
		{"another actor's failed run", []domain.AnalysisRun{failed("u-other", domain.RunStatusFailed, "chia nhỏ")}, domain.RequestStatusPlanning, "plan-1", false},
		{"own run still running (a repeated click)", []domain.AnalysisRun{failed("u-approver", domain.RunStatusRunning, "chia nhỏ")}, domain.RequestStatusPlanning, "plan-1", true},
		{"another actor's running run", []domain.AnalysisRun{failed("u-other", domain.RunStatusRunning, "chia nhỏ")}, domain.RequestStatusPlanning, "plan-1", false},
		{"own run succeeded", []domain.AnalysisRun{failed("u-approver", domain.RunStatusSucceeded, "chia nhỏ")}, domain.RequestStatusPlanning, "plan-1", false},
		{"own failed run without feedback", []domain.AnalysisRun{failed("u-approver", domain.RunStatusFailed, "")}, domain.RequestStatusPlanning, "plan-1", false},
		{"request not in planning", []domain.AnalysisRun{failed("u-approver", domain.RunStatusFailed, "x")}, domain.RequestStatusAwaitingPlanApproval, "plan-1", false},
		{"request without a plan", []domain.AnalysisRun{failed("u-approver", domain.RunStatusFailed, "x")}, domain.RequestStatusPlanning, "", false},
		{"no run at all", nil, domain.RequestStatusPlanning, "plan-1", false},
		{"a later run of another actor wins", []domain.AnalysisRun{
			failed("u-approver", domain.RunStatusFailed, "x"),
			{ID: "run-2", RequestID: "req", Kind: domain.RunKindPlan, Status: domain.RunStatusFailed, ActorID: "u-other", Feedback: "y", StartedAt: time.Unix(200, 0)},
		}, domain.RequestStatusPlanning, "plan-1", false},
		{"a newer solution run is not a plan run", []domain.AnalysisRun{
			failed("u-approver", domain.RunStatusFailed, "x"),
			{ID: "run-3", RequestID: "req", Kind: domain.RunKindSolution, Status: domain.RunStatusSucceeded, ActorID: "u-other", StartedAt: time.Unix(300, 0)},
		}, domain.RequestStatusPlanning, "plan-1", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newSolWorld()
			for _, r := range c.runs {
				w.runs[r.ID] = r
			}
			f := newReviseFixture(domain.SubjectPlan) // the Approval was cancelled with the revision: nobody is a pending approver
			f.pending.bySubject = map[domain.SubjectType][]domain.Approval{}
			req := domain.Request{ID: "req", ReporterID: "u-reporter", Status: c.status, PlanTaskID: c.plan}
			err := planReviseAuthorizer(f, fakeRuns{w}, true).AuthorizePlanRevise(reviseActor("u-approver", "member"), req)
			if c.allow && err != nil {
				t.Fatalf("want allowed, got %v", err)
			}
			if !c.allow {
				requireCode(t, err, "REQUEST_PLAN_FORBIDDEN")
			}
		})
	}
}

func TestAuthorizePlanRevise_RetryRuleIsOffWhenApproversMayNotRevise(t *testing.T) {
	w := newSolWorld()
	w.runs["run-1"] = domain.AnalysisRun{ID: "run-1", RequestID: "req", Kind: domain.RunKindPlan, Status: domain.RunStatusFailed, ActorID: "u-approver", Feedback: "x"}
	f := newReviseFixture(domain.SubjectPlan)
	f.pending.bySubject = map[domain.SubjectType][]domain.Approval{}
	req := domain.Request{ID: "req", ReporterID: "u-reporter", Status: domain.RequestStatusPlanning, PlanTaskID: "plan-1"}
	err := planReviseAuthorizer(f, fakeRuns{w}, false).AuthorizePlanRevise(reviseActor("u-approver", "member"), req)
	requireCode(t, err, "REQUEST_PLAN_FORBIDDEN")
}

func TestAuthorizePlan_StillReporterOrAdminOnly(t *testing.T) {
	f := newReviseFixture(domain.SubjectPlan, domain.Principal{Kind: domain.PrincipalKindUser, ID: "u-approver"})
	auth := planReviseAuthorizer(f, fakeRuns{newSolWorld()}, true)
	req := domain.Request{ID: "req", ReporterID: "u-reporter", Status: domain.RequestStatusAwaitingPlanApproval, PlanTaskID: "plan-1"}
	requireCode(t, auth.AuthorizePlan(reviseActor("u-approver", "member"), req), "REQUEST_PLAN_FORBIDDEN")
	if err := auth.AuthorizePlan(reviseActor("u-reporter", "member"), req); err != nil {
		t.Fatal(err)
	}
}

// --- GeneratePlan wiring ---

func planReviseKit(t *testing.T, status domain.RequestStatus, planTaskID string, auth PlanActorAuthorizer) (*planKit, domain.Request) {
	t.Helper()
	k := newPlanKit(nil)
	k.generate.auth = auth
	req := k.w.seedRequest(domain.RequestTypeChangeRequest, status)
	req.PlanTaskID = planTaskID
	k.w.requests[req.ID] = req
	k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)
	return k, req
}

func approverAuth(t *testing.T, k *planKit, req domain.Request) PlanActorAuthorizer {
	t.Helper()
	f := newReviseFixture(domain.SubjectPlan, domain.Principal{Kind: domain.PrincipalKindUser, ID: "u-approver"})
	ap := f.pending.bySubject[domain.SubjectPlan][0]
	ap.RequestID = req.ID
	f.pending.bySubject[domain.SubjectPlan] = []domain.Approval{ap}
	return NewPlanActorAuthorizer(f.authorizer(), fakeRuns{k.w}, true)
}

func TestGeneratePlan_Revise_ApproverNotReporter_Succeeds(t *testing.T) {
	k, req := planReviseKit(t, domain.RequestStatusAwaitingPlanApproval, "plan-1", nil)
	k.generate.auth = approverAuth(t, k, req)
	res, err := k.generate.Execute(reviseActor("u-approver", "member"), GeneratePlanInput{RequestID: req.ID, Feedback: "chia nhỏ migration"})
	if err != nil || !res.Created {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if k.w.runs[res.RunID].ActorID != "u-approver" {
		t.Fatalf("the run must be attributed to the approver, got %q", k.w.runs[res.RunID].ActorID)
	}
	if got := k.w.requests[req.ID].Status; got != domain.RequestStatusPlanning {
		t.Fatalf("status = %s, want planning", got)
	}
}

func TestGeneratePlan_Revise_ApproverStartingFirstPlan_Forbidden(t *testing.T) {
	for _, feedback := range []string{"", "có phản hồi"} {
		k, req := planReviseKit(t, domain.RequestStatusPlanning, "", nil)
		k.generate.auth = approverAuth(t, k, req)
		_, err := k.generate.Execute(reviseActor("u-approver", "member"), GeneratePlanInput{RequestID: req.ID, Feedback: feedback})
		requireCode(t, err, "REQUEST_PLAN_FORBIDDEN")
		if len(k.w.runs) != 0 {
			t.Fatalf("feedback=%q: an approver started the first Plan", feedback)
		}
	}
}

func TestGeneratePlan_Revise_StrangerForbiddenAndNothingChanges(t *testing.T) {
	k, req := planReviseKit(t, domain.RequestStatusAwaitingPlanApproval, "plan-1", nil)
	k.generate.auth = approverAuth(t, k, req)
	_, err := k.generate.Execute(reviseActor("u-stranger", "member"), GeneratePlanInput{RequestID: req.ID, Feedback: "x"})
	requireCode(t, err, "REQUEST_PLAN_FORBIDDEN")
	if len(k.w.runs) != 0 || len(k.w.cancelled) != 0 || k.w.requests[req.ID].Status != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("a refused revision changed data: runs=%d cancelled=%v status=%s", len(k.w.runs), k.w.cancelled, k.w.requests[req.ID].Status)
	}
}

type planOrderRecorder struct{ *orderRecorder }

func (o planOrderRecorder) AuthorizePlan(ctx context.Context, _ domain.Request) error {
	o.log = append(o.log, o.mark("authorize_plan", ctx))
	return nil
}

func (o planOrderRecorder) AuthorizePlanRevise(ctx context.Context, _ domain.Request) error {
	o.log = append(o.log, o.mark("authorize_revise_plan", ctx))
	return nil
}

func TestGeneratePlan_Revise_AuthorizeRunsBeforeCancelPending(t *testing.T) {
	k, req := planReviseKit(t, domain.RequestStatusAwaitingPlanApproval, "plan-1", nil)
	rec := &orderRecorder{w: k.w}
	k.generate.auth, k.generate.approvals = planOrderRecorder{rec}, rec
	if _, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID, Feedback: "chỉnh"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"authorize_revise_plan@outside", "cancel_pending@tx"}
	if len(rec.log) != len(want) || rec.log[0] != want[0] || rec.log[1] != want[1] {
		t.Fatalf("order = %v, want %v", rec.log, want)
	}
}

func TestGeneratePlan_FirstPlanUsesGenerateRule(t *testing.T) {
	k, req := planReviseKit(t, domain.RequestStatusPlanning, "", nil)
	rec := &orderRecorder{w: k.w}
	k.generate.auth, k.generate.approvals = planOrderRecorder{rec}, rec
	if _, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID}); err != nil {
		t.Fatal(err)
	}
	if len(rec.log) != 1 || rec.log[0] != "authorize_plan@outside" {
		t.Fatalf("order = %v, want a single authorize_plan", rec.log)
	}
}

func TestGeneratePlan_RetryInPlanningUsesReviseRule(t *testing.T) {
	k, req := planReviseKit(t, domain.RequestStatusPlanning, "plan-1", nil)
	rec := &orderRecorder{w: k.w}
	k.generate.auth, k.generate.approvals = planOrderRecorder{rec}, rec
	if _, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID}); err != nil {
		t.Fatal(err)
	}
	if len(rec.log) != 1 || rec.log[0] != "authorize_revise_plan@outside" {
		t.Fatalf("order = %v, want a single authorize_revise_plan", rec.log)
	}
}

func TestGeneratePlan_Revise_InfraErrorNotMaskedAsForbidden(t *testing.T) {
	boom := errors.New("approvals database down")
	k, req := planReviseKit(t, domain.RequestStatusAwaitingPlanApproval, "plan-1", nil)
	f := newReviseFixture(domain.SubjectPlan, domain.Principal{Kind: domain.PrincipalKindUser, ID: "u-approver"})
	f.pending.failFor[domain.SubjectPlan] = boom
	k.generate.auth = NewPlanActorAuthorizer(f.authorizer(), fakeRuns{k.w}, true)
	_, err := k.generate.Execute(reviseActor("u-approver", "member"), GeneratePlanInput{RequestID: req.ID, Feedback: "x"})
	if !errors.Is(err, boom) {
		t.Fatalf("want the infrastructure error, got %v", err)
	}
}
