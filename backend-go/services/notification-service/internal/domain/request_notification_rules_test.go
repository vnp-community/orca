package domain

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func goldenPayload(t *testing.T, name string) EventPayload {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "request_notification", name))
	if err != nil {
		t.Fatal(err)
	}
	p, err := DecodePayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTranslateEvent_RequestSubjects(t *testing.T) {
	cases := []struct {
		name, file, subject, typ, title, deepLink string
		severity                                  Severity
		channels                                  int
		recipients                                int
	}{
		{"approval requested", "approval_requested.json", "orca.request.approval.requested", TypeRequestApprovalRequested, "Approval needed", "/?section=requests&request=req-1&approval=apr-1", SeverityWarning, 2, 2},
		{"approval reminder", "approval_requested_reminder.json", "orca.request.approval.requested", TypeRequestApprovalRequested, "Approval reminder", "/?section=requests&request=req-1&approval=apr-1", SeverityWarning, 2, 1},
		{"approval decided", "approval_decided.json", "orca.request.approval.decided", TypeRequestApprovalDecided, "Plan rejected", "/?section=requests&request=req-1&approval=apr-1", SeverityInfo, 2, 1},
		{"clarification requested", "clarification_requested.json", "orca.request.clarification.requested", TypeRequestClarificationRequested, "Request cần bổ sung thông tin", "/?section=requests&request=req-1&clarification=clr-1", SeverityWarning, 2, 2},
		{"clarification reminder", "clarification_requested_reminder.json", "orca.request.clarification.requested", TypeRequestClarificationRequested, "Nhắc: cần bổ sung thông tin", "/?section=requests&request=req-1&clarification=clr-1", SeverityWarning, 2, 1},
		{"clarification expired", "clarification_expired.json", "orca.request.clarification.expired", TypeRequestClarificationExpired, "Yêu cầu bổ sung thông tin đã hết hạn", "/?section=requests&request=req-1", SeverityWarning, 1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := TranslateEvent("ne-1", "evt-1", c.subject, "t1", goldenPayload(t, c.file), time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if got.Type != c.typ || got.Title != c.title || got.DeepLink != c.deepLink || got.Severity != c.severity ||
				len(got.Channels) != c.channels || len(got.RecipientUserIDs) != c.recipients || got.Body == "" || got.SourceSubject != c.subject {
				t.Fatalf("unexpected %+v", got)
			}
		})
	}
}

func TestTranslateEvent_RequestSubjectsWithoutUserIDsIsErrNoRecipients(t *testing.T) {
	for _, s := range []string{"orca.request.approval.requested", "orca.request.approval.decided", "orca.request.clarification.requested", "orca.request.clarification.expired"} {
		_, err := TranslateEvent("ne", "evt", s, "t1", goldenPayload(t, "approval_requested_no_recipients.json"), time.Now())
		if !errors.Is(err, ErrNoRecipients) {
			t.Errorf("%s: err = %v, want ErrNoRecipients", s, err)
		}
	}
}

func TestTranslateEvent_RequestSubjectsFallBackToRuleDefaults(t *testing.T) {
	got, err := TranslateEvent("ne", "evt", "orca.request.approval.decided", "t1", EventPayload{UserIDs: []string{"u1"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Approval decided" || got.DeepLink != "/?section=requests" {
		t.Fatalf("defaults not applied: %+v", got)
	}
}

func TestTranslateEvent_RequestSubjectsRejectExternalDeepLink(t *testing.T) {
	for _, link := range []string{"https://evil.example/x", "//evil.example", `/\evil.example`, "javascript:alert(1)"} {
		got, err := TranslateEvent("ne", "evt", "orca.request.clarification.requested", "t1",
			EventPayload{UserIDs: []string{"u1"}, DeepLink: link}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if got.DeepLink != "/?section=requests" {
			t.Errorf("deep_link %q leaked: %q", link, got.DeepLink)
		}
	}
}

func TestTranslateEvent_RequestRulesDoNotChangeLegacyDeepLinkBehaviour(t *testing.T) {
	got, err := TranslateEvent("ne", "evt", "orca.mcp.approval.requested", "t1",
		EventPayload{UserID: "u1", DeepLink: "https://example.test/x"}, time.Now())
	if err != nil || got.DeepLink != "https://example.test/x" {
		t.Fatalf("legacy rule changed: %+v %v", got, err)
	}
}
