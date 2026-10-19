package domain

// PlanShape is the tree a Request type gets, derived from its FlowDefinition so the type table lives in one place.
type PlanShape int

const (
	// ShapeNone: the type has no Plan (spike, question); planning does not apply.
	ShapeNone PlanShape = iota
	// ShapePlanPhases: Plan -> Phase -> task.
	ShapePlanPhases
	// ShapePlanTasks: Plan -> task.
	ShapePlanTasks
	// ShapeTaskListShell: a task-typed "plan" shell holding the tasks, so the task_list approval has a task id as its subject.
	ShapeTaskListShell
	// ShapeSingleTask: one task and no Plan (hotfix).
	ShapeSingleTask
)

func (s PlanShape) String() string {
	switch s {
	case ShapePlanPhases:
		return "plan_phases"
	case ShapePlanTasks:
		return "plan_tasks"
	case ShapeTaskListShell:
		return "task_list"
	case ShapeSingleTask:
		return "single_task"
	}
	return "none"
}

// AllowsPhases reports whether the proposal contains Phases: a shape either always has them or never does.
func (s PlanShape) AllowsPhases() bool { return s == ShapePlanPhases }

// PlanShapeFor decides the tree for a Request of this flow and size.
func PlanShapeFor(f FlowDefinition, size RequestSize) PlanShape {
	switch f.PlanKind {
	case PlanPlan:
		if f.PhasesFor(size) {
			return ShapePlanPhases
		}
		return ShapePlanTasks
	case PlanTaskList:
		return ShapeTaskListShell
	case PlanSingleTask:
		return ShapeSingleTask
	}
	return ShapeNone
}
