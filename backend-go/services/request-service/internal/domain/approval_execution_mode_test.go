package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

func appErrCode(t *testing.T, err error) (apperrors.Kind, string) {
	t.Helper()
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("not an AppError: %v", err)
	}
	return ae.Kind, ae.Code
}

func TestParseApprovalExecutionMode(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    ExecutionMode
		wantErr bool
	}{
		{"empty is no choice", "", "", false},
		{"blanks are no choice", "  ", "", false},
		{"continuous", "continuous", ExecutionContinuous, false},
		{"upper-cased first letter", "Continuous", ExecutionContinuous, false},
		{"padded and upper-cased", " CONTINUOUS ", ExecutionContinuous, false},
		{"step_by_step is not a choice", "step_by_step", "", true},
		{"suffix is not trimmed", "continuous;x", "", true},
		{"long garbage", strings.Repeat("x", 1000), "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseApprovalExecutionMode(c.in)
			if got != c.want {
				t.Fatalf("mode %q, want %q", got, c.want)
			}
			if !c.wantErr {
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				return
			}
			kind, code := appErrCode(t, err)
			if kind != apperrors.KindInvalidArgument || code != "REQUEST_EXECUTION_MODE_INVALID" {
				t.Fatalf("got %v %s", kind, code)
			}
		})
	}
}

func TestExecutionModeChoiceApplies(t *testing.T) {
	applies := map[SubjectType]bool{SubjectPlan: true, SubjectTaskList: true, SubjectPreDeploy: true}
	stages := []struct {
		stage string
		ok    bool
	}{
		{string(RequestStatusAwaitingPlanApproval), true},
		{string(RequestStatusExecuting), false},
		{string(RequestStatusAwaitingAnalysisApproval), false},
		{"", false},
	}
	for _, st := range AllSubjectTypes {
		for _, s := range stages {
			want := applies[st] && s.ok
			if got := ExecutionModeChoiceApplies(Approval{SubjectType: st, Stage: s.stage}); got != want {
				t.Errorf("subject %s stage %q: %v, want %v", st, s.stage, got, want)
			}
		}
	}
	if !ExecutionModeChoiceApplies(Approval{SubjectType: SubjectPlan, Stage: "awaiting_plan_approval"}) {
		t.Error("plan at awaiting_plan_approval must apply")
	}
}

func TestExecutionModeErrors(t *testing.T) {
	kind, code := appErrCode(t, ErrExecutionModeNotApplicable())
	if kind != apperrors.KindFailedPrecondition || code != "REQUEST_EXECUTION_MODE_NOT_APPLICABLE" {
		t.Fatalf("not applicable: %v %s", kind, code)
	}
	err := ErrExecutionModeAlreadyPinned(ExecutionStepByStep)
	kind, code = appErrCode(t, err)
	if kind != apperrors.KindFailedPrecondition || code != "REQUEST_EXECUTION_MODE_ALREADY_PINNED" {
		t.Fatalf("already pinned: %v %s", kind, code)
	}
	if !strings.Contains(err.Error(), "step_by_step") {
		t.Fatalf("message should name the pinned mode: %v", err)
	}
}

func TestApprovalDecidedPayload_ExecutionModeKey(t *testing.T) {
	p := NewApprovalDecidedPayload(Approval{ID: "a"}, Request{}, "approved")
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "execution_mode") {
		t.Fatalf("no chosen mode must not add the key: %s", b)
	}
	p.ExecutionMode = string(ExecutionContinuous)
	if b, err = json.Marshal(p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"execution_mode":"continuous"`) {
		t.Fatalf("chosen mode missing: %s", b)
	}
}
