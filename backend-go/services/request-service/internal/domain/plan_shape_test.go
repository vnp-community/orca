package domain

import "testing"

func TestPlanShapeFor_EveryTypeAndSize(t *testing.T) {
	cases := []struct {
		typ  RequestType
		size RequestSize
		want PlanShape
	}{
		{RequestTypeChangeRequest, RequestSizeS, ShapePlanPhases},
		{RequestTypeChangeRequest, RequestSizeM, ShapePlanPhases},
		{RequestTypeChangeRequest, RequestSizeL, ShapePlanPhases},
		{RequestTypeBug, RequestSizeS, ShapePlanTasks},
		{RequestTypeBug, RequestSizeM, ShapePlanTasks},
		{RequestTypeBug, RequestSizeL, ShapePlanPhases},
		{RequestTypeRefactor, RequestSizeS, ShapePlanTasks},
		{RequestTypeRefactor, RequestSizeL, ShapePlanPhases},
		{RequestTypeSecurity, RequestSizeL, ShapePlanTasks},
		{RequestTypePerformance, RequestSizeL, ShapePlanTasks},
		{RequestTypeOpsRequest, RequestSizeL, ShapePlanTasks},
		{RequestTypeTask, RequestSizeS, ShapeTaskListShell},
		{RequestTypeDocs, RequestSizeM, ShapeTaskListShell},
		{RequestTypeHotfix, RequestSizeS, ShapeSingleTask},
		{RequestTypeSpike, RequestSizeM, ShapeNone},
		{RequestTypeQuestion, RequestSizeS, ShapeNone},
	}
	for _, c := range cases {
		flow, err := FlowFor(c.typ)
		if err != nil {
			t.Fatal(err)
		}
		if got := PlanShapeFor(flow, c.size); got != c.want {
			t.Errorf("%s/%s: got %s want %s", c.typ, c.size, got, c.want)
		}
	}
}

// Every type of the flow registry must have a defined shape, so a new type cannot silently plan as "none".
func TestPlanShapeFor_CoversEveryRegisteredType(t *testing.T) {
	for typ := range flowDefinitions {
		flow, _ := FlowFor(typ)
		if flow.PlanKind == PlanPlan || flow.PlanKind == PlanTaskList || flow.PlanKind == PlanSingleTask {
			if PlanShapeFor(flow, RequestSizeM) == ShapeNone {
				t.Errorf("%s has PlanKind %s but shape none", typ, flow.PlanKind)
			}
		}
	}
}
