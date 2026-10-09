package domain

import "testing"

func TestPlanLimitsFromEnv(t *testing.T) {
	got, warns := PlanLimitsFromEnv(envOf(nil))
	if got != DefaultPlanLimits || len(warns) != 0 {
		t.Fatalf("defaults expected, got %+v %v", got, warns)
	}
	got, warns = PlanLimitsFromEnv(envOf(map[string]string{"REQUEST_PLAN_MAX_PHASES": "3", "REQUEST_PLAN_MAX_TASKS_TOTAL": "oops", "REQUEST_PLAN_MAX_TITLE_CHARS": "0"}))
	if got.MaxPhases != 3 || got.MaxTasksTotal != DefaultPlanLimits.MaxTasksTotal || got.MaxTitleRunes != DefaultPlanLimits.MaxTitleRunes {
		t.Errorf("override or fallback wrong: %+v", got)
	}
	if len(warns) != 2 {
		t.Errorf("want 2 warnings, got %v", warns)
	}
}
