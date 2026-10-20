package domain

import (
	"strings"
	"testing"
	"time"
)

var activityT0 = time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)

func TestActivityCursorRoundTrip(t *testing.T) {
	c := ActivityCursor{At: activityT0, ID: "abc:solution"}
	got, err := DecodeActivityCursor(c.Encode())
	if err != nil || got == nil || !got.At.Equal(activityT0) || got.ID != "abc:solution" {
		t.Fatalf("got %+v %v", got, err)
	}
	if got, err := DecodeActivityCursor(""); got != nil || err != nil {
		t.Fatalf("an empty token is the first page: %+v %v", got, err)
	}
	for _, bad := range []string{"%%%", "e30", ActivityCursor{At: activityT0}.Encode()} {
		if _, err := DecodeActivityCursor(bad); err == nil || !strings.Contains(err.Error(), "REQUEST_ACTIVITY_CURSOR_INVALID") && err.Error() == "" {
			t.Errorf("token %q must be refused", bad)
		}
	}
}

func TestParseActivitySeverity(t *testing.T) {
	for in, want := range map[string]ActivitySeverity{"": ActivityInfo, "INFO": ActivityInfo, "attention": ActivityAttention, " error ": ActivityError} {
		if got, err := ParseActivitySeverity(in); err != nil || got != want {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	if _, err := ParseActivitySeverity("loud"); err == nil {
		t.Fatal("an unknown severity is refused")
	}
	if !ActivityError.AtLeast(ActivityAttention) || ActivityInfo.AtLeast(ActivityAttention) || !ActivityInfo.AtLeast(ActivityInfo) {
		t.Fatal("severity order is info < attention < error")
	}
}

func TestBuildActivityEntry_TaskFailureSeverityFollowsTheAttempt(t *testing.T) {
	tasks := map[string]ActivityTask{"t1": {Title: "Run the e2e acceptance"}}
	failed := func(attempt int64, cause, msg string) ActivityEntry {
		return BuildActivityEntry(ActivityRow{Kind: ActivityTaskFailed, ID: "o", OccurredAt: activityT0, ActorKind: "agent", SubjectType: "task", SubjectID: "t1", A: "failed", B: cause, C: msg, N: attempt}, tasks, 2)
	}
	if e := failed(1, "execution_failed", "execution failed: agent_defect RESULT_BLOCK_MISSING"); e.Severity != ActivityAttention || e.Attempt != 1 || e.Details["cause"] != "agent_error" {
		t.Fatalf("first failure: %+v", e)
	}
	e := failed(2, "execution_failed", "execution failed: retryable TIMED_OUT")
	if e.Severity != ActivityError || e.Details["cause"] != "timed_out" || e.Details["taskTitle"] != "Run the e2e acceptance" || e.Details["maxAttempts"] != 2 {
		t.Fatalf("last failure: %+v", e)
	}
	if e := failed(2, CauseDispatchError, "TASK_EXECUTE_NO_CONNECTION"); e.Severity != ActivityAttention {
		t.Fatalf("a dispatch error never used an attempt: %+v", e)
	}
	if !strings.Contains(e.Summary, "Run the e2e acceptance") {
		t.Fatalf("the summary names the task: %q", e.Summary)
	}
}

func TestBuildActivityEntry_UnknownTaskIsNamedByItsID(t *testing.T) {
	e := BuildActivityEntry(ActivityRow{Kind: ActivityTaskStarted, OccurredAt: activityT0, SubjectID: "0123456789abcdef", N: 1}, nil, 2)
	if !strings.Contains(e.Summary, "01234567") || e.Attempt != 1 {
		t.Fatalf("%+v", e)
	}
}

func TestBuildActivityEntry_ByKind(t *testing.T) {
	cases := []struct {
		name string
		row  ActivityRow
		sev  ActivitySeverity
		actr string
	}{
		{"classified by AI", ActivityRow{Kind: ActivityRequestClassified, ActorKind: "agent", B: "change_request", C: "new capability"}, ActivityInfo, "ai"},
		{"type changed by a person", ActivityRow{Kind: ActivityRequestTypeChange, ActorKind: "user", A: "bug", B: "task"}, ActivityInfo, "user"},
		{"returned", ActivityRow{Kind: ActivityRequestReturned, ActorKind: "system", A: "returned", B: "task", C: "other", D: "task failed twice"}, ActivityError, "system"},
		{"reopened", ActivityRow{Kind: ActivityRequestReopened, ActorKind: "user", A: "reopened"}, ActivityInfo, "user"},
		{"pending approval", ActivityRow{Kind: ActivityApprovalRequested, SubjectType: "plan", A: "awaiting_plan_approval", B: "pending", ActorKind: "system"}, ActivityAttention, "system"},
		{"settled approval request", ActivityRow{Kind: ActivityApprovalRequested, SubjectType: "plan", B: "approved", ActorKind: "user"}, ActivityInfo, "user"},
		{"approval approved", ActivityRow{Kind: ActivityApprovalDecided, SubjectType: "plan", A: "approved", ActorKind: "user"}, ActivityInfo, "user"},
		{"approval rejected", ActivityRow{Kind: ActivityApprovalDecided, SubjectType: "plan", A: "rejected", C: "too risky", ActorKind: "user"}, ActivityAttention, "user"},
		{"open question", ActivityRow{Kind: ActivityClarifyAsked, B: "open", ActorKind: "system"}, ActivityAttention, "system"},
		{"answered question", ActivityRow{Kind: ActivityClarifyAnswered, ActorKind: "user"}, ActivityInfo, "user"},
		{"phase started by Orca", ActivityRow{Kind: ActivityPhaseStarted, A: "auto", ActorKind: "system"}, ActivityInfo, "system"},
		{"analysis", ActivityRow{Kind: ActivityAnalysisGenerated, A: "solution", B: "approved", N: 2, ActorKind: "system"}, ActivityInfo, "system"},
		{"an actor nobody knows", ActivityRow{Kind: ActivityRequestCreated, ActorKind: "robot"}, ActivityInfo, "system"},
	}
	for _, c := range cases {
		c.row.OccurredAt = activityT0
		e := BuildActivityEntry(c.row, nil, 2)
		if e.Severity != c.sev || e.ActorKind != c.actr || e.Summary == "" || e.Kind != c.row.Kind {
			t.Errorf("%s: %+v", c.name, e)
		}
	}
	long := strings.Repeat("é", 1000)
	e := BuildActivityEntry(ActivityRow{Kind: ActivityRequestReturned, OccurredAt: activityT0, D: long}, nil, 2)
	if r, _ := e.Details["reason"].(string); len(r) > 410 {
		t.Fatalf("details are bounded: %d bytes", len(r))
	}
}

func TestBuildActivityEntry_AnalysisRevisionRequestedIsInfoWithAFixedSummary(t *testing.T) {
	row := ActivityRow{Kind: ActivityAnalysisRevisionRequested, ID: "r:revision", OccurredAt: activityT0, ActorKind: "user", ActorID: "u1", SubjectType: "solution", SubjectID: "s1"}
	e := BuildActivityEntry(row, nil, 2)
	if e.Summary != "Requested changes to the analysis" || e.Severity != ActivityInfo || e.ActorKind != "user" || e.ActorID != "u1" || e.SubjectID != "s1" || e.Details["solutionId"] != "s1" {
		t.Fatalf("entry: %+v", e)
	}
	if len(e.Details) != 1 {
		t.Fatalf("the line carries no reviewer text, only the solution id: %+v", e.Details)
	}
}

func TestAllActivityKinds_NamesTheRevisionKindOnce(t *testing.T) {
	n := 0
	seen := map[string]bool{}
	for _, k := range AllActivityKinds {
		if seen[k] {
			t.Fatalf("kind %q listed twice", k)
		}
		seen[k] = true
		if k == "analysis.revision_requested" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("analysis.revision_requested must be a valid filter kind, found %d times", n)
	}
}

func TestBuildActivityEntry_PlanRevisionRequestedIsInfoWithAFixedSummary(t *testing.T) {
	row := ActivityRow{Kind: ActivityPlanRevisionRequested, ID: "r:revision", OccurredAt: activityT0, ActorKind: "user", ActorID: "u1", SubjectType: "plan"}
	e := BuildActivityEntry(row, nil, 2)
	if e.Summary != "Requested changes to the plan" || e.Severity != ActivityInfo || e.ActorKind != "user" || e.ActorID != "u1" || len(e.Details) != 0 {
		t.Fatalf("the line carries no reviewer text and no detail: %+v", e)
	}
}

func TestAllActivityKinds_NamesThePlanRevisionKindOnce(t *testing.T) {
	n := 0
	for _, k := range AllActivityKinds {
		if k == "plan.revision_requested" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("plan.revision_requested must be a valid filter kind, found %d times", n)
	}
}
