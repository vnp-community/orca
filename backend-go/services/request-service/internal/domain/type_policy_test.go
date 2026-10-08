package domain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeChecks map[CheckKind]RequestCheck

func (f fakeChecks) LatestCheck(_ context.Context, _ string, kind CheckKind) (RequestCheck, bool, error) {
	c, ok := f[kind]
	return c, ok, nil
}

type fakeApprovals map[string]*Approval

func (f fakeApprovals) LatestApproval(_ context.Context, _ string, st SubjectType, id string) (*Approval, error) {
	return f[string(st)+"/"+id], nil
}

func check(kind CheckKind, status CheckStatus, source CheckSource, metrics string, at int) RequestCheck {
	return RequestCheck{
		ID: string(kind) + "-" + status2s(at), Kind: kind, Status: status, Source: source, Metrics: json.RawMessage(metrics),
		CreatedAt: time.Date(2026, 10, 1, 0, at, 0, 0, time.UTC),
	}
}

func status2s(n int) string { return string(rune('a' + n)) }

func TestPolicyFor_UnknownType_ReturnsNoop(t *testing.T) {
	reg := NewPolicyRegistry(PolicyDeps{})
	for _, typ := range []RequestType{"", "mystery"} {
		if _, ok := reg.PolicyFor(typ).(noopPolicy); !ok {
			t.Errorf("%q must get the no-op policy", typ)
		}
	}
	var nilReg *PolicyRegistry
	if _, ok := nilReg.PolicyFor(RequestTypeHotfix).(noopPolicy); !ok {
		t.Error("a nil registry must degrade to the no-op policy")
	}
}

func TestPolicyFor_AllKnownTypesRegistered(t *testing.T) {
	reg := NewPolicyRegistry(PolicyDeps{})
	withPolicy := map[RequestType]bool{
		RequestTypeHotfix: true, RequestTypeSecurity: true, RequestTypePerformance: true, RequestTypeRefactor: true, RequestTypeOpsRequest: true,
	}
	if len(AllFlowTypes()) != 11 {
		t.Fatalf("expected 11 request types, got %d", len(AllFlowTypes()))
	}
	for _, typ := range AllFlowTypes() {
		if reg.HasPolicy(typ) != withPolicy[typ] {
			t.Errorf("%s: HasPolicy=%v, want %v", typ, reg.HasPolicy(typ), withPolicy[typ])
		}
		p := reg.PolicyFor(typ)
		if p == nil {
			t.Fatalf("%s: nil policy", typ)
		}
		_, isNoop := p.(noopPolicy)
		if isNoop == withPolicy[typ] {
			t.Errorf("%s: noop=%v but expected policy=%v", typ, isNoop, withPolicy[typ])
		}
	}
}

func TestNoopPolicy_ChangesNothing(t *testing.T) {
	p := noopPolicy{}
	if err := p.PlanPreconditions(context.Background(), Request{}, PlanProposal{}); err != nil {
		t.Fatal(err)
	}
	if g, err := p.PreExecutionGate(context.Background(), Request{}, TaskRef{}); g != nil || err != nil {
		t.Fatal("noop gate must be nil")
	}
	if v, _ := p.CompletionChecks(Request{}, nil); len(v) != 0 {
		t.Fatal("noop has no completion checks")
	}
	if f, _ := p.OnCompleted(Request{}); len(f) != 0 {
		t.Fatal("noop has no follow-ups")
	}
}

func TestImprovementPercent_BothDirections(t *testing.T) {
	cases := []struct {
		baseline, value float64
		dir             Direction
		want            float64
	}{
		{200, 150, LowerIsBetter, 25},
		{200, 250, LowerIsBetter, -25},
		{100, 130, HigherIsBetter, 30},
		{100, 80, HigherIsBetter, -20},
	}
	for _, tc := range cases {
		got, err := ImprovementPercent(tc.baseline, tc.value, tc.dir)
		if err != nil || got != tc.want {
			t.Errorf("%v: got %v %v, want %v", tc, got, err, tc.want)
		}
	}
}

func TestImprovementPercent_BaselineZero(t *testing.T) {
	if _, err := ImprovementPercent(0, 5, LowerIsBetter); !errors.Is(err, ErrBaselineZero) {
		t.Fatalf("got %v, want ErrBaselineZero", err)
	}
	if _, err := ImprovementPercent(1, 5, "sideways"); err == nil {
		t.Fatal("an unknown direction must be an error")
	}
}

