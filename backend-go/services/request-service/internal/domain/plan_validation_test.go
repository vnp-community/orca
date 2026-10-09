package domain

import (
	"strings"
	"testing"
)

func okTask(title string, deps ...int) TaskProposal {
	return TaskProposal{Title: title, TaskType: "task", DependsOnIndices: deps}
}

func planOf(tasks ...TaskProposal) PlanProposal { return PlanProposal{Title: "Plan", Tasks: tasks} }

func phased(phases ...PhaseProposal) PlanProposal { return PlanProposal{Title: "Plan", Phases: phases} }

func phase(title string, tasks ...TaskProposal) PhaseProposal {
	return PhaseProposal{Title: title, Tasks: tasks}
}

func TestValidatePlanProposal(t *testing.T) {
	lim := PlanLimits{MaxPhases: 2, MaxTasksPerContainer: 3, MaxTasksTotal: 4, MaxTitleRunes: 20}
	regression := okTask("add regression test")
	regression.Labels = []string{PlanLabelTestRegression}
	cases := []struct {
		name  string
		typ   RequestType
		shape PlanShape
		plan  PlanProposal
		want  string // empty = valid
	}{
		{"plain plan with tasks", RequestTypePerformance, ShapePlanTasks, planOf(okTask("a"), okTask("b", 0)), ""},
		{"phased plan", RequestTypeChangeRequest, ShapePlanPhases, phased(phase("P1", okTask("a")), phase("P2", okTask("b"))), ""},
		{"not applicable", RequestTypeSpike, ShapeNone, planOf(okTask("a")), "REQUEST_PLAN_NOT_APPLICABLE"},
		{"phases where none allowed", RequestTypeTask, ShapePlanTasks, phased(phase("P", okTask("a"))), "REQUEST_PLAN_PHASES_NOT_ALLOWED"},
		{"phases required but absent", RequestTypeChangeRequest, ShapePlanPhases, planOf(okTask("a")), "REQUEST_PLAN_MIXED_CHILDREN"},
		{"phases required, nothing given", RequestTypeChangeRequest, ShapePlanPhases, PlanProposal{Title: "Plan"}, "REQUEST_PLAN_PHASES_REQUIRED"},
		{"mixed phases and tasks", RequestTypeChangeRequest, ShapePlanPhases, PlanProposal{Title: "Plan", Phases: []PhaseProposal{phase("P", okTask("a"))}, Tasks: []TaskProposal{okTask("b")}}, "REQUEST_PLAN_MIXED_CHILDREN"},
		{"no tasks", RequestTypeTask, ShapeTaskListShell, PlanProposal{Title: "Plan"}, "REQUEST_PLAN_EMPTY"},
		{"empty phase", RequestTypeChangeRequest, ShapePlanPhases, phased(phase("P")), "REQUEST_PLAN_EMPTY"},
		{"too many phases", RequestTypeChangeRequest, ShapePlanPhases, phased(phase("a", okTask("x")), phase("b", okTask("x")), phase("c", okTask("x"))), "REQUEST_PLAN_TOO_LARGE"},
		{"too many tasks in one container", RequestTypeTask, ShapeTaskListShell, planOf(okTask("a"), okTask("b"), okTask("c"), okTask("d")), "REQUEST_PLAN_TOO_LARGE"},
		{"too many tasks overall", RequestTypeChangeRequest, ShapePlanPhases, phased(phase("a", okTask("1"), okTask("2"), okTask("3")), phase("b", okTask("4"), okTask("5"))), "REQUEST_PLAN_TOO_LARGE"},
		{"dependency out of range", RequestTypeTask, ShapeTaskListShell, planOf(okTask("a", 5)), "REQUEST_PLAN_INVALID_DEPENDENCY"},
		{"negative dependency", RequestTypeTask, ShapeTaskListShell, planOf(okTask("a", -1)), "REQUEST_PLAN_INVALID_DEPENDENCY"},
		{"self dependency", RequestTypeTask, ShapeTaskListShell, planOf(okTask("a", 0)), "REQUEST_PLAN_INVALID_DEPENDENCY"},
		{"task cycle", RequestTypeTask, ShapeTaskListShell, planOf(okTask("a", 1), okTask("b", 0)), "REQUEST_PLAN_INVALID_DEPENDENCY"},
		{"phase cycle", RequestTypeChangeRequest, ShapePlanPhases, phased(
			PhaseProposal{Title: "A", Tasks: []TaskProposal{okTask("x")}, DependsOnPhaseIndices: []int{1}},
			PhaseProposal{Title: "B", Tasks: []TaskProposal{okTask("y")}, DependsOnPhaseIndices: []int{0}}), "REQUEST_PLAN_INVALID_DEPENDENCY"},
		{"duplicate dependency edge is tolerated", RequestTypeTask, ShapeTaskListShell, planOf(okTask("a"), okTask("b", 0, 0)), ""},
		{"bad task type", RequestTypeTask, ShapeTaskListShell, planOf(TaskProposal{Title: "a", TaskType: "plan"}), "REQUEST_PLAN_INVALID_TASK_TYPE"},
		{"blank title", RequestTypeTask, ShapeTaskListShell, planOf(okTask("  ")), "REQUEST_PLAN_INVALID_TITLE"},
		{"long title", RequestTypeTask, ShapeTaskListShell, planOf(okTask(strings.Repeat("x", 21))), "REQUEST_PLAN_INVALID_TITLE"},
		{"negative hours", RequestTypeTask, ShapeTaskListShell, planOf(TaskProposal{Title: "a", TaskType: "task", EstimatedHours: -1}), "REQUEST_PLAN_INVALID_FIELD"},
		{"bug without regression test", RequestTypeBug, ShapePlanTasks, planOf(okTask("fix")), "REQUEST_PLAN_REGRESSION_TEST_MISSING"},
		{"bug with regression test", RequestTypeBug, ShapePlanTasks, planOf(okTask("fix"), regression), ""},
		{"security without regression test", RequestTypeSecurity, ShapePlanTasks, planOf(okTask("fix")), "REQUEST_PLAN_REGRESSION_TEST_MISSING"},
	}
	for _, c := range cases {
		err := ValidatePlanProposal(c.typ, c.shape, lim, NormalizePlanProposal(c.plan))
		if got := codeOf(err); got != c.want {
			t.Errorf("%s: got %q (%v), want %q", c.name, got, err, c.want)
		}
	}
}

func TestNormalizePlanProposal(t *testing.T) {
	got := NormalizePlanProposal(PlanProposal{Title: "  P ", Tasks: []TaskProposal{{Title: " a ", Labels: []string{" Test:Regression ", "test:regression", ""}}}})
	task := got.Tasks[0]
	if got.Title != "P" || task.Title != "a" || task.TaskType != "task" {
		t.Errorf("not trimmed or defaulted: %+v", got)
	}
	if len(task.Labels) != 1 || task.Labels[0] != PlanLabelTestRegression {
		t.Errorf("labels should be lower-cased and de-duplicated: %v", task.Labels)
	}
}
