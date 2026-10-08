package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	userAda   = "11111111-1111-1111-1111-111111111111"
	userBob   = "22222222-2222-2222-2222-222222222222"
	adminRoot = "33333333-3333-3333-3333-333333333333"
)

func asUser(id string) context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), id)
}

func asAdmin() context.Context { return tenant.WithRole(asUser(adminRoot), "admin") }

type artFx struct {
	env       *artEnv
	tr        *TransitionRequest
	appendRev *AppendRequestRevision
	edit      *EditRequestContent
}

func newArtFx() *artFx {
	env := newArtEnv()
	tr := NewTransitionRequest(env.s, env, env.s)
	ap := NewAppendRequestRevision(env.s, env, revisionsView{env}, env, env.s)
	return &artFx{env: env, tr: tr, appendRev: ap, edit: NewEditRequestContent(env.s, ap, env)}
}

// seed stores a request with valid content columns (revision 1 and its row), as creation would leave it.
func (f *artFx) seed(mod func(r *domain.Request)) domain.Request {
	r := f.env.s.seed(func(r *domain.Request) {
		r.ReporterID = userAda
		r.Status = domain.RequestStatusAwaitingTypeConfirmation
		r.Type = domain.RequestTypeBug
		r.Title, r.Body = "Lỗi đăng nhập", "Đăng nhập thất bại khi mật khẩu có dấu cách ở cuối chuỗi."
		if mod != nil {
			mod(r)
		}
	})
	c, _ := domain.ContentFromRequest(r)
	stored, _ := r.WithContent(c)
	stored.ContentRevision, stored.ContentSchemaVersion = 1, 1
	f.env.s.requests[r.ID] = stored
	snap, _ := c.Snapshot(nil)
	f.env.revisions[r.ID] = []domain.RequestRevision{{ID: newID(), RequestID: r.ID, Revision: 1, Cause: domain.RevisionCauseCreated, Snapshot: snap, Digest: stored.ContentDigest}}
	return stored
}

func (f *artFx) eventsOf(subject string) []domain.OutboxEvent {
	var out []domain.OutboxEvent
	for _, e := range f.env.s.events {
		if e.Subject == subject {
			out = append(out, e)
		}
	}
	return out
}

