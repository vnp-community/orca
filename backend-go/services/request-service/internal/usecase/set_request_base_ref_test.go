package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type setBaseRefFixture struct {
	store    *baseRefStore
	presence *fakePresence
	checker  *fakeRefChecker
	uc       *SetRequestBaseRef
}

func newSetBaseRefFixture(t *testing.T) *setBaseRefFixture {
	t.Helper()
	f := &setBaseRefFixture{store: newBaseRefStore(), presence: newFakePresence(), checker: &fakeRefChecker{}}
	uc, err := NewSetRequestBaseRef(SetRequestBaseRefDeps{
		Requests: f.store, Locker: f.store, Writer: f.store, Presence: f.presence, Checker: f.checker, Tx: f.store, Outbox: f.store,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.uc = uc
	return f
}

func (f *setBaseRefFixture) seed(mod func(r *domain.Request)) domain.Request {
	r := domain.Request{
		ID: uuid.NewString(), TenantID: testTenant, ProjectID: uuid.NewString(), Number: 1, Title: "PRIVATE-TITLE", Body: "PRIVATE-BODY",
		SourceProvider: domain.SourceProviderManual, Status: domain.RequestStatusNew, Urgency: domain.UrgencyNormal, Version: 3,
	}
	if mod != nil {
		mod(&r)
	}
	f.store.put(r)
	return r
}

func setIn(r domain.Request, ref string) SetRequestBaseRefInput {
	return SetRequestBaseRefInput{RequestID: r.ID, BaseRef: ref, ExpectedVersion: r.Version, ActorID: "u1", ActorKind: domain.ActorKindUser}
}

func requireNoWrite(t *testing.T, f *setBaseRefFixture, before domain.Request) {
	t.Helper()
	got, _ := f.store.read(before.ID)
	if got.BaseRef != before.BaseRef || got.Version != before.Version || len(f.store.events) != 0 {
		t.Fatalf("a refused change must write nothing: %+v events=%d", got, len(f.store.events))
	}
}

func lockReasonOf(t *testing.T, err error) string {
	t.Helper()
	mustCode(t, err, "REQUEST_BASE_REF_LOCKED")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("locked must be FailedPrecondition: %v", err)
	}
	d, _ := domain.BaseRefDetails(err)
	return d["reason"]
}

func TestSetRequestBaseRef_Unlocked_Succeeds(t *testing.T) {
	f := newSetBaseRefFixture(t)
	r := f.seed(nil)
	res, err := f.uc.Execute(tctx(), setIn(r, "  release/1.4 "))
	if err != nil {
		t.Fatal(err)
	}
	if res.Request.BaseRef != "release/1.4" || res.Request.Version != r.Version+1 || res.Unverified {
		t.Fatalf("%+v", res)
	}
	if f.checker.calls != 1 || f.checker.ref != "release/1.4" || f.checker.project != r.ProjectID {
		t.Fatalf("checker saw %q for %q (%d calls)", f.checker.ref, f.checker.project, f.checker.calls)
	}
	if len(f.store.events) != 1 || f.store.events[0].Subject != domain.SubjectRequestBaseRefChanged {
		t.Fatalf("events: %+v", f.store.events)
	}
	var payload map[string]any
	if err := json.Unmarshal(f.store.events[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["request_id"] != r.ID || payload["from"] != "" || payload["to"] != "release/1.4" || payload["actor_id"] != "u1" {
		t.Fatalf("payload: %v", payload)
	}
	if body := string(f.store.events[0].Payload); strings.Contains(body, "PRIVATE") {
		t.Fatalf("the event must not carry Request content: %s", body)
	}
}

func TestSetRequestBaseRef_EmptyRestoresDefaultWithoutCheck(t *testing.T) {
	f := newSetBaseRefFixture(t)
	r := f.seed(func(r *domain.Request) { r.BaseRef = "release/1.4" })
	res, err := f.uc.Execute(tctx(), setIn(r, ""))
	if err != nil || res.Request.BaseRef != "" || f.checker.calls != 0 {
		t.Fatalf("%+v %v checker calls=%d", res, err, f.checker.calls)
	}
}

func TestSetRequestBaseRef_LockedByProposal(t *testing.T) {
	f := newSetBaseRefFixture(t)
	r := f.seed(nil)
	f.presence.set(r.ID, domain.BaseRefLockedProposal)
	_, err := f.uc.Execute(tctx(), setIn(r, "main"))
	if got := lockReasonOf(t, err); got != "proposal" {
		t.Fatalf("reason %q", got)
	}
	requireNoWrite(t, f, r)
}

func TestSetRequestBaseRef_LockedByTask(t *testing.T) {
	f := newSetBaseRefFixture(t)
	r := f.seed(nil)
	f.presence.set(r.ID, domain.BaseRefLockedTask)
	_, err := f.uc.Execute(tctx(), setIn(r, "main"))
	if got := lockReasonOf(t, err); got != "task" {
		t.Fatalf("reason %q", got)
	}
	requireNoWrite(t, f, r)
}

func TestSetRequestBaseRef_Terminal(t *testing.T) {
	for _, st := range []domain.RequestStatus{domain.RequestStatusCompleted, domain.RequestStatusCancelled} {
		t.Run(string(st), func(t *testing.T) {
			f := newSetBaseRefFixture(t)
			r := f.seed(func(r *domain.Request) { r.Status = st })
			_, err := f.uc.Execute(tctx(), setIn(r, "main"))
			if got := lockReasonOf(t, err); got != "terminal" {
				t.Fatalf("reason %q", got)
			}
			requireNoWrite(t, f, r)
		})
	}
}

func TestSetRequestBaseRef_VersionConflict(t *testing.T) {
	f := newSetBaseRefFixture(t)
	r := f.seed(nil)
	in := setIn(r, "main")
	in.ExpectedVersion = r.Version - 1
	_, err := f.uc.Execute(tctx(), in)
	mustCode(t, err, "REQUEST_VERSION_CONFLICT")
	requireNoWrite(t, f, r)

	in.ExpectedVersion = 0 // 0 means "do not check", like ChangeRequestType
	if _, err := f.uc.Execute(tctx(), in); err != nil {
		t.Fatalf("expected_version 0 must skip the check: %v", err)
	}
}

func TestSetRequestBaseRef_NotFoundRef_RejectedNoWrite(t *testing.T) {
	f := newSetBaseRefFixture(t)
	r := f.seed(nil)
	f.checker.check = BaseRefCheck{Status: BaseRefNotFound, Suggestions: []string{"a", "b", "c", "d", "e", "f"}}
	_, err := f.uc.Execute(tctx(), setIn(r, "rel/9"))
	mustCode(t, err, "REQUEST_BASE_REF_NOT_FOUND")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindNotFound {
		t.Fatalf("kind: %v", err)
	}
	if d, _ := domain.BaseRefDetails(err); d["suggestions"] != "a,b,c,d,e" {
		t.Fatalf("at most 5 suggestions: %v", d)
	}
	requireNoWrite(t, f, r)
	if f.store.txOpened != 0 {
		t.Fatal("the existence check must run before the transaction so a slow dev server never holds the row lock")
	}
}

func TestSetRequestBaseRef_Unreachable_StoredWithWarning(t *testing.T) {
	for name, checker := range map[string]*fakeRefChecker{
		"unreachable status": {check: BaseRefCheck{Status: BaseRefUnreachable}},
		"checker error":      {err: errors.New("dial tcp: connection refused")},
	} {
		t.Run(name, func(t *testing.T) {
			f := newSetBaseRefFixture(t)
			f.uc.d.Checker = checker
			r := f.seed(nil)
			res, err := f.uc.Execute(tctx(), setIn(r, "release/1.4"))
			if err != nil || !res.Unverified || res.Request.BaseRef != "release/1.4" {
				t.Fatalf("fail-open: %+v %v", res, err)
			}
		})
	}
}

func TestSetRequestBaseRef_PermissionDenied(t *testing.T) {
	f := newSetBaseRefFixture(t)
	r := f.seed(nil)
	for _, kind := range []domain.ActorKind{domain.ActorKindAgent, domain.ActorKindSystem, ""} {
		in := setIn(r, "main")
		in.ActorKind = kind
		_, err := f.uc.Execute(tctx(), in)
		mustCode(t, err, "REQUEST_ACTOR_NOT_ALLOWED")
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied {
			t.Fatalf("kind: %v", err)
		}
	}
	if f.checker.calls != 0 {
		t.Fatal("a refused actor must not trigger a git call")
	}
	requireNoWrite(t, f, r)
}

func TestSetRequestBaseRef_InvalidRef_NoCheckNoWrite(t *testing.T) {
	f := newSetBaseRefFixture(t)
	r := f.seed(nil)
	for _, bad := range []string{"-x", "a..b", "   ", "a b", strings.Repeat("a", 256)} {
		_, err := f.uc.Execute(tctx(), setIn(r, bad))
		mustCode(t, err, "REQUEST_BASE_REF_INVALID")
	}
	if f.checker.calls != 0 || f.store.txOpened != 0 {
		t.Fatal("syntax errors must be rejected before any git call or transaction")
	}
	requireNoWrite(t, f, r)
}

func (f checkerFunc) Check(ctx context.Context, _, _ string) (BaseRefCheck, error) { return f(ctx) }
