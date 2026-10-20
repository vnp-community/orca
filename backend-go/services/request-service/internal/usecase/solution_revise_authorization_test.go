package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// revisePending is an ApprovalRepository that lists pending Approvals per subject type and can fail on demand.
type revisePending struct {
	ApprovalRepository
	bySubject map[domain.SubjectType][]domain.Approval
	failFor   map[domain.SubjectType]error
	listed    []domain.SubjectType
}

func (r *revisePending) List(_ context.Context, _ string, f ApprovalListFilter) ([]domain.Approval, string, error) {
	r.listed = append(r.listed, f.SubjectType)
	if err := r.failFor[f.SubjectType]; err != nil {
		return nil, "", err
	}
	if f.Status != domain.ApprovalStatusPending {
		return nil, "", nil
	}
	return r.bySubject[f.SubjectType], "", nil
}

type reviseApprovers struct {
	byApproval map[string][]domain.Principal
	err        error
}

func (r reviseApprovers) InsertSnapshot(context.Context, string, string, []domain.Principal) error {
	return nil
}

func (r reviseApprovers) ListForApproval(_ context.Context, _, id string) ([]domain.Principal, error) {
	return r.byApproval[id], r.err
}

type reviseTeams struct {
	teamsOf map[string][]string
	err     error
}

func (r reviseTeams) TeamsForUser(_ context.Context, u string) ([]string, error) {
	return r.teamsOf[u], r.err
}
func (r reviseTeams) MembersOfTeam(context.Context, string) ([]string, error) { return nil, nil }

// reviseFixture wires the real decision check (AuthorizeApprovalDecision) over in-memory snapshots.
type reviseFixture struct {
	pending   *revisePending
	approvers reviseApprovers
	teams     reviseTeams
}

func newReviseFixture(subject domain.SubjectType, principals ...domain.Principal) *reviseFixture {
	ap := domain.Approval{ID: "ap-1", RequestID: "req", SubjectType: subject, Status: domain.ApprovalStatusPending, RequestedBy: "system"}
	return &reviseFixture{
		pending:   &revisePending{bySubject: map[domain.SubjectType][]domain.Approval{subject: {ap}}, failFor: map[domain.SubjectType]error{}},
		approvers: reviseApprovers{byApproval: map[string][]domain.Principal{"ap-1": principals}},
		teams:     reviseTeams{teamsOf: map[string][]string{"u-team": {"team-sec"}}},
	}
}

func (f *reviseFixture) authorizer() ApproverAwareAuthorizer {
	return ApproverAwareAuthorizer{
		Approvals: f.pending,
		Decider:   &AuthorizeApprovalDecision{ApproverRepo: f.approvers, Teams: f.teams},
	}
}

func reviseActor(user, role string) context.Context {
	ctx := tenant.WithUserID(lcCtx(), user)
	if role != "" {
		ctx = tenant.WithRole(ctx, role)
	}
	return ctx
}

var gateSubjectsForRevise = []domain.SubjectType{domain.SubjectSolution, domain.SubjectFindings, domain.SubjectAnswer}

func TestAuthorizeRevise_PermissionTable(t *testing.T) {
	principals := []domain.Principal{
		{Kind: domain.PrincipalKindUser, ID: "u-approver"},
		{Kind: domain.PrincipalKindTeam, ID: "team-sec"},
		{Kind: domain.PrincipalKindRole, ID: "qa"},
	}
	actors := []struct {
		name       string
		user, role string
		allowed    bool
	}{
		{"reporter", "u-reporter", "member", true},
		{"admin", "u-admin", "admin", true},
		{"lead", "u-lead", "lead", true},
		{"owner", "u-owner", "owner", true},
		{"approver by user", "u-approver", "member", true},
		{"approver by team", "u-team", "member", true},
		{"approver by role", "u-qa", "qa", true},
		{"read-only member", "u-viewer", "viewer", false},
		{"stranger", "u-stranger", "member", false},
	}
	for _, subject := range gateSubjectsForRevise {
		for _, a := range actors {
			t.Run(string(subject)+"/"+a.name, func(t *testing.T) {
				f := newReviseFixture(subject, principals...)
				req := domain.Request{ID: "req", ReporterID: "u-reporter"}
				err := f.authorizer().AuthorizeRevise(reviseActor(a.user, a.role), req)
				if a.allowed && err != nil {
					t.Fatalf("want allowed, got %v", err)
				}
				if !a.allowed {
					requireCode(t, err, "REQUEST_SOLUTION_FORBIDDEN")
				}
			})
		}
	}
}

