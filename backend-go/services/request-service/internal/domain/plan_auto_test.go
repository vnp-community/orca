package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

func TestIsTransientPlanRunCode(t *testing.T) {
	transient := []string{
		"REQUEST_AI_COMPLETE_TIMEOUT", "REQUEST_PLAN_AI_UNAVAILABLE", "REQUEST_AI_BUSY",
		"REQUEST_OPENSPEC_TIMEOUT", "REQUEST_OPENSPEC_AGENT_FAILED", "REQUEST_SOLUTION_RUN_INTERRUPTED",
	}
	for _, code := range transient {
		if !IsTransientPlanRunCode(code) {
			t.Errorf("%s must be transient", code)
		}
	}
	permanent := []string{
		"REQUEST_PLAN_AI_INVALID_JSON", "REQUEST_OPENSPEC_INVALID_OUTPUT", "REQUEST_SPEC_RUN_OUT_OF_SCOPE",
		"REQUEST_AI_NOT_CONFIGURED", "REQUEST_AI_QUOTA_EXHAUSTED", "REQUEST_AI_AGENT_UNAVAILABLE", "REQUEST_AI_SESSION_EXPIRED",
		"REQUEST_BASE_REF_NOT_FOUND", "REQUEST_PLAN_REQUEST_MOVED", "REQUEST_PLAN_NOT_APPLICABLE", "REQUEST_ENGINE_UNAVAILABLE",
		"REQUEST_ANALYSIS_NO_CONNECTION", "REQUEST_ANALYSIS_NO_REPO_PATH", "REQUEST_SOLUTION_AI_FAILED", "REQUEST_SOLUTION_AI_TIMEOUT",
		"REQUEST_PLAN_COMMIT_FAILED", "", "SOMETHING_ELSE", "request_ai_busy",
	}
	for _, code := range permanent {
		if IsTransientPlanRunCode(code) {
			t.Errorf("%q must not be transient", code)
		}
	}
}

func TestPlanAutoStateCanMove(t *testing.T) {
	all := []PlanAutoState{PlanAutoPending, PlanAutoRunning, PlanAutoCommitting, PlanAutoDone, PlanAutoGaveUp, PlanAutoSkipped, PlanAutoSuperseded}
	allowed := map[PlanAutoState][]PlanAutoState{
		PlanAutoPending:    {PlanAutoPending, PlanAutoRunning, PlanAutoGaveUp, PlanAutoSkipped, PlanAutoSuperseded},
		PlanAutoRunning:    {PlanAutoRunning, PlanAutoPending, PlanAutoCommitting, PlanAutoGaveUp, PlanAutoSuperseded},
		PlanAutoCommitting: {PlanAutoCommitting, PlanAutoDone, PlanAutoGaveUp, PlanAutoSuperseded},
	}
	for _, from := range all {
		for _, to := range all {
			want := false
			for _, a := range allowed[from] {
				want = want || a == to
			}
			if got := from.CanMove(to); got != want {
				t.Errorf("%s -> %s: got %v, want %v", from, to, got, want)
			}
		}
	}
	for _, s := range all {
		active := s == PlanAutoPending || s == PlanAutoRunning || s == PlanAutoCommitting
		if s.IsActive() != active || s.IsFinal() == active {
			t.Errorf("%s: IsActive=%v IsFinal=%v", s, s.IsActive(), s.IsFinal())
		}
	}
	if PlanAutoState("").IsActive() || PlanAutoState("").IsFinal() || PlanAutoState("").CanMove(PlanAutoPending) {
		t.Error("an unknown state is neither active nor final and moves nowhere")
	}
}

func TestPlanAutoKey(t *testing.T) {
	cases := []struct {
		anchor  string
		attempt int
		want    string
	}{
		{"sol-1", 1, "plan-auto-sol-1"},
		{"sol-1", 2, "plan-auto-sol-1-2"},
		{"sol-1", 3, "plan-auto-sol-1-3"},
		{TypeConfirmAnchor("evt"), 1, "plan-auto-ty-evt"},
		{RetryAnchor("u"), 2, "plan-auto-retry-u-2"},
	}
	for _, c := range cases {
		got, err := PlanAutoKey(c.anchor, c.attempt)
		if err != nil || got != c.want {
			t.Errorf("PlanAutoKey(%q,%d) = %q, %v; want %q", c.anchor, c.attempt, got, err, c.want)
		}
	}
	// 36-character UUID anchors fit with room to spare; the limit is the column width.
	fits := strings.Repeat("a", PlanAutoKeyMaxLen-len("plan-auto-"))
	if k, err := PlanAutoKey(fits, 1); err != nil || len(k) != PlanAutoKeyMaxLen {
		t.Errorf("a key of exactly %d characters must be accepted: %d, %v", PlanAutoKeyMaxLen, len(k), err)
	}
	for _, c := range []struct {
		anchor  string
		attempt int
	}{{fits + "a", 1}, {fits, 2}, {"", 1}, {"x", 0}, {"x", -1}} {
		_, err := PlanAutoKey(c.anchor, c.attempt)
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Kind != apperrors.KindInvalidArgument {
			t.Errorf("PlanAutoKey(%d chars,%d) must fail with an invalid-argument error, got %v", len(c.anchor), c.attempt, err)
		}
	}
}