const perfBaselineJSON = `{"metrics":[{"name":"p95","unit":"ms","direction":"lower_is_better","baseline":200,"target_change_percent":20},{"name":"rps","unit":"1/s","direction":"higher_is_better","baseline":100,"target_change_percent":10}],"method":"wrk"}`

func TestPerformancePolicy_Completion(t *testing.T) {
	cases := []struct {
		name   string
		after  string
		hasAft bool
		want   []CheckVerdictStatus
		hint   string
	}{
		{"both metrics reach target", `{"metrics":[{"name":"p95","value":150},{"name":"rps","value":120}]}`, true, []CheckVerdictStatus{CheckVerdictPassed, CheckVerdictPassed}, ""},
		{"one metric short", `{"metrics":[{"name":"p95","value":190},{"name":"rps","value":120}]}`, true, []CheckVerdictStatus{CheckVerdictFailed, CheckVerdictPassed}, "Hiệu năng chưa đạt: p95 5% < 20%"},
		{"exactly on target passes", `{"metrics":[{"name":"p95","value":160},{"name":"rps","value":110}]}`, true, []CheckVerdictStatus{CheckVerdictPassed, CheckVerdictPassed}, ""},
		{"no perf_after yet", ``, false, []CheckVerdictStatus{CheckVerdictMissing}, ""},
		{"one metric missing from perf_after", `{"metrics":[{"name":"p95","value":100}]}`, true, []CheckVerdictStatus{CheckVerdictPassed, CheckVerdictMissing}, "rps"},
		{"worse than baseline", `{"metrics":[{"name":"p95","value":300},{"name":"rps","value":50}]}`, true, []CheckVerdictStatus{CheckVerdictFailed, CheckVerdictFailed}, "p95"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checks := []RequestCheck{check(CheckPerfBaseline, CheckStatusPassed, CheckSourceManual, perfBaselineJSON, 0)}
			if tc.hasAft {
				// The declared status is ignored: an agent claiming "passed" cannot rescue short numbers.
				checks = append(checks, check(CheckPerfAfter, CheckStatusPassed, CheckSourceAgent, tc.after, 1))
			}
			got, err := performancePolicy{}.CompletionChecks(Request{}, checks)
			if err != nil || len(got) != len(tc.want) {
				t.Fatalf("got %v %v", got, err)
			}
			for i, v := range got {
				if v.Status != tc.want[i] {
					t.Errorf("verdict %d: got %s (%s), want %s", i, v.Status, v.Summary, tc.want[i])
				}
				if tc.hint != "" && i == 0 && !strings.Contains(v.Summary, tc.hint) && tc.name != "one metric missing from perf_after" {
					t.Errorf("summary %q lacks %q", v.Summary, tc.hint)
				}
			}
		})
	}
}

