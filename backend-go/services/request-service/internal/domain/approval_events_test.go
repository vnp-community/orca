package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func loadGolden(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "approval_events", name))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

func TestApprovalEvents_RequestedPayloadMatchesGoldenAndNeverCarriesComment(t *testing.T) {
	due := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	a := Approval{ID: "apr-1", RequestID: "req-1", SubjectType: SubjectPlan, SubjectID: "plan-1", Stage: "awaiting_plan_approval",
		RequestedBy: "user-9", DueAt: &due, SelfApprovalAllowed: true, Comment: "secret rationale"}
	got := asMap(t, NewApprovalRequestedPayload(a, Request{Number: 12, ReporterID: "u-reporter"}, ""))
	if !reflect.DeepEqual(got, loadGolden(t, "requested.json")) {
		t.Fatalf("payload %v does not match the golden file", got)
	}
	if strings.Contains(strings.ToLower(mustJSON(got)), "comment") || strings.Contains(mustJSON(got), "secret rationale") {
		t.Fatal("payload carries the comment")
	}
	if r := asMap(t, NewApprovalRequestedPayload(a, Request{}, "reminder")); r["reason"] != "reminder" {
		t.Fatalf("reminder reason missing: %v", r)
	}
}

func TestApprovalEvents_DecidedPayloadMatchesGolden(t *testing.T) {
	by := "u-approver-1"
	a := Approval{ID: "apr-1", RequestID: "req-1", SubjectType: SubjectPlan, SubjectID: "plan-1", RequestedBy: "user-9", DecidedBy: &by, Comment: "scope too wide"}
	got := asMap(t, NewApprovalDecidedPayload(a, Request{Number: 12, ReporterID: "u-reporter"}, "rejected"))
	if !reflect.DeepEqual(got, loadGolden(t, "decided.json")) {
		t.Fatalf("payload %v does not match the golden file", got)
	}
	if strings.Contains(mustJSON(got), "scope too wide") {
		t.Fatal("payload carries the comment")
	}
	if sys := asMap(t, NewApprovalDecidedPayload(Approval{ID: "a"}, Request{}, "expired")); sys["decided_by"] != nil {
		t.Fatalf("an expiry has no decider: %v", sys)
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