func TestPlanAutoPolicyFromEnv(t *testing.T) {
	got, warns := PlanAutoPolicyFromEnv(envOf(nil))
	if got != DefaultPlanAutoPolicy || len(warns) != 0 {
		t.Fatalf("defaults expected, got %+v %v", got, warns)
	}
	if got.MaxExtraAttempts != 1 || got.MaxAttempts() != 2 || got.RetryDelay != 30*time.Second || got.BusyDelay != time.Minute ||
		got.StartWindow != 30*time.Minute || got.MaxCommitAttempts != 5 || got.PollInterval != 5*time.Second || got.LeaseTTL != time.Minute {
		t.Errorf("default values changed: %+v", got)
	}

	got, warns = PlanAutoPolicyFromEnv(envOf(map[string]string{
		"REQUEST_PLAN_AUTO_MAX_EXTRA_ATTEMPTS": "0", "REQUEST_PLAN_AUTO_RETRY_DELAY": "2s", "REQUEST_PLAN_AUTO_BUSY_DELAY": " 10s ",
		"REQUEST_PLAN_AUTO_START_WINDOW": "5m", "REQUEST_PLAN_AUTO_POLL_INTERVAL": "500ms",
	}))
	if len(warns) != 0 || got.MaxExtraAttempts != 0 || got.MaxAttempts() != 1 || got.RetryDelay != 2*time.Second || got.BusyDelay != 10*time.Second ||
		got.StartWindow != 5*time.Minute || got.PollInterval != 500*time.Millisecond {
		t.Errorf("valid overrides misread: %+v %v", got, warns)
	}

	got, warns = PlanAutoPolicyFromEnv(envOf(map[string]string{"REQUEST_PLAN_AUTO_MAX_EXTRA_ATTEMPTS": "5"}))
	if got.MaxExtraAttempts != 2 || len(warns) != 1 || !strings.Contains(warns[0], "REQUEST_PLAN_AUTO_MAX_EXTRA_ATTEMPTS") {
		t.Errorf("5 must clamp to 2 with a warning: %+v %v", got, warns)
	}
	got, warns = PlanAutoPolicyFromEnv(envOf(map[string]string{"REQUEST_PLAN_AUTO_MAX_EXTRA_ATTEMPTS": "-3"}))
	if got.MaxExtraAttempts != 0 || len(warns) != 1 {
		t.Errorf("-3 must clamp to 0 with a warning: %+v %v", got, warns)
	}

	got, warns = PlanAutoPolicyFromEnv(envOf(map[string]string{
		"REQUEST_PLAN_AUTO_MAX_EXTRA_ATTEMPTS": "many", "REQUEST_PLAN_AUTO_RETRY_DELAY": "soon", "REQUEST_PLAN_AUTO_BUSY_DELAY": "0s",
		"REQUEST_PLAN_AUTO_START_WINDOW": "-1m", "REQUEST_PLAN_AUTO_POLL_INTERVAL": "30",
	}))
	if got != DefaultPlanAutoPolicy || len(warns) != 5 {
		t.Errorf("unreadable values must keep the defaults and warn once each: %+v %v", got, warns)
	}
}

func TestWantsAutoPlan(t *testing.T) {
	triggers := []Trigger{TriggerAnalysisApproved, TriggerTypeConfirmed, TriggerPlanRevision, TriggerAnalysisReady, TriggerInformationProvided}
	want := map[RequestType]Trigger{
		RequestTypeChangeRequest: TriggerAnalysisApproved, RequestTypeBug: TriggerAnalysisApproved,
		RequestTypeRefactor: TriggerAnalysisApproved, RequestTypeSecurity: TriggerAnalysisApproved,
		RequestTypePerformance: TriggerAnalysisApproved,
		RequestTypeTask:        TriggerTypeConfirmed, RequestTypeDocs: TriggerTypeConfirmed, RequestTypeOpsRequest: TriggerTypeConfirmed,
	}
	if len(AllFlowTypes()) != 11 {
		t.Fatalf("the matrix covers 11 types, registry has %d", len(AllFlowTypes()))
	}
	for _, typ := range AllFlowTypes() {
		flow, err := FlowFor(typ)
		if err != nil {
			t.Fatal(err)
		}
		for _, size := range []RequestSize{"", RequestSizeL} {
			for _, planTask := range []string{"", "plan-1"} {
				for _, to := range []RequestStatus{RequestStatusPlanning, RequestStatusAnalyzing} {
					for _, trig := range triggers {
						req := Request{Type: typ, Size: size, PlanTaskID: planTask}
						expect := planTask == "" && to == RequestStatusPlanning && want[typ] == trig
						if got := WantsAutoPlan(req, flow, trig, to); got != expect {
							t.Errorf("%s size=%q planTask=%q to=%s trigger=%s: got %v, want %v", typ, size, planTask, to, trig, got, expect)
						}
					}
				}
			}
		}
	}
}

func TestErrPlanAutoDisabled(t *testing.T) {
	var ae *apperrors.AppError
	if err := ErrPlanAutoDisabled(); !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition || ae.Code != "REQUEST_PLAN_AUTO_DISABLED" {
		t.Errorf("want FailedPrecondition REQUEST_PLAN_AUTO_DISABLED, got %v", err)
	}
}