func TestPerformancePolicy_Completion_BaselineZeroFailsWithoutDividing(t *testing.T) {
	baseline := `{"metrics":[{"name":"errors","unit":"n","direction":"lower_is_better","baseline":0,"target_change_percent":10}],"method":"m"}`
	checks := []RequestCheck{
		check(CheckPerfBaseline, CheckStatusPassed, CheckSourceManual, baseline, 0),
		check(CheckPerfAfter, CheckStatusPassed, CheckSourceManual, `{"metrics":[{"name":"errors","value":0}]}`, 1),
	}
	got, err := performancePolicy{}.CompletionChecks(Request{}, checks)
	if err != nil || len(got) != 1 || got[0].Status != CheckVerdictFailed || !strings.Contains(got[0].Summary, "baseline bằng 0") {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestPerformancePolicy_Completion_NoBaselineIsMissing(t *testing.T) {
	got, _ := performancePolicy{}.CompletionChecks(Request{}, nil)
	if len(got) != 1 || got[0].Status != CheckVerdictMissing {
		t.Fatalf("got %v", got)
	}
}

func proposalOf(labels ...[]string) PlanProposal {
	p := PlanProposal{}
	for _, l := range labels {
		p.Tasks = append(p.Tasks, TaskProposal{Title: strings.Join(l, ","), Labels: l})
	}
	return p
}

func TestPerformancePolicy_PlanPreconditions_MissingBaseline(t *testing.T) {
	plan := proposalOf([]string{"check:baseline"}, nil, []string{"check:after"})
	for name, checks := range map[string]fakeChecks{
		"none":   {},
		"failed": {CheckPerfBaseline: check(CheckPerfBaseline, CheckStatusFailed, CheckSourceManual, perfBaselineJSON, 0)},
	} {
		if err := (performancePolicy{checks: checks}).PlanPreconditions(context.Background(), Request{ID: "r"}, plan); codeOf(err) != "REQUEST_PERF_BASELINE_MISSING" {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if err := (performancePolicy{}).PlanPreconditions(context.Background(), Request{ID: "r"}, plan); codeOf(err) != "REQUEST_PERF_BASELINE_MISSING" {
		t.Errorf("no reader wired must fail closed, got %v", err)
	}
}

func TestPerformancePolicy_PlanPreconditions_MissingCheckTasks(t *testing.T) {
	ok := fakeChecks{CheckPerfBaseline: check(CheckPerfBaseline, CheckStatusPassed, CheckSourceManual, perfBaselineJSON, 0)}
	p := performancePolicy{checks: ok}
	bad := map[string]PlanProposal{
		"no check tasks":     proposalOf(nil, nil),
		"baseline not first": proposalOf(nil, []string{"check:baseline"}, []string{"check:after"}),
		"after not last":     proposalOf([]string{"check:baseline"}, []string{"check:after"}, nil),
		"single task":        proposalOf([]string{"check:baseline", "check:after"}),
	}
	for name, plan := range bad {
		if err := p.PlanPreconditions(context.Background(), Request{ID: "r"}, plan); codeOf(err) != "REQUEST_PLAN_PERF_CHECK_TASKS_MISSING" {
			t.Errorf("%s: got %v", name, err)
		}
	}
	good := proposalOf([]string{"check:baseline"}, nil, []string{"check:after"})
	if err := p.PlanPreconditions(context.Background(), Request{ID: "r"}, good); err != nil {
		t.Errorf("valid plan rejected: %v", err)
	}
	// Tasks inside phases count in order after the top-level ones.
	phased := PlanProposal{Phases: []PhaseProposal{{Tasks: []TaskProposal{{Labels: []string{"check:baseline"}}, {Labels: []string{"check:after"}}}}}}
	if err := p.PlanPreconditions(context.Background(), Request{ID: "r"}, phased); err != nil {
		t.Errorf("phased plan rejected: %v", err)
	}
}

func TestRefactorPolicy_Completion(t *testing.T) {
	before := check(CheckTestsBefore, CheckStatusPassed, CheckSourceAgent, `{"total":40,"passed":40,"failed":0,"command":"go test"}`, 0)
	cases := []struct {
		name   string
		after  string
		checks func([]RequestCheck) []RequestCheck
		want   CheckVerdictStatus
		hint   string
	}{
		{"all three satisfied", `{"total":40,"passed":40,"failed":0,"command":"x","tests_modified":false}`, nil, CheckVerdictPassed, ""},
		{"more tests than before", `{"total":55,"passed":55,"failed":0,"command":"x","tests_modified":false}`, nil, CheckVerdictPassed, ""},
		{"tests still red", `{"total":40,"passed":38,"failed":2,"command":"x","tests_modified":false}`, nil, CheckVerdictFailed, "2 test còn đỏ"},
		{"test count dropped", `{"total":30,"passed":30,"failed":0,"command":"x","tests_modified":false}`, nil, CheckVerdictFailed, "giảm từ 40 xuống 30"},
		{"tests modified", `{"total":40,"passed":40,"failed":0,"command":"x","tests_modified":true}`, nil, CheckVerdictFailed, "test bị sửa"},
		{"all violations at once", `{"total":10,"passed":5,"failed":5,"command":"x","tests_modified":true}`, nil, CheckVerdictFailed, "chưa được xác minh"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checks := []RequestCheck{before, check(CheckTestsAfter, CheckStatusPassed, CheckSourceAgent, tc.after, 1)}
			got, err := refactorPolicy{}.CompletionChecks(Request{}, checks)
			if err != nil || len(got) != 1 || got[0].Status != tc.want {
				t.Fatalf("got %v %v", got, err)
			}
			if tc.hint != "" && !strings.Contains(got[0].Summary, tc.hint) {
				t.Errorf("summary %q lacks %q", got[0].Summary, tc.hint)
			}
			if !strings.Contains(got[0].Summary, "tests_modified do agent khai") {
				t.Errorf("summary must say tests_modified is self-declared: %q", got[0].Summary)
			}
		})
	}
	got, _ := refactorPolicy{}.CompletionChecks(Request{}, []RequestCheck{check(CheckTestsAfter, CheckStatusPassed, CheckSourceAgent, `{"total":1,"passed":1,"failed":0,"command":"x","tests_modified":false}`, 1)})
	if len(got) != 1 || got[0].Kind != CheckTestsBefore || got[0].Status != CheckVerdictMissing {
		t.Errorf("missing tests_before: got %v", got)
	}
	got, _ = refactorPolicy{}.CompletionChecks(Request{}, nil)
	if len(got) != 2 {
		t.Errorf("both missing: got %v", got)
	}
}

func TestRefactorPolicy_PlanPreconditions_MissingCheckTasks(t *testing.T) {
	for name, plan := range map[string]PlanProposal{
		"none":      proposalOf(nil, nil),
		"swapped":   proposalOf([]string{"check:tests_after"}, []string{"check:tests_before"}),
		"too short": proposalOf([]string{"check:tests_before"}),
	} {
		if err := (refactorPolicy{}).PlanPreconditions(context.Background(), Request{}, plan); codeOf(err) != "REQUEST_PLAN_TEST_CHECK_TASKS_MISSING" {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if err := (refactorPolicy{}).PlanPreconditions(context.Background(), Request{}, proposalOf([]string{"check:tests_before"}, nil, []string{"check:tests_after"})); err != nil {
		t.Errorf("valid plan rejected: %v", err)
	}
}

func TestSecurityPolicy_Completion(t *testing.T) {
	cases := []struct {
		name   string
		checks []RequestCheck
		want   CheckVerdictStatus
	}{
		{"no re-check yet", nil, CheckVerdictMissing},
		{"passed by a person", []RequestCheck{check(CheckSecurityRecheck, CheckStatusPassed, CheckSourceManual, `{}`, 0)}, CheckVerdictPassed},
		{"passed and verified by Orca", []RequestCheck{check(CheckSecurityRecheck, CheckStatusPassed, CheckSourceOrcaVerified, `{}`, 0)}, CheckVerdictPassed},
		{"agent says passed: not enough alone", []RequestCheck{check(CheckSecurityRecheck, CheckStatusPassed, CheckSourceAgent, `{}`, 0)}, CheckVerdictMissing},
		{"agent says failed: honoured", []RequestCheck{check(CheckSecurityRecheck, CheckStatusFailed, CheckSourceAgent, `{}`, 0)}, CheckVerdictFailed},
		{"newest failed beats older passed", []RequestCheck{check(CheckSecurityRecheck, CheckStatusPassed, CheckSourceManual, `{}`, 0), check(CheckSecurityRecheck, CheckStatusFailed, CheckSourceManual, `{}`, 1)}, CheckVerdictFailed},
		{"person confirms after the agent", []RequestCheck{check(CheckSecurityRecheck, CheckStatusPassed, CheckSourceAgent, `{}`, 0), check(CheckSecurityRecheck, CheckStatusPassed, CheckSourceManual, `{}`, 1)}, CheckVerdictPassed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := securityPolicy{}.CompletionChecks(Request{}, tc.checks)
			if err != nil || len(got) != 1 || got[0].Status != tc.want || got[0].Kind != CheckSecurityRecheck {
				t.Fatalf("got %v %v, want %s", got, err, tc.want)
			}
		})
	}
}

func TestHotfixPolicy_NoGateDuringExecution(t *testing.T) {
	g, err := hotfixPolicy{}.PreExecutionGate(context.Background(), Request{}, TaskRef{ID: "t", Labels: []string{PolicyLabelGatePreDeploy}})
	if g != nil || err != nil {
		t.Fatalf("hotfix gates at StartGate only, got %v %v", g, err)
	}
	if v, _ := (hotfixPolicy{}).CompletionChecks(Request{}, nil); len(v) != 0 {
		t.Fatal("hotfix has no completion checks")
	}
}

func TestHotfixPolicy_PlanShape(t *testing.T) {
	one := PlanProposal{Tasks: []TaskProposal{{Title: "fix"}}}
	if err := (hotfixPolicy{}).PlanPreconditions(context.Background(), Request{}, one); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]PlanProposal{
		"two tasks": {Tasks: []TaskProposal{{}, {}}}, "no task": {}, "phase": {Phases: []PhaseProposal{{Tasks: []TaskProposal{{}}}}, Tasks: []TaskProposal{{}}},
	} {
		if err := (hotfixPolicy{}).PlanPreconditions(context.Background(), Request{}, p); codeOf(err) != "REQUEST_HOTFIX_PLAN_SHAPE" {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestHotfixOnCompleted_TwoFollowUps_StableIDs(t *testing.T) {
	req := Request{ID: "req-1", Number: 7, Title: "login 500"}
	a, _ := hotfixPolicy{}.OnCompleted(req)
	b, _ := hotfixPolicy{}.OnCompleted(req)
	if len(a) != 2 || a[0].TypeHint != RequestTypeBug || a[1].TypeHint != RequestTypeTask {
		t.Fatalf("got %+v", a)
	}
	for i := range a {
		if a[i].ClientRequestID != b[i].ClientRequestID || a[i].LinkReason != LinkReasonFollowupHotfix {
			t.Errorf("follow-up %d must be repeatable: %+v vs %+v", i, a[i], b[i])
		}
	}
	if a[0].ClientRequestID != "hotfix-followup:req-1:bug" || a[1].ClientRequestID != "hotfix-followup:req-1:task" {
		t.Errorf("unexpected client request ids %q %q", a[0].ClientRequestID, a[1].ClientRequestID)
	}
	if !strings.Contains(a[0].Title, "login 500") || !strings.Contains(a[1].Title, "login 500") {
		t.Errorf("titles must name the hotfix: %q %q", a[0].Title, a[1].Title)
	}
}

func opsTask(title string, labels []string, irreversible bool, deps ...int) TaskProposal {
	return TaskProposal{Title: title, Labels: labels, Irreversible: irreversible, DependsOnIndices: deps}
}

func TestCheckRunbook_Table(t *testing.T) {
	cases := []struct {
		name string
		plan PlanProposal
		code string
	}{
		{"no rollback task at all", PlanProposal{Tasks: []TaskProposal{opsTask("migrate", nil, false)}}, "REQUEST_RUNBOOK_ROLLBACK_MISSING"},
		{"irreversible step with no rollback after it", PlanProposal{Tasks: []TaskProposal{opsTask("drop", nil, true), opsTask("undo", []string{"rollback"}, false)}}, "REQUEST_RUNBOOK_ROLLBACK_MISSING"},
		{"rollback placed before the step, not after", PlanProposal{Tasks: []TaskProposal{opsTask("undo", []string{"rollback"}, false), opsTask("drop", nil, true, 0)}}, "REQUEST_RUNBOOK_ROLLBACK_MISSING"},
		{"rollback_note stands in for a rollback task", PlanProposal{Tasks: []TaskProposal{
			{Title: "drop", Irreversible: true, Description: "Drop the table\nrollback_note: restore from snapshot 12"},
			opsTask("undo other", []string{"rollback"}, false),
		}}, ""},
		{"empty rollback_note does not count", PlanProposal{Tasks: []TaskProposal{
			{Title: "drop", Irreversible: true, Description: "rollback_note:   "}, opsTask("undo other", []string{"rollback"}, false),
		}}, "REQUEST_RUNBOOK_ROLLBACK_MISSING"},
		{"valid: rollback depends on the step", PlanProposal{Tasks: []TaskProposal{opsTask("drop", nil, true), opsTask("undo", []string{"rollback"}, false, 0)}}, ""},
		{"valid: rollback reaches the step through another task", PlanProposal{Tasks: []TaskProposal{opsTask("drop", nil, true), opsTask("verify", nil, false, 0), opsTask("undo", []string{"rollback"}, false, 1)}}, ""},
		{"valid: all reversible, one rollback", PlanProposal{Tasks: []TaskProposal{opsTask("a", nil, false), opsTask("undo", []string{"rollback"}, false)}}, ""},
		{"valid across phases", PlanProposal{Phases: []PhaseProposal{{Tasks: []TaskProposal{opsTask("drop", nil, true), opsTask("undo", []string{"rollback"}, false, 0)}}}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := codeOf(CheckRunbook(tc.plan)); got != tc.code {
				t.Fatalf("got %q, want %q", got, tc.code)
			}
		})
	}
}

func TestEffectiveLabels_IrreversibleGetsGate(t *testing.T) {
	got := EffectiveLabels(TaskProposal{Labels: []string{"x"}, Irreversible: true})
	if !containsString(got, PolicyLabelGatePreDeploy) || !containsString(got, "x") {
		t.Fatalf("got %v", got)
	}
	again := EffectiveLabels(TaskProposal{Labels: []string{PolicyLabelGatePreDeploy}, Irreversible: true})
	if len(again) != 1 {
		t.Fatalf("an existing gate label must not be duplicated: %v", again)
	}
}

func TestOpsPolicy_PreExecutionGate(t *testing.T) {
	ctx := context.Background()
	labelled := TaskRef{ID: "t1", Labels: []string{PolicyLabelGatePreDeploy}}
	approvedRow := &Approval{Status: ApprovalStatusApproved}
	cases := []struct {
		name      string
		approvals ApprovalLookup
		task      TaskRef
		want      bool
	}{
		{"labelled without approval", fakeApprovals{}, labelled, true},
		{"labelled with a pending approval", fakeApprovals{"pre_deploy/t1": {Status: ApprovalStatusPending}}, labelled, true},
		{"labelled with a rejected approval", fakeApprovals{"pre_deploy/t1": {Status: ApprovalStatusRejected}}, labelled, true},
		{"labelled and approved", fakeApprovals{"pre_deploy/t1": approvedRow}, labelled, false},
		{"unlabelled task runs", fakeApprovals{}, TaskRef{ID: "t2"}, false},
		{"labelled and no lookup wired: fail closed", nil, labelled, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, err := opsRequestPolicy{approvals: tc.approvals}.PreExecutionGate(ctx, Request{ID: "r"}, tc.task)
			if err != nil || (g != nil) != tc.want {
				t.Fatalf("got %v %v, want gate=%v", g, err, tc.want)
			}
			if g != nil && (g.SubjectType != SubjectPreDeploy || g.SubjectID != tc.task.ID) {
				t.Errorf("wrong requirement %+v", g)
			}
		})
	}
}

func TestOpsPolicy_Completion_RequiresOpsResult(t *testing.T) {
	p := opsRequestPolicy{}
	if got, _ := p.CompletionChecks(Request{}, nil); got[0].Status != CheckVerdictMissing {
		t.Errorf("no ops_result must wait, got %v", got)
	}
	if got, _ := p.CompletionChecks(Request{}, []RequestCheck{check(CheckOpsResult, CheckStatusPassed, CheckSourceManual, `{}`, 0)}); got[0].Status != CheckVerdictPassed {
		t.Errorf("a person's passed ops_result completes, got %v", got)
	}
	if got, _ := p.CompletionChecks(Request{}, []RequestCheck{check(CheckOpsResult, CheckStatusFailed, CheckSourceManual, `{}`, 0)}); got[0].Status != CheckVerdictFailed {
		t.Errorf("a failed ops_result returns the request, got %v", got)
	}
}

func TestOpsPolicy_FailureHint_NamesRollbackAfterTheStep(t *testing.T) {
	all := []TaskRef{
		{ID: "a", Title: "drop column"},
		{ID: "b", Title: "restore column", Labels: []string{"rollback"}, DependsOn: []string{"a"}},
		{ID: "c", Title: "unrelated rollback", Labels: []string{"rollback"}},
	}
	hint := opsRequestPolicy{}.FailureHint(Request{}, all[0], all)
	if !strings.Contains(hint, "restore column") || strings.Contains(hint, "unrelated") {
		t.Fatalf("hint %q should name only the rollback that follows the failed step", hint)
	}
	fallback := opsRequestPolicy{}.FailureHint(Request{}, TaskRef{ID: "z"}, all)
	if !strings.Contains(fallback, "restore column") || !strings.Contains(fallback, "unrelated rollback") {
		t.Fatalf("without a dependency every rollback task is named, got %q", fallback)
	}
	if (opsRequestPolicy{}).FailureHint(Request{}, all[0], all[:1]) != "" {
		t.Fatal("no rollback task means no hint")
	}
}
