//go:build e2e

package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// flowScenarios are E01 to E12 of CR-REQ-025 section 2.3: one per flow family and type, built from the flow
// registry so the expected statuses cannot drift from it.
func flowScenarios() []Scenario {
	row := func(id string, g Group, typ domain.RequestType, size domain.RequestSize) Scenario {
		return newFlowScenario(id, g, typ, size)
	}
	hotfix := row("E10", GroupHotfix, domain.RequestTypeHotfix, domain.RequestSizeS)
	hotfix.Stages = append(hotfix.Stages, Stage{
		Name: "the_fix_task_was_made_by_the_diagnosis_and_there_is_no_plan",
		Run: func(t *testing.T, w *world) {
			var fixes []string
			for _, task := range theStack.tasks.TasksOfRequest(w.id()) {
				if task.GetParentId() != "" || task.GetTaskType() == "plan" {
					t.Fatalf("a hotfix has no Plan and no container, found %s %q under %q", task.GetTaskType(), task.GetTitle(), task.GetParentId())
				}
				fixes = append(fixes, task.GetId())
			}
			if len(fixes) != 1 {
				t.Fatalf("a hotfix has exactly one fix task, found %d", len(fixes))
			}
			if got := w.k.get(w.id()); got.GetPlanTaskId() != "" {
				t.Fatalf("plan_task_id = %q, want empty", got.GetPlanTaskId())
			}
		},
	})
	return []Scenario{
		row("E01", GroupFull, domain.RequestTypeChangeRequest, domain.RequestSizeM),
		row("E02", GroupFull, domain.RequestTypeBug, domain.RequestSizeL),
		row("E03", GroupFull, domain.RequestTypeRefactor, domain.RequestSizeL),
		row("E04", GroupShort, domain.RequestTypeBug, domain.RequestSizeS),
		row("E05", GroupShort, domain.RequestTypeTask, domain.RequestSizeS),
		row("E06", GroupShort, domain.RequestTypeDocs, domain.RequestSizeS),
		row("E07", GroupShort, domain.RequestTypePerformance, domain.RequestSizeM),
		row("E08", GroupShort, domain.RequestTypeSecurity, domain.RequestSizeM),
		row("E09", GroupShort, domain.RequestTypeOpsRequest, domain.RequestSizeM),
		hotfix,
		row("E11", GroupSpike, domain.RequestTypeSpike, domain.RequestSizeM),
		row("E12", GroupSpike, domain.RequestTypeQuestion, domain.RequestSizeS),
	}
}

func newFlowScenario(id string, g Group, typ domain.RequestType, size domain.RequestSize) Scenario {
	flow, err := domain.FlowFor(typ)
	if err != nil {
		panic(err)
	}
	afterConfirm, err := domain.NextStatus(flow, size, domain.RequestStatusAwaitingTypeConfirmation, domain.TriggerTypeConfirmed)
	if err != nil {
		panic(err)
	}
	sc := Scenario{ID: id, Group: g, Type: string(typ), Size: string(size)}
	sc.Stages = append(sc.Stages,
		Stage{
			Name: "create_and_classify",
			Run: func(t *testing.T, w *world) {
				w.request = w.k.create("e2e "+id, w.typ, w.size)
			},
			WantStatus:    "awaiting_type_confirmation",
			WantEvents:    []string{domain.SubjectRequestCreated, domain.SubjectRequestClassified},
			WantAudits:    []string{domain.ActionRequestCreate},
			WantApprovals: []string{"request_type"},
		},
		Stage{
			Name: "human_confirms_type",
			Run: func(t *testing.T, w *world) {
				got := w.request
				if got.GetType() != w.typ || got.GetSize() != w.size {
					t.Fatalf("the AI proposal was %s/%s, the stub was told %s/%s", got.GetType(), got.GetSize(), w.typ, w.size)
				}
				w.request = w.k.confirm(w.request, w.typ, w.size)
			},
			WantStatus: string(afterConfirm),
			WantEvents: []string{domain.SubjectRequestTypeConfirmed},
			WantAudits: []string{domain.ActionRequestTypeConfirm},
		},
	)
	return appendRemainingPath(sc, flow, size, afterConfirm)
}

// pathStep is the next move of a Request that waits in a status: the trigger it fires and the people or agents who make it.
type pathStep struct {
	trigger domain.Trigger
	run     func(w *world)
}

var pathSteps = map[domain.RequestStatus]pathStep{
	domain.RequestStatusAnalyzing:                {domain.TriggerAnalysisReady, (*world).analyze},
	domain.RequestStatusAwaitingAnalysisApproval: {domain.TriggerAnalysisApproved, (*world).approveAnalysis},
	domain.RequestStatusPlanning:                 {domain.TriggerPlanReady, (*world).plan},
	domain.RequestStatusAwaitingPlanApproval:     {domain.TriggerPlanApproved, (*world).approvePlan},
	domain.RequestStatusExecuting:                {domain.TriggerExecutionFinished, (*world).execute},
}

// appendRemainingPath adds the stages from the first status after type confirmation to completed, derived from the
// transition table, so the expected status of each stage cannot drift from the registry.
func appendRemainingPath(sc Scenario, flow domain.FlowDefinition, size domain.RequestSize, from domain.RequestStatus) Scenario {
	for from != domain.RequestStatusCompleted {
		step, ok := pathSteps[from]
		if !ok {
			panic(fmt.Sprintf("%s: no step for status %s", sc.ID, from))
		}
		to, err := domain.NextStatus(flow, size, from, step.trigger)
		if err != nil {
			panic(fmt.Sprintf("%s: %v", sc.ID, err))
		}
		st := Stage{
			Name:       fmt.Sprintf("%s_to_%s", from, to),
			Run:        func(t *testing.T, w *world) { step.run(w) },
			WantStatus: string(to),
		}
		switch to {
		case domain.RequestStatusAwaitingAnalysisApproval:
			st.WantApprovals = []string{string(flow.AnalysisGate)}
		case domain.RequestStatusAwaitingPlanApproval:
			st.WantApprovals = []string{string(flow.StartGate)}
		case domain.RequestStatusCompleted:
			st.WantEvents, st.StatusTimeout = []string{domain.SubjectRequestCompleted}, executionBudget+30*time.Second
		}
		sc.Stages = append(sc.Stages, st)
		from = to
	}
	return sc
}
