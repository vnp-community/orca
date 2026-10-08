//go:build e2e

package e2e

import (
	"fmt"
	"testing"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/grpc/codes"
)

// flowScenarios are E01 to E12 of CR-REQ-025 section 2.3: one per flow family and type, built from the flow
// registry so the expected statuses cannot drift from it.
func flowScenarios() []Scenario {
	row := func(id string, g Group, typ domain.RequestType, size domain.RequestSize) Scenario {
		return newFlowScenario(id, g, typ, size)
	}
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
		row("E10", GroupHotfix, domain.RequestTypeHotfix, domain.RequestSizeS),
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

// pathStep is the next move of a Request that no human or stub can make before the delivering task exists.
type pathStep struct {
	trigger domain.Trigger
	probe   string // RPC that starts the move
	needs   string
}

var pathSteps = map[domain.RequestStatus]pathStep{
	domain.RequestStatusAnalyzing:                {domain.TriggerAnalysisReady, "GenerateSolution", "CR-REQ-007/008 (GenerateSolution, analysis runs)"},
	domain.RequestStatusAwaitingAnalysisApproval: {domain.TriggerAnalysisApproved, "ChooseSolutionOption", "CR-REQ-007/009 (ChooseSolutionOption, analysis approval)"},
	domain.RequestStatusPlanning:                 {domain.TriggerPlanReady, "GeneratePlan", "CR-REQ-012 (GeneratePlan)"},
	domain.RequestStatusAwaitingPlanApproval:     {domain.TriggerPlanApproved, "CommitPlan", "CR-REQ-012 (CommitPlan, plan approval)"},
	domain.RequestStatusExecuting:                {domain.TriggerExecutionFinished, "ReportTaskOutcome", "CR-REQ-013 (StartPhase, ReportTaskOutcome)"},
}

// appendRemainingPath adds the stages from the first status after type confirmation to completed, derived from the
// transition table. They wait on RPCs other tasks deliver; see Stage.Needs.
func appendRemainingPath(sc Scenario, flow domain.FlowDefinition, size domain.RequestSize, from domain.RequestStatus) Scenario {
	for from != domain.RequestStatusCompleted {
		step, ok := pathSteps[from]
		if !ok {
			break
		}
		to, err := domain.NextStatus(flow, size, from, step.trigger)
		if err != nil {
			panic(fmt.Sprintf("%s: %v", sc.ID, err))
		}
		sc.Stages = append(sc.Stages, Stage{
			Name:       fmt.Sprintf("%s_to_%s", from, to),
			Needs:      step.needs,
			Run:        func(t *testing.T, w *world) { probeRPC(t, w, step.probe) },
			WantStatus: string(to),
		})
		from = to
	}
	return sc
}

// probeRPC calls the RPC that opens a stage with an empty message. Unimplemented blocks the stage with the
// delivering task named; any other answer means the RPC exists now and the stage still has to be written.
func probeRPC(t *testing.T, w *world, rpc string) {
	t.Helper()
	k := w.k
	var err error
	switch rpc {
	case "GenerateSolution":
		_, err = k.req.GenerateSolution(k.asReporter(), &requestv1.GenerateSolutionRequest{})
	case "ChooseSolutionOption":
		_, err = k.req.ChooseSolutionOption(k.asReporter(), &requestv1.ChooseSolutionOptionRequest{})
	case "GeneratePlan":
		_, err = k.req.GeneratePlan(k.asReporter(), &requestv1.GeneratePlanRequest{})
	case "CommitPlan":
		_, err = k.req.CommitPlan(k.asReporter(), &requestv1.CommitPlanRequest{})
	case "ReportTaskOutcome":
		_, err = k.req.ReportTaskOutcome(k.as(k.reporter, "user"), &requestv1.ReportTaskOutcomeRequest{})
	default:
		t.Fatalf("no probe for %s", rpc)
	}
	if code(err) == codes.Unimplemented {
		panic(blocked{rpc})
	}
	t.Skipf("%s is implemented now (answered %v); extend this stage with its real steps (TASK-REQ-025-04)", rpc, err)
}