func TestAuthorizeRevise_PendingApprovalOfAnotherSubjectDoesNotCount(t *testing.T) {
	f := newReviseFixture(domain.SubjectPlan, domain.Principal{Kind: domain.PrincipalKindUser, ID: "u-approver"})
	err := f.authorizer().AuthorizeRevise(reviseActor("u-approver", "member"), domain.Request{ID: "req", ReporterID: "u-reporter"})
	requireCode(t, err, "REQUEST_SOLUTION_FORBIDDEN")
}

func TestAuthorizeRevise_NoPendingApprovalMeansNoApprover(t *testing.T) {
	f := newReviseFixture(domain.SubjectSolution, domain.Principal{Kind: domain.PrincipalKindUser, ID: "u-approver"})
	f.pending.bySubject = nil
	err := f.authorizer().AuthorizeRevise(reviseActor("u-approver", "member"), domain.Request{ID: "req", ReporterID: "u-reporter"})
	requireCode(t, err, "REQUEST_SOLUTION_FORBIDDEN")
}

func TestAuthorizeRevise_ApproverMayNotStartAnalysis(t *testing.T) {
	f := newReviseFixture(domain.SubjectSolution, domain.Principal{Kind: domain.PrincipalKindUser, ID: "u-approver"})
	err := f.authorizer().AuthorizeGenerate(reviseActor("u-approver", "member"), domain.Request{ID: "req", ReporterID: "u-reporter"})
	requireCode(t, err, "REQUEST_SOLUTION_FORBIDDEN")
	if len(f.pending.listed) != 0 {
		t.Fatalf("AuthorizeGenerate must not consult approvals, listed %v", f.pending.listed)
	}
}

func TestAuthorizeRevise_BaseCallerSkipsApprovalLookup(t *testing.T) {
	f := newReviseFixture(domain.SubjectSolution)
	if err := f.authorizer().AuthorizeRevise(reviseActor("u-reporter", "member"), domain.Request{ID: "req", ReporterID: "u-reporter"}); err != nil {
		t.Fatal(err)
	}
	if len(f.pending.listed) != 0 {
		t.Fatalf("the reporter needs no approval lookup, listed %v", f.pending.listed)
	}
}

func TestAuthorizeRevise_InfrastructureErrorsAreNotForbidden(t *testing.T) {
	boom := errors.New("database down")
	approverUser := domain.Principal{Kind: domain.PrincipalKindUser, ID: "u-approver"}
	stranger := reviseActor("u-stranger", "member")
	cases := []struct {
		name  string
		setup func(f *reviseFixture)
		ctx   context.Context
		want  func(error) bool
	}{
		{"listing solution approvals", func(f *reviseFixture) { f.pending.failFor[domain.SubjectSolution] = boom }, stranger, func(e error) bool { return errors.Is(e, boom) }},
		{"listing findings approvals after an empty solution list", func(f *reviseFixture) { f.pending.failFor[domain.SubjectFindings] = boom }, stranger, func(e error) bool { return errors.Is(e, boom) }},
		{"listing answer approvals", func(f *reviseFixture) { f.pending.failFor[domain.SubjectAnswer] = boom }, stranger, func(e error) bool { return errors.Is(e, boom) }},
		{"reading the approver snapshot", func(f *reviseFixture) { f.approvers.err = boom }, reviseActor("u-approver", "member"), func(e error) bool { return errors.Is(e, boom) }},
		{"team directory outage", func(f *reviseFixture) { f.teams.err = boom }, reviseActor("u-team", "member"), func(e error) bool { return errors.Is(e, boom) }},
		{"team directory unavailable error", func(f *reviseFixture) { f.teams.err = domain.ErrApprovalDirectoryUnavailable }, reviseActor("u-team", "member"), func(e error) bool { return errorHasCode(e, "REQUEST_APPROVAL_DIRECTORY_UNAVAILABLE") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newReviseFixture(domain.SubjectSolution, approverUser, domain.Principal{Kind: domain.PrincipalKindTeam, ID: "team-sec"})
			tc.setup(f)
			err := f.authorizer().AuthorizeRevise(tc.ctx, domain.Request{ID: "req", ReporterID: "u-reporter"})
			if err == nil || errorHasCode(err, "REQUEST_SOLUTION_FORBIDDEN") || !tc.want(err) {
				t.Fatalf("infrastructure failure must surface unchanged, got %v", err)
			}
		})
	}
}

func TestNewApproverAwareAuthorizer_RejectsMissingDependencies(t *testing.T) {
	f := newReviseFixture(domain.SubjectSolution)
	decider := &AuthorizeApprovalDecision{ApproverRepo: f.approvers, Teams: f.teams}
	if _, err := NewApproverAwareAuthorizer(nil, decider); err == nil {
		t.Fatal("a missing approval repository must fail startup")
	}
	if _, err := NewApproverAwareAuthorizer(f.pending, nil); err == nil {
		t.Fatal("a missing decision authorizer must fail startup")
	}
	if _, err := NewApproverAwareAuthorizer(f.pending, decider); err != nil {
		t.Fatal(err)
	}
}

