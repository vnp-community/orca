package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestValidateMetrics_PerKind(t *testing.T) {
	good := map[CheckKind]string{
		CheckPerfBaseline:    `{"metrics":[{"name":"p95","unit":"ms","direction":"lower_is_better","baseline":200,"target_change_percent":20}],"method":"wrk -t4"}`,
		CheckPerfAfter:       `{"metrics":[{"name":"p95","value":150}]}`,
		CheckTestsBefore:     `{"total":40,"passed":40,"failed":0,"command":"go test ./..."}`,
		CheckTestsAfter:      `{"total":40,"passed":40,"failed":0,"command":"go test ./...","tests_modified":false}`,
		CheckSecurityRecheck: `{"scanner":"semgrep"}`,
		CheckOpsResult:       `{}`,
	}
	for kind, metrics := range good {
		if err := ValidateCheckSubmission(kind, CheckStatusPassed, json.RawMessage(metrics), "ok"); err != nil {
			t.Errorf("%s: valid payload rejected: %v", kind, err)
		}
	}
	bad := []struct {
		name    string
		kind    CheckKind
		status  CheckStatus
		metrics string
		summary string
	}{
		{"unknown kind", "nope", CheckStatusPassed, `{}`, "x"},
		{"unknown status", CheckPerfAfter, "maybe", `{"metrics":[{"name":"a","value":1}]}`, ""},
		{"not an object", CheckPerfAfter, CheckStatusPassed, `[1]`, ""},
		{"not json", CheckPerfAfter, CheckStatusPassed, `{`, ""},
		{"baseline without metrics", CheckPerfBaseline, CheckStatusPassed, `{"metrics":[],"method":"m"}`, ""},
		{"baseline bad direction", CheckPerfBaseline, CheckStatusPassed, `{"metrics":[{"name":"a","unit":"ms","direction":"sideways","baseline":1,"target_change_percent":1}],"method":"m"}`, ""},
		{"baseline missing target", CheckPerfBaseline, CheckStatusPassed, `{"metrics":[{"name":"a","unit":"ms","direction":"lower_is_better","baseline":1}],"method":"m"}`, ""},
		{"baseline duplicate name", CheckPerfBaseline, CheckStatusPassed, `{"metrics":[{"name":"a","unit":"ms","direction":"lower_is_better","baseline":1,"target_change_percent":1},{"name":"a","unit":"ms","direction":"lower_is_better","baseline":1,"target_change_percent":1}],"method":"m"}`, ""},
		{"baseline missing method", CheckPerfBaseline, CheckStatusPassed, `{"metrics":[{"name":"a","unit":"ms","direction":"lower_is_better","baseline":1,"target_change_percent":1}]}`, ""},
		{"after missing value", CheckPerfAfter, CheckStatusPassed, `{"metrics":[{"name":"a"}]}`, ""},
		{"tests negative", CheckTestsBefore, CheckStatusPassed, `{"total":-1,"passed":0,"failed":0,"command":"x"}`, ""},
		{"tests after missing tests_modified", CheckTestsAfter, CheckStatusPassed, `{"total":1,"passed":1,"failed":0,"command":"x"}`, ""},
		{"security without summary", CheckSecurityRecheck, CheckStatusPassed, `{}`, "  "},
		{"ops without summary", CheckOpsResult, CheckStatusFailed, `{}`, ""},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCheckSubmission(tc.kind, tc.status, json.RawMessage(tc.metrics), tc.summary)
			if err == nil || !strings.Contains(err.Error(), "REQUEST_CHECK_INVALID_METRICS") {
				t.Fatalf("want REQUEST_CHECK_INVALID_METRICS, got %v", err)
			}
		})
	}
}

func TestValidateMetrics_RejectsOversized(t *testing.T) {
	big := `{"pad":"` + strings.Repeat("x", MaxCheckMetricsBytes) + `"}`
	if err := ValidateCheckSubmission(CheckSecurityRecheck, CheckStatusPassed, json.RawMessage(big), "s"); err == nil {
		t.Fatal("a payload over 64 KB must be rejected")
	}
}

func TestCheckAllowedNow(t *testing.T) {
	cases := []struct {
		kind   CheckKind
		typ    RequestType
		status RequestStatus
		want   bool
	}{
		{CheckPerfBaseline, RequestTypePerformance, RequestStatusAnalyzing, true},
		{CheckPerfBaseline, RequestTypePerformance, RequestStatusPlanning, true},
		{CheckPerfBaseline, RequestTypePerformance, RequestStatusExecuting, false},
		{CheckPerfBaseline, RequestTypeBug, RequestStatusAnalyzing, false},
		{CheckPerfAfter, RequestTypePerformance, RequestStatusExecuting, true},
		{CheckPerfAfter, RequestTypePerformance, RequestStatusCompleted, false},
		{CheckTestsAfter, RequestTypeRefactor, RequestStatusExecuting, true},
		{CheckTestsAfter, RequestTypePerformance, RequestStatusExecuting, false},
		{CheckSecurityRecheck, RequestTypeSecurity, RequestStatusExecuting, true},
		{CheckSecurityRecheck, RequestTypeSecurity, RequestStatusRequestBacklog, false},
		{CheckOpsResult, RequestTypeOpsRequest, RequestStatusExecuting, true},
		{CheckOpsResult, RequestTypeHotfix, RequestStatusExecuting, false},
	}
	for _, tc := range cases {
		if got := CheckAllowedNow(tc.kind, tc.typ, tc.status); got != tc.want {
			t.Errorf("%s/%s/%s: got %v, want %v", tc.kind, tc.typ, tc.status, got, tc.want)
		}
	}
}

func TestMarkEffective_NewestPerKind(t *testing.T) {
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	checks := []RequestCheck{
		{ID: "1", Kind: CheckTestsBefore, CreatedAt: base},
		{ID: "2", Kind: CheckTestsBefore, CreatedAt: base.Add(time.Minute)},
		{ID: "3", Kind: CheckTestsAfter, CreatedAt: base},
	}
	got := MarkEffective(checks)
	want := map[string]bool{"1": false, "2": true, "3": true}
	for _, c := range got {
		if c.Effective != want[c.ID] {
			t.Errorf("check %s: Effective=%v, want %v", c.ID, c.Effective, want[c.ID])
		}
	}
	if checks[0].Effective || checks[1].Effective {
		t.Error("MarkEffective must not mutate its input")
	}
	if _, ok := LatestCheck(checks, CheckOpsResult); ok {
		t.Error("no ops_result recorded")
	}
}

func TestRequestChecks_EverySeedKindIsKnown(t *testing.T) {
	for _, k := range AllCheckKinds() {
		if !k.Valid() {
			t.Errorf("%s must be valid", k)
		}
		if _, ok := checkRules[k]; !ok {
			t.Errorf("%s has no placement rule", k)
		}
	}
	if CheckKind("x").Valid() {
		t.Error("unknown kind must be invalid")
	}
}
