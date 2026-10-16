package domain

import (
	"reflect"
	"testing"
)

func TestParseExecutionMode(t *testing.T) {
	cases := []struct {
		in      string
		want    ExecutionMode
		wantErr bool
	}{
		{"", ExecutionStepByStep, false},
		{"step_by_step", ExecutionStepByStep, false},
		{" Continuous ", ExecutionContinuous, false},
		{"turbo", "", true},
	}
	for _, c := range cases {
		got, err := ParseExecutionMode(c.in)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("ParseExecutionMode(%q) = %q, %v", c.in, got, err)
		}
	}
}

// Only change_request carries a Phase gate; continuous removes exactly that one and nothing else, for every type.
func TestEffectiveExecutionGates_AllTypesBothModes(t *testing.T) {
	for _, typ := range AllFlowTypes() {
		flow, err := FlowFor(typ)
		if err != nil {
			t.Fatal(err)
		}
		if got := flow.EffectiveExecutionGates(ExecutionStepByStep); !reflect.DeepEqual(got, append([]GateSubject{}, flow.ExecutionGates...)) {
			t.Errorf("%s step_by_step: gates %v, want %v", typ, got, flow.ExecutionGates)
		}
		var want []GateSubject
		for _, g := range flow.ExecutionGates {
			if g != GatePhase {
				want = append(want, g)
			}
		}
		got := flow.EffectiveExecutionGates(ExecutionContinuous)
		if len(got) != len(want) || (len(want) > 0 && !reflect.DeepEqual(got, want)) {
			t.Errorf("%s continuous: gates %v, want %v", typ, got, want)
		}
		if flow.HasExecutionGateFor(GatePhase, ExecutionContinuous) {
			t.Errorf("%s continuous must not have a Phase gate", typ)
		}
		wantStepPhase := typ == RequestTypeChangeRequest
		if flow.HasExecutionGateFor(GatePhase, ExecutionStepByStep) != wantStepPhase {
			t.Errorf("%s step_by_step Phase gate = %v, want %v", typ, !wantStepPhase, wantStepPhase)
		}
	}
}

func TestEffectiveExecutionGates_PreDeployStaysInContinuous(t *testing.T) {
	flow, _ := FlowFor(RequestTypeOpsRequest)
	if !flow.HasExecutionGateFor(GatePreDeploy, ExecutionContinuous) {
		t.Fatal("ops_request keeps its pre_deploy gate when continuous")
	}
}

func TestEffectiveExecutionGates_ReturnsACopy(t *testing.T) {
	flow, _ := FlowFor(RequestTypeChangeRequest)
	got := flow.EffectiveExecutionGates(ExecutionStepByStep)
	got[0] = GateNone
	if again := flow.EffectiveExecutionGates(ExecutionStepByStep); again[0] != GatePhase {
		t.Fatal("callers must not be able to change the registry")
	}
}

func TestStartsPhasesItself(t *testing.T) {
	cr, _ := FlowFor(RequestTypeChangeRequest)
	bug, _ := FlowFor(RequestTypeBug)
	task, _ := FlowFor(RequestTypeTask)
	cases := []struct {
		name string
		flow FlowDefinition
		mode ExecutionMode
		size RequestSize
		want bool
	}{
		{"change_request continuous", cr, ExecutionContinuous, RequestSizeM, true},
		{"change_request step_by_step", cr, ExecutionStepByStep, RequestSizeM, false},
		{"change_request unpinned", cr, "", RequestSizeM, false},
		{"bug size L continuous", bug, ExecutionContinuous, RequestSizeL, true},
		{"bug size M continuous has no phases", bug, ExecutionContinuous, RequestSizeM, false},
		{"task continuous has no phases", task, ExecutionContinuous, RequestSizeL, false},
	}
	for _, c := range cases {
		if got := c.flow.StartsPhasesItself(c.mode, c.size); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEffectiveExecutionModeOf(t *testing.T) {
	continuous := &ProjectEngineSettings{ExecutionMode: ExecutionContinuous}
	cases := []struct {
		name     string
		pinned   ExecutionMode
		settings *ProjectEngineSettings
		defaults EngineDefaults
		want     ExecutionMode
	}{
		{"pinned wins over the setting", ExecutionStepByStep, continuous, EngineDefaults{}, ExecutionStepByStep},
		{"setting wins over the default", "", continuous, EngineDefaults{ExecutionMode: ExecutionStepByStep}, ExecutionContinuous},
		{"default applies without a setting", "", nil, EngineDefaults{ExecutionMode: ExecutionContinuous}, ExecutionContinuous},
		{"nothing set is step_by_step", "", nil, EngineDefaults{}, ExecutionStepByStep},
		{"a setting row without a mode falls to the default", "", &ProjectEngineSettings{}, EngineDefaults{ExecutionMode: ExecutionContinuous}, ExecutionContinuous},
	}
	for _, c := range cases {
		if got := EffectiveExecutionModeOf(c.pinned, c.settings, c.defaults); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPhaseStartTriggerDefault(t *testing.T) {
	if PhaseStartTrigger("").OrDefault() != PhaseStartManual || PhaseStartAuto.OrDefault() != PhaseStartAuto {
		t.Fatal("an unset trigger reads as manual, an explicit one is kept")
	}
}