func TestReporterOrAdminAuthorizer_ReviseEqualsGenerate(t *testing.T) {
	w := newSolWorld()
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval)
	a := ReporterOrAdminAuthorizer{}
	requireCode(t, a.AuthorizeRevise(w.ctx(), r), "REQUEST_SOLUTION_FORBIDDEN")
	if err := a.AuthorizeRevise(tenant.WithUserID(lcCtx(), r.ReporterID), r); err != nil {
		t.Fatal(err)
	}
	if err := a.AuthorizeRevise(reviseActor("someone", "lead"), r); err != nil {
		t.Fatal(err)
	}
}

// --- GenerateSolution.prepare wiring ---

// reviseWorld seeds a Request of the given type and a pending Approval of its gate subject for u-approver.
func reviseWorld(t *testing.T, typ domain.RequestType, status domain.RequestStatus) (*solWorld, domain.Request, *reviseFixture) {
	t.Helper()
	w := newSolWorld()
	r := w.seedRequest(typ, status)
	flow, err := domain.FlowFor(typ)
	if err != nil {
		t.Fatal(err)
	}
	f := newReviseFixture(domain.SubjectType(flow.AnalysisGate), domain.Principal{Kind: domain.PrincipalKindUser, ID: "u-approver"})
	ap := f.pending.bySubject[domain.SubjectType(flow.AnalysisGate)][0]
	ap.RequestID = r.ID
	f.pending.bySubject[domain.SubjectType(flow.AnalysisGate)] = []domain.Approval{ap}
	return w, r, f
}

func newReviseGenerate(w *solWorld, auth SolutionActorAuthorizer) *GenerateSolution {
	uc := w.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/repo"}})
	uc.auth = auth
	return uc
}

func TestPrepare_Revise_ApproverNotReporter_Succeeds(t *testing.T) {
	for _, typ := range []domain.RequestType{domain.RequestTypeChangeRequest, domain.RequestTypeSpike, domain.RequestTypeQuestion} {
		t.Run(string(typ), func(t *testing.T) {
			w, r, f := reviseWorld(t, typ, domain.RequestStatusAwaitingAnalysisApproval)
			uc := newReviseGenerate(w, f.authorizer())
			res, err := uc.Execute(reviseActor("u-approver", "member"), GenerateSolutionInput{RequestID: r.ID, Feedback: "đổi phương án"})
			if err != nil || !res.Created {
				t.Fatalf("res=%+v err=%v", res, err)
			}
			if len(w.cancelled) != 1 || w.cancelled[0] != r.ID+":revision_requested" {
				t.Fatalf("cancelled = %v", w.cancelled)
			}
			if got := w.requests[r.ID].Status; got != domain.RequestStatusAnalyzing {
				t.Fatalf("request = %s, want analyzing", got)
			}
			if w.runs[res.RunID].ActorID != "u-approver" {
				t.Fatalf("run must be attributed to the approver, got %q", w.runs[res.RunID].ActorID)
			}
		})
	}
}

func TestPrepare_Revise_StrangerForbiddenAndNothingChanges(t *testing.T) {
	w, r, f := reviseWorld(t, domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval)
	uc := newReviseGenerate(w, f.authorizer())
	_, err := uc.Execute(reviseActor("u-stranger", "member"), GenerateSolutionInput{RequestID: r.ID, Feedback: "x"})
	requireCode(t, err, "REQUEST_SOLUTION_FORBIDDEN")
	if len(w.runs) != 0 || len(w.solutions) != 0 || len(w.cancelled) != 0 || w.requests[r.ID].Status != domain.RequestStatusAwaitingAnalysisApproval {
		t.Fatalf("a refused revision changed data: runs=%d solutions=%d cancelled=%v status=%s", len(w.runs), len(w.solutions), w.cancelled, w.requests[r.ID].Status)
	}
}

