package domain

import (
	"reflect"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

func errCode(err error) string {
	if ae, ok := err.(*apperrors.AppError); ok {
		return ae.Code
	}
	return ""
}

// Expected values are written by hand from CR-REQ-003 section 2.1, not read from the registry.
func TestFlowFor_AllElevenMatchTable(t *testing.T) {
	want := []FlowDefinition{
		{Type: RequestTypeChangeRequest, AnalysisKind: "solution", AnalysisGate: "solution", PlanKind: "plan", PhaseRule: "always", StartGate: "plan", ExecutionGates: []GateSubject{"phase"}},
		{Type: RequestTypeBug, AnalysisKind: "diagnosis", AnalysisGate: "solution", PlanKind: "plan", PhaseRule: "when_size_L", StartGate: "plan"},
		{Type: RequestTypeHotfix, HumanConfirmRequired: true, AnalysisKind: "diagnosis", AnalysisGate: "", PlanKind: "single_task", PhaseRule: "never", StartGate: "pre_deploy"},
		{Type: RequestTypeTask, AnalysisKind: "none", PlanKind: "task_list", PhaseRule: "never", StartGate: "task_list"},
		{Type: RequestTypeSpike, AnalysisKind: "findings", AnalysisGate: "findings", PlanKind: "none", PhaseRule: "never", CompletesAfterAnalysis: true},
		{Type: RequestTypeQuestion, AnalysisKind: "answer", AnalysisGate: "answer", PlanKind: "none", PhaseRule: "never", CompletesAfterAnalysis: true},
		{Type: RequestTypeRefactor, AnalysisKind: "solution", AnalysisGate: "solution", PlanKind: "plan", PhaseRule: "when_size_L", StartGate: "plan"},
		{Type: RequestTypeSecurity, HumanConfirmRequired: true, AnalysisKind: "diagnosis", AnalysisGate: "solution", PlanKind: "plan", PhaseRule: "never", StartGate: "pre_deploy"},
		{Type: RequestTypePerformance, AnalysisKind: "diagnosis", AnalysisGate: "solution", PlanKind: "plan", PhaseRule: "never", StartGate: "plan"},
		{Type: RequestTypeDocs, AnalysisKind: "none", PlanKind: "task_list", PhaseRule: "never", StartGate: "task_list"},
		{Type: RequestTypeOpsRequest, AnalysisKind: "none", PlanKind: "plan", PhaseRule: "never", StartGate: "plan", ExecutionGates: []GateSubject{"pre_deploy"}},
	}
	for _, w := range want {
		got, err := FlowFor(w.Type)
		if err != nil {
			t.Fatalf("%s: %v", w.Type, err)
		}
		got.OpenSpecProfile = "" // covered by openspec_profile_test
		if !reflect.DeepEqual(got, w) {
			t.Errorf("%s:\n got  %+v\n want %+v", w.Type, got, w)
		}
	}
}

func TestFlowFor_UnknownType(t *testing.T) {
	for _, typ := range []RequestType{"", "feature", "BUG"} {
		if _, err := FlowFor(typ); errCode(err) != "REQUEST_FLOW_UNKNOWN_TYPE" {
			t.Errorf("%q: got %v", typ, err)
		}
	}
}

func TestFlowFor_ReturnsCopy(t *testing.T) {
	f, _ := FlowFor(RequestTypeChangeRequest)
	f.ExecutionGates[0] = "tampered"
	g, _ := FlowFor(RequestTypeChangeRequest)
	if g.ExecutionGates[0] != GatePhase {
		t.Fatalf("registry was mutated through the returned copy: %v", g.ExecutionGates)
	}
}

func TestPhasesFor(t *testing.T) {
	cases := []struct {
		rule PhaseRule
		size RequestSize
		want bool
	}{
		{PhaseAlways, "S", true}, {PhaseAlways, "M", true}, {PhaseAlways, "L", true}, {PhaseAlways, "", true},
		{PhaseWhenSizeL, "S", false}, {PhaseWhenSizeL, "M", false}, {PhaseWhenSizeL, "L", true}, {PhaseWhenSizeL, "", false},
		{PhaseNever, "S", false}, {PhaseNever, "M", false}, {PhaseNever, "L", false}, {PhaseNever, "", false},
	}
	for _, c := range cases {
		if got := (FlowDefinition{PhaseRule: c.rule}).PhasesFor(c.size); got != c.want {
			t.Errorf("%s/%q = %v, want %v", c.rule, c.size, got, c.want)
		}
	}
}

func TestFlowTypesCoverAllRequestTypes(t *testing.T) {
	if !reflect.DeepEqual(AllFlowTypes(), AllRequestTypes()) {
		t.Fatalf("flow types %v != request types %v", AllFlowTypes(), AllRequestTypes())
	}
	if len(AllFlowTypes()) != 11 {
		t.Fatalf("want 11 types, got %d", len(AllFlowTypes()))
	}
}

func TestFlowFor_CompletesAfterAnalysisOnlySpikeAndQuestion(t *testing.T) {
	for _, typ := range AllFlowTypes() {
		f, _ := FlowFor(typ)
		want := typ == RequestTypeSpike || typ == RequestTypeQuestion
		if f.CompletesAfterAnalysis != want {
			t.Errorf("%s CompletesAfterAnalysis=%v", typ, f.CompletesAfterAnalysis)
		}
	}
}
