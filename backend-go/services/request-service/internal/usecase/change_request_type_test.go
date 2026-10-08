package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

var errFake = errors.New("fake failure")

type changeFixture struct {
	s    *memStore
	appr *recordingApprovals
	uc   *ChangeRequestType
}

func newChangeFixture() *changeFixture {
	s := newMemStore()
	a := &recordingApprovals{}
	return &changeFixture{s: s, appr: a, uc: NewChangeRequestType(s, historyView{s}, &memTransitioner{s: s}, a, a, s, s)}
}

func (f *changeFixture) in(t *testing.T, status domain.RequestStatus, typ domain.RequestType) domain.Request {
	return f.s.seed(t, func(r *domain.Request) { r.Status, r.Type, r.TypeSource = status, typ, domain.TypeSourceHuman })
}

func changeIn(id, to string) ChangeInput {
	return ChangeInput{RequestID: id, NewType: to, Reason: "scope grew", ActorID: testReporter, ActorKind: domain.ActorKindUser}
}

func TestChange_BugToChangeRequest_FromAnalyzing_KeepsSolutionsAndTasks(t *testing.T) {
	f := newChangeFixture()
	r := f.in(t, domain.RequestStatusAnalyzing, domain.RequestTypeBug)
	out, err := f.uc.Execute(tctx(), changeIn(r.ID, "change_request"))
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != domain.RequestStatusAwaitingTypeConfirmation || out.Type != domain.RequestTypeChangeRequest {
		t.Fatalf("%+v", out)
	}
	if len(f.s.history) != 1 || f.s.history[0].FromType != domain.RequestTypeBug || f.s.history[0].Reason != "scope grew" {
		t.Fatalf("%+v", f.s.history)
	}
	// Only the Request row and history change: this use case has no handle on solutions, plans or tasks.
	found := false
	for _, s := range f.s.subjects() {
		found = found || s == domain.SubjectRequestTypeChanged
	}
	if !found || len(f.appr.cancelled) != 1 || f.appr.cancelled[0] != "type_changed" {
		t.Fatalf("events=%v cancelled=%v", f.s.subjects(), f.appr.cancelled)
	}
}

func TestChange_Rejections(t *testing.T) {
	cases := []struct {
		name   string
		status domain.RequestStatus
		from   domain.RequestType
		to     string
		want   string
	}{
		{"spike use child", domain.RequestStatusAnalyzing, domain.RequestTypeSpike, "change_request", "REQUEST_TYPE_CHANGE_USE_CHILD"},
		{"hotfix use child", domain.RequestStatusAnalyzing, domain.RequestTypeHotfix, "bug", "REQUEST_TYPE_CHANGE_USE_CHILD"},
		{"bug to docs", domain.RequestStatusAnalyzing, domain.RequestTypeBug, "docs", "REQUEST_TYPE_CHANGE_NOT_ALLOWED"},
		{"same type", domain.RequestStatusAnalyzing, domain.RequestTypeBug, "bug", "REQUEST_TYPE_UNCHANGED"},
		{"from awaiting confirmation", domain.RequestStatusAwaitingTypeConfirmation, domain.RequestTypeBug, "change_request", "REQUEST_TRANSITION_NOT_ALLOWED"},
		{"from new", domain.RequestStatusNew, domain.RequestTypeBug, "change_request", "REQUEST_TRANSITION_NOT_ALLOWED"},
		{"from completed", domain.RequestStatusCompleted, domain.RequestTypeBug, "change_request", "REQUEST_TRANSITION_NOT_ALLOWED"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newChangeFixture()
			r := f.in(t, c.status, c.from)
			_, err := f.uc.Execute(tctx(), changeIn(r.ID, c.to))
			mustCode(t, err, c.want)
			if len(f.s.history) != 0 || len(f.s.events) != 0 {
				t.Fatal("rejected change must write nothing")
			}
		})
	}
}

func TestChange_SecurityToHotfix_Allowed(t *testing.T) {
	f := newChangeFixture()
	r := f.in(t, domain.RequestStatusPlanning, domain.RequestTypeSecurity)
	if _, err := f.uc.Execute(tctx(), changeIn(r.ID, "hotfix")); err != nil {
		t.Fatal(err)
	}
}

func TestChange_ReasonRequired_StaleVersion_NonUser(t *testing.T) {
	f := newChangeFixture()
	r := f.in(t, domain.RequestStatusAnalyzing, domain.RequestTypeBug)
	in := changeIn(r.ID, "change_request")
	in.Reason = "  "
	_, err := f.uc.Execute(tctx(), in)
	mustCode(t, err, "REQUEST_REASON_REQUIRED")
	in = changeIn(r.ID, "change_request")
	in.ExpectedVersion = r.Version + 3
	_, err = f.uc.Execute(tctx(), in)
	mustCode(t, err, "REQUEST_VERSION_CONFLICT")
	in = changeIn(r.ID, "change_request")
	in.ActorKind = domain.ActorKindSystem
	_, err = f.uc.Execute(tctx(), in)
	mustCode(t, err, "REQUEST_ACTOR_NOT_ALLOWED")
}

func TestChange_ExecutingWithActiveExecution_Blocked(t *testing.T) {
	f := newChangeFixture()
	f.appr.active = true
	r := f.in(t, domain.RequestStatusExecuting, domain.RequestTypeBug)
	_, err := f.uc.Execute(tctx(), changeIn(r.ID, "change_request"))
	mustCode(t, err, "REQUEST_TYPE_CHANGE_BLOCKED_ACTIVE_EXECUTION")
	f.appr.active = false
	if _, err := f.uc.Execute(tctx(), changeIn(r.ID, "change_request")); err != nil {
		t.Fatalf("executing without active execution may change: %v", err)
	}
}

func TestListTypeHistory_OrderAfterAIUserChange(t *testing.T) {
	s := newMemStore()
	hist := historyView{s}
	r := s.seed(t, func(r *domain.Request) { r.Status = domain.RequestStatusAnalyzing })
	base := time.Now().UTC()
	// Appended out of order on purpose: the result must follow `at`.
	_ = hist.Append(tctx(), domain.RequestTypeChange{RequestID: r.ID, ToType: domain.RequestTypeChangeRequest, FromType: domain.RequestTypeBug, ActorKind: domain.ActorKindUser, ActorID: testReporter, At: base.Add(2 * time.Second)})
	_ = hist.Append(tctx(), domain.RequestTypeChange{RequestID: r.ID, ToType: domain.RequestTypeTask, ActorKind: domain.ActorKindAgent, ActorID: testReporter, At: base})
	_ = hist.Append(tctx(), domain.RequestTypeChange{RequestID: r.ID, ToType: domain.RequestTypeBug, FromType: domain.RequestTypeTask, ActorKind: domain.ActorKindUser, ActorID: testReporter, At: base.Add(time.Second)})
	got, err := NewListRequestTypeHistory(s, hist).Execute(tctx(), r.ID)
	if err != nil || len(got) != 3 || got[0].ToType != domain.RequestTypeTask || got[1].ToType != domain.RequestTypeBug || got[2].ToType != domain.RequestTypeChangeRequest {
		t.Fatalf("%+v %v", got, err)
	}
	_, err = NewListRequestTypeHistory(s, hist).Execute(tctx(), "00000000-0000-0000-0000-000000000001")
	mustCode(t, err, "REQUEST_NOT_FOUND")
}