func TestPrepare_Revise_ApproverStartingFirstAnalysis_Forbidden(t *testing.T) {
	for _, feedback := range []string{"", "có phản hồi"} {
		w, r, f := reviseWorld(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
		uc := newReviseGenerate(w, f.authorizer())
		_, err := uc.Execute(reviseActor("u-approver", "member"), GenerateSolutionInput{RequestID: r.ID, Feedback: feedback})
		requireCode(t, err, "REQUEST_SOLUTION_FORBIDDEN")
		if len(w.runs) != 0 {
			t.Fatalf("feedback=%q: an approver started an analysis", feedback)
		}
	}
}

func TestPrepare_Revise_ApproverWithoutFeedbackOnPendingGate_Forbidden(t *testing.T) {
	w, r, f := reviseWorld(t, domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval)
	uc := newReviseGenerate(w, f.authorizer())
	_, err := uc.Execute(reviseActor("u-approver", "member"), GenerateSolutionInput{RequestID: r.ID})
	requireCode(t, err, "REQUEST_SOLUTION_FORBIDDEN")
}

func TestPrepare_Revise_ReporterStillWorksWithoutApprovalLookup(t *testing.T) {
	w, r, f := reviseWorld(t, domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval)
	uc := newReviseGenerate(w, f.authorizer())
	if _, err := uc.Execute(reviseActor(r.ReporterID, "member"), GenerateSolutionInput{RequestID: r.ID, Feedback: "x"}); err != nil {
		t.Fatal(err)
	}
}

// orderRecorder logs the authorizer and the approval cancel on one timeline, noting whether a transaction was open.
type orderRecorder struct {
	w   *solWorld
	log []string
}

func (o *orderRecorder) AuthorizeGenerate(ctx context.Context, _ domain.Request) error {
	o.log = append(o.log, o.mark("authorize_generate", ctx))
	return nil
}
func (o *orderRecorder) AuthorizeChoose(context.Context, domain.Request) error { return nil }
func (o *orderRecorder) AuthorizeRevise(ctx context.Context, _ domain.Request) error {
	o.log = append(o.log, o.mark("authorize_revise", ctx))
	return nil
}

func (o *orderRecorder) AuthorizeRevisePlan(context.Context, domain.Request) error { return nil }

func (o *orderRecorder) CancelPending(ctx context.Context, requestID, why string) error {
	o.log = append(o.log, o.mark("cancel_pending", ctx))
	return fakeCanceller{o.w}.CancelPending(ctx, requestID, why)
}

func (o *orderRecorder) mark(step string, ctx context.Context) string {
	if o.w.InTransaction(ctx) {
		return step + "@tx"
	}
	return step + "@outside"
}

func TestPrepare_Revise_AuthorizeRunsBeforeCancelPending(t *testing.T) {
	w := newSolWorld()
	rec := &orderRecorder{w: w}
	uc := w.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	uc.auth, uc.approvals = rec, rec
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval)

	if _, err := uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID, Feedback: "chỉnh"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"authorize_revise@outside", "cancel_pending@tx"}
	if len(rec.log) != len(want) || rec.log[0] != want[0] || rec.log[1] != want[1] {
		t.Fatalf("order = %v, want %v", rec.log, want)
	}
}

func TestPrepare_Revise_FirstAnalysisUsesGenerateRule(t *testing.T) {
	w := newSolWorld()
	rec := &orderRecorder{w: w}
	uc := w.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	uc.auth, uc.approvals = rec, rec
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)

	if _, err := uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID, Feedback: "chạy lại"}); err != nil {
		t.Fatal(err)
	}
	if len(rec.log) != 1 || rec.log[0] != "authorize_generate@outside" {
		t.Fatalf("order = %v, want a single authorize_generate", rec.log)
	}
}

func TestPrepare_Revise_InfraErrorNotMaskedAsForbidden(t *testing.T) {
	boom := errors.New("approvals database down")
	w, r, f := reviseWorld(t, domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval)
	f.pending.failFor[domain.SubjectSolution] = boom
	uc := newReviseGenerate(w, f.authorizer())
	_, err := uc.Execute(reviseActor("u-approver", "member"), GenerateSolutionInput{RequestID: r.ID, Feedback: "x"})
	if !errors.Is(err, boom) || errorHasCode(err, "REQUEST_SOLUTION_FORBIDDEN") {
		t.Fatalf("want the infrastructure error, got %v", err)
	}
	if len(w.runs) != 0 || len(w.cancelled) != 0 {
		t.Fatalf("a failed permission check must not change data: runs=%d cancelled=%v", len(w.runs), w.cancelled)
	}
}

func TestPrepare_System_SkipsAuthorization(t *testing.T) {
	w := newSolWorld()
	rec := &orderRecorder{w: w}
	uc := w.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	uc.auth, uc.approvals = rec, rec
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	if _, err := uc.ExecuteForSystem(lcCtx(), GenerateSolutionInput{RequestID: r.ID}); err != nil {
		t.Fatal(err)
	}
	if len(rec.log) != 0 {
		t.Fatalf("system calls are not authorised: %v", rec.log)
	}
}