func contentOf(t *testing.T, r domain.Request) domain.RequestContent {
	t.Helper()
	c, err := domain.ContentFromRequest(r)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAppendRequestRevision_IncrementsOnce_WritesRowAndEvent(t *testing.T) {
	f := newArtFx()
	r := f.seed(nil)
	c := contentOf(t, r)
	c.Title = "Lỗi đăng nhập (đã sửa)"
	res, err := f.appendRev.Execute(asUser(userAda), AppendInput{RequestID: r.ID, Content: c, Cause: domain.RevisionCauseEdited, ActorID: userAda, ActorKind: domain.ActorKindUser, ExpectedVersion: r.Version})
	if err != nil || !res.Applied {
		t.Fatalf("%+v %v", res, err)
	}
	got := f.env.s.requests[r.ID]
	if got.ContentRevision != 2 || got.Title != "Lỗi đăng nhập (đã sửa)" || got.Version != r.Version+1 || got.ContentDigest == r.ContentDigest {
		t.Fatalf("request after append: %+v", got)
	}
	revs := f.env.revisions[r.ID]
	if len(revs) != 2 || revs[1].Revision != 2 || revs[1].Cause != domain.RevisionCauseEdited || revs[1].ActorKind != domain.RevisionActorUser || revs[1].Digest != got.ContentDigest {
		t.Fatalf("revisions: %+v", revs)
	}
	if ev := f.eventsOf(domain.SubjectRequestRevised); len(ev) != 1 {
		t.Fatalf("want one revised event, got %d", len(ev))
	}
}

func TestAppendRequestRevision_PayloadHasNoContent(t *testing.T) {
	f := newArtFx()
	r := f.seed(nil)
	c := contentOf(t, r)
	c.Body = "BÍ MẬT-CỦA-NGƯỜI-DÙNG " + c.Body
	if _, err := f.appendRev.Execute(asUser(userAda), AppendInput{RequestID: r.ID, Content: c, Cause: domain.RevisionCauseEdited}); err != nil {
		t.Fatal(err)
	}
	ev := f.eventsOf(domain.SubjectRequestRevised)[0]
	if strings.Contains(string(ev.Payload), "BÍ MẬT") || strings.Contains(string(ev.Payload), r.Title) {
		t.Fatalf("payload leaks content: %s", ev.Payload)
	}
	var p RevisedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil || p.RequestID != r.ID || p.Revision != 2 || p.Cause != "edited" || len(p.Digest) != 64 {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestAppendRequestRevision_NoOpWhenDigestSame(t *testing.T) {
	f := newArtFx()
	r := f.seed(nil)
	res, err := f.appendRev.Execute(asUser(userAda), AppendInput{RequestID: r.ID, Content: contentOf(t, r), Cause: domain.RevisionCauseEdited})
	if err != nil || res.Applied {
		t.Fatalf("same content must be a no-op: %+v %v", res, err)
	}
	if len(f.env.revisions[r.ID]) != 1 || len(f.eventsOf(domain.SubjectRequestRevised)) != 0 || f.env.s.requests[r.ID].Version != r.Version {
		t.Fatal("a no-op wrote something")
	}
}

func TestAppendRequestRevision_StaleVersion(t *testing.T) {
	f := newArtFx()
	r := f.seed(nil)
	c := contentOf(t, r)
	c.Title = "x"
	_, err := f.appendRev.Execute(asUser(userAda), AppendInput{RequestID: r.ID, Content: c, Cause: domain.RevisionCauseEdited, ExpectedVersion: r.Version + 5})
	mustCode(t, err, "REQUEST_VERSION_CONFLICT")
	if len(f.env.revisions[r.ID]) != 1 {
		t.Fatal("a refused append left a revision")
	}
}

func TestAppendRequestRevision_ConcurrentCallsOneWins(t *testing.T) {
	// Two writers read the same version; the second CAS must lose and leave exactly one new revision and event.
	f := newArtFx()
	r := f.seed(nil)
	a, b := contentOf(t, r), contentOf(t, r)
	a.Title, b.Title = "viết bởi A", "viết bởi B"
	in := func(c domain.RequestContent) AppendInput {
		return AppendInput{RequestID: r.ID, Content: c, Cause: domain.RevisionCauseEdited, ExpectedVersion: r.Version}
	}
	if _, err := f.appendRev.Execute(asUser(userAda), in(a)); err != nil {
		t.Fatal(err)
	}
	_, err := f.appendRev.Execute(asUser(userBob), in(b))
	mustCode(t, err, "REQUEST_VERSION_CONFLICT")
	if len(f.env.revisions[r.ID]) != 2 || len(f.eventsOf(domain.SubjectRequestRevised)) != 1 || f.env.s.requests[r.ID].Title != "viết bởi A" {
		t.Fatalf("revisions=%d events=%d title=%q", len(f.env.revisions[r.ID]), len(f.eventsOf(domain.SubjectRequestRevised)), f.env.s.requests[r.ID].Title)
	}
}

func TestAppendRequestRevision_RollsBackWhenRevisionRowFails(t *testing.T) {
	f := newArtFx()
	r := f.seed(nil)
	c := contentOf(t, r)
	c.Title = "đổi"
	f.env.failRevAdd = errBoom
	if _, err := f.appendRev.Execute(asUser(userAda), AppendInput{RequestID: r.ID, Content: c, Cause: domain.RevisionCauseEdited}); err == nil {
		t.Fatal("expected failure")
	}
	if f.env.s.requests[r.ID].ContentRevision != 1 || f.env.s.requests[r.ID].Title != r.Title || len(f.eventsOf(domain.SubjectRequestRevised)) != 0 {
		t.Fatal("the content update must roll back with the failed revision row")
	}
}

func TestAppendRequestRevision_RejectsInvalidDraftContent(t *testing.T) {
	f := newArtFx()
	r := f.seed(nil)
	c := contentOf(t, r)
	c.Title = ""
	_, err := f.appendRev.Execute(asUser(userAda), AppendInput{RequestID: r.ID, Content: c, Cause: domain.RevisionCauseEdited})
	mustCode(t, err, domain.CodeArtifactSchemaInvalid)
	_, err = f.appendRev.Execute(asUser(userAda), AppendInput{RequestID: r.ID, Content: contentOf(t, r), Cause: "magic"})
	mustCode(t, err, "REQUEST_REVISION_CAUSE_INVALID")
}

func TestAppendRequestRevision_ClarificationAnsweredAlwaysWritesRevision(t *testing.T) {
	f := newArtFx()
	r := f.seed(nil)
	res, err := f.appendRev.Execute(asUser(userAda), AppendInput{RequestID: r.ID, Content: contentOf(t, r), Cause: domain.RevisionCauseClarificationAnswered, ClarificationID: newID()})
	if err != nil || !res.Applied || res.Request.ContentRevision != 2 {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestEditRequestContent_OnlyBeforeAnalysis(t *testing.T) {
	for st, editable := range map[domain.RequestStatus]bool{
		domain.RequestStatusNew: true, domain.RequestStatusClassifying: true, domain.RequestStatusAwaitingTypeConfirmation: true, domain.RequestStatusRequestBacklog: true,
		domain.RequestStatusAnalyzing: false, domain.RequestStatusAwaitingAnalysisApproval: false, domain.RequestStatusPlanning: false,
		domain.RequestStatusAwaitingPlanApproval: false, domain.RequestStatusExecuting: false, domain.RequestStatusCompleted: false,
		domain.RequestStatusCancelled: false, domain.RequestStatusAwaitingInformation: false,
	} {
		f := newArtFx()
		r := f.seed(func(r *domain.Request) {
			r.Status = st
			if st == domain.RequestStatusRequestBacklog {
				r.ReturnedFromStage, r.ReturnedCategory = domain.ReturnStageAnalysis, domain.ReturnCategoryOther
			}
		})
		title := "Tiêu đề mới"
		_, err := f.edit.Execute(asUser(userAda), EditInput{RequestID: r.ID, ExpectedRevision: 1, Patch: domain.ContentPatch{Title: &title}})
		if editable && err != nil {
			t.Errorf("%s: %v", st, err)
		}
		if !editable {
			mustCode(t, err, "REQUEST_CONTENT_NOT_EDITABLE")
		}
	}
}

func TestEditRequestContent_StaleRevision(t *testing.T) {
	f := newArtFx()
	r := f.seed(nil)
	title := "a"
	if _, err := f.edit.Execute(asUser(userAda), EditInput{RequestID: r.ID, ExpectedRevision: 1, Patch: domain.ContentPatch{Title: &title}}); err != nil {
		t.Fatal(err)
	}
	title2 := "b"
	_, err := f.edit.Execute(asUser(userAda), EditInput{RequestID: r.ID, ExpectedRevision: 1, Patch: domain.ContentPatch{Title: &title2}})
	mustCode(t, err, "REQUEST_VERSION_CONFLICT")
}

func TestEditRequestContent_WhoMayEdit(t *testing.T) {
	f := newArtFx()
	r := f.seed(nil)
	title := "x"
	_, err := f.edit.Execute(asUser(userBob), EditInput{RequestID: r.ID, ExpectedRevision: 1, Patch: domain.ContentPatch{Title: &title}})
	mustCode(t, err, "REQUEST_FORBIDDEN")
	if _, err := f.edit.Execute(asAdmin(), EditInput{RequestID: r.ID, ExpectedRevision: 1, Patch: domain.ContentPatch{Title: &title}}); err != nil {
		t.Fatalf("an admin may edit: %v", err)
	}
	_, err = f.edit.Execute(tenant.WithTenantID(context.Background(), "other"), EditInput{RequestID: r.ID, ExpectedRevision: 2, Patch: domain.ContentPatch{Title: &title}})
	if err == nil {
		t.Fatal("anonymous edit must fail")
	}
}

func TestEditRequestContent_RetiredACKeepsOldRevisionReadable(t *testing.T) {
	f := newArtFx()
	r := f.seed(func(r *domain.Request) {
		c := domain.RequestContent{Title: r.Title, Body: r.Body, Type: r.Type, TypeFields: map[string]any{}}
		_, _ = c.AcceptanceCriteria.Add("Đăng nhập được với mật khẩu có dấu cách", "test")
		withAC, _ := r.WithContent(c)
		*r = withAC
	})
	// seed recomputed the snapshot from the stored content, so revision 1 carries AC-1.
	drop := []domain.ACInput{}
	if _, err := f.edit.Execute(asUser(userAda), EditInput{RequestID: r.ID, ExpectedRevision: 1, Patch: domain.ContentPatch{AcceptanceCriteria: &drop}}); err != nil {
		t.Fatal(err)
	}
	add := []domain.ACInput{{Text: "Thông báo lỗi rõ ràng"}}
	if _, err := f.edit.Execute(asUser(userAda), EditInput{RequestID: r.ID, ExpectedRevision: 2, Patch: domain.ContentPatch{AcceptanceCriteria: &add}}); err != nil {
		t.Fatal(err)
	}
	now := contentOf(t, f.env.s.requests[r.ID])
	if it, _ := now.AcceptanceCriteria.Find("AC-1"); it.Status != domain.ACStatusRetired {
		t.Fatalf("AC-1 should be retired, not removed: %+v", now.AcceptanceCriteria.Items)
	}
	if it, ok := now.AcceptanceCriteria.Find("AC-2"); !ok || it.Text != "Thông báo lỗi rõ ràng" {
		t.Fatalf("the new criterion must take AC-2: %+v", now.AcceptanceCriteria.Items)
	}
	old, err := revisionsView{f.env}.Get(asUser(userAda), r.ID, 1)
	if err != nil || !strings.Contains(string(old.Snapshot), `"id":"AC-1"`) || strings.Contains(string(old.Snapshot), "retired") {
		t.Fatalf("revision 1 must still show AC-1 as active: %v %s", err, old.Snapshot)
	}
}

func TestPatchFromJSON(t *testing.T) {
	p, err := PatchFromJSON("t", "", true, false, `[{"text":"một"}]`, `{"a":1,"b":null}`)
	if err != nil || *p.Title != "t" || p.Body != nil || len(*p.AcceptanceCriteria) != 1 || p.TypeFields["a"] != float64(1) {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := PatchFromJSON("", "", false, false, `{`, ""); err == nil {
		t.Fatal("bad AC JSON must fail")
	}
	if _, err := PatchFromJSON("", "", false, false, "", `[1]`); err == nil {
		t.Fatal("bad type_fields JSON must fail")
	}
}
