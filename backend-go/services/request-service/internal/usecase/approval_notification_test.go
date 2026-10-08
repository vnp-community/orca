package usecase

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func goldenJSON(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "approval_notification", name))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func enrichFixture(t *testing.T) (*PublishApprovalNotifications, *apprEnv) {
	t.Helper()
	e := newApprEnv()
	e.s.approvers["apr-1"] = []domain.Principal{{Kind: domain.PrincipalKindUser, ID: "u-approver-1"}, {Kind: domain.PrincipalKindUser, ID: "u-approver-2"}}
	e.s.approvers["apr-2"] = nil
	ex := &ExpandApprovalRecipients{ApproverRepo: apprApprovers{e.s}, TeamResolver: e.teams, AdminResolver: e.admins}
	return &PublishApprovalNotifications{Expander: ex}, e
}

func enrich(t *testing.T, n *PublishApprovalNotifications, subject string, in map[string]any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(in)
	out, err := n.Enrich(lcCtx(), subject, b)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	return m
}

func TestApprovalNotification_RequestedMatchesGolden(t *testing.T) {
	n, _ := enrichFixture(t)
	due := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	got := enrich(t, n, domain.SubjectApprovalRequested, map[string]any{
		"approval_id": "apr-1", "request_id": "req-1", "request_number": 12, "subject_type": "plan", "subject_id": "plan-1",
		"stage": "awaiting_plan_approval", "requested_by": "user-9", "due_at": due, "reporter_id": "u-reporter", "self_approval_allowed": true,
	})
	want := goldenJSON(t, "approval_requested.json")
	for k, v := range want {
		if !reflect.DeepEqual(got[k], v) {
			t.Errorf("field %s = %v, golden %v", k, got[k], v)
		}
	}
	if _, has := got["comment"]; has {
		t.Error("no comment in a stored notification")
	}
}

func TestApprovalNotification_ReminderAndNoRecipientsMatchGolden(t *testing.T) {
	n, _ := enrichFixture(t)
	got := enrich(t, n, domain.SubjectApprovalRequested, map[string]any{
		"approval_id": "apr-1", "request_id": "req-1", "request_number": 12, "subject_type": "plan", "reason": "reminder", "reporter_id": "u-reporter",
	})
	want := goldenJSON(t, "approval_requested_reminder.json")
	delete(want, "user_ids") // the golden lists one recipient; the fixture snapshot has two approvers
	for k, v := range want {
		if !reflect.DeepEqual(got[k], v) {
			t.Errorf("reminder field %s = %v, golden %v", k, got[k], v)
		}
	}
	none := enrich(t, n, domain.SubjectApprovalRequested, map[string]any{"approval_id": "apr-2", "request_id": "req-1", "subject_type": "plan"})
	if _, has := none["user_ids"]; has || none["title"] != "Approval needed" {
		t.Fatalf("no approvers means no user_ids: %v", none)
	}
}

func TestApprovalNotification_DecidedMatchesGoldenAndExcludesDecider(t *testing.T) {
	n, _ := enrichFixture(t)
	got := enrich(t, n, domain.SubjectApprovalDecided, map[string]any{
		"approval_id": "apr-1", "request_id": "req-1", "subject_type": "plan", "decision": "rejected", "decided_by": "u-approver-1",
		"reporter_id": "u-reporter", "requested_by": "system", "comment": "must never leak",
	})
	want := goldenJSON(t, "approval_decided.json")
	for k, v := range want {
		if !reflect.DeepEqual(got[k], v) {
			t.Errorf("decided field %s = %v, golden %v", k, got[k], v)
		}
	}
	if _, has := got["comment"]; has {
		t.Error("comment must be stripped")
	}
	self := enrich(t, n, domain.SubjectApprovalDecided, map[string]any{
		"approval_id": "apr-1", "request_id": "req-1", "subject_type": "plan", "decision": "approved", "decided_by": "u-reporter", "reporter_id": "u-reporter", "requested_by": "u-reporter",
	})
	if _, has := self["user_ids"]; has {
		t.Fatalf("the decider is never notified of their own decision: %v", self)
	}
}

func TestApprovalNotification_BadDecisionTypeDoesNotPanic(t *testing.T) {
	n, _ := enrichFixture(t)
	out, err := n.Enrich(lcCtx(), domain.SubjectApprovalDecided, []byte(`{"approval_id":"a","request_id":"r","decision":42,"subject_type":7}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"deep_link"`) {
		t.Fatalf("out = %s", out)
	}
	if _, err := n.Enrich(lcCtx(), domain.SubjectApprovalDecided, []byte(`not json`)); err != ErrNotEnrichable {
		t.Fatalf("malformed payloads are published as is: %v", err)
	}
}

func TestApprovalNotification_DeepLinkIsRealRequestID(t *testing.T) {
	n, _ := enrichFixture(t)
	got := enrich(t, n, domain.SubjectApprovalRequested, map[string]any{"approval_id": "apr-1", "request_id": "11111111-2222", "subject_type": "plan"})
	if got["deep_link"] != "/?section=requests&request=11111111-2222&approval=apr-1" || strings.Contains(got["deep_link"].(string), "req_id") {
		t.Fatalf("deep_link = %v", got["deep_link"])
	}
}

func TestApprovalRecipients_ExpandsAllKindsDedupesCapsAndExcludesRequester(t *testing.T) {
	e := newApprEnv()
	e.s.approvers["a1"] = []domain.Principal{
		{Kind: domain.PrincipalKindUser, ID: "u1"}, {Kind: domain.PrincipalKindReporter}, {Kind: domain.PrincipalKindTeam, ID: "tm"},
		{Kind: domain.PrincipalKindRole, ID: "admin"}, {Kind: domain.PrincipalKindRole, ID: "custom"},
	}
	e.teams.members["tm"] = []string{"u1", "u2", "req"}
	e.admins.admins = []string{"adm", "u2"}
	ex := &ExpandApprovalRecipients{ApproverRepo: apprApprovers{e.s}, TeamResolver: e.teams, AdminResolver: e.admins}
	got, err := ex.Execute(lcCtx(), "a1", true, "rep", "req")
	want := []string{"u1", "rep", "u2", "req", "adm"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v err %v, want %v", got, err, want)
	}
	got, _ = ex.Execute(lcCtx(), "a1", false, "rep", "req")
	if !reflect.DeepEqual(got, []string{"u1", "u2", "adm"}) {
		t.Fatalf("separation of duties: %v", got)
	}
	ex.MaxRecipients = 2
	if got, _ = ex.Execute(lcCtx(), "a1", true, "rep", "req"); len(got) != 2 {
		t.Fatalf("cap: %v", got)
	}
	e.teams.err = domain.ErrApprovalDirectoryUnavailable
	if _, err = ex.Execute(lcCtx(), "a1", true, "rep", "req"); errCode(err) != "REQUEST_APPROVAL_DIRECTORY_UNAVAILABLE" {
		t.Fatalf("lookup failures go back to the relay: %v", err)
	}
}
