package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// flowFixturePath is the copy of the flow registry the frontend reads (its fallback table is tested against it), so the table
// the screens use when the gateway cannot answer `request.flow` can never drift from FlowFor unnoticed.
const flowFixturePath = "../../../../../frontend/src/shared/request-flow-registry.fixture.json"

type flowFixtureEntry struct {
	AnalysisKind           string   `json:"analysisKind"`
	AnalysisGate           string   `json:"analysisGate"`
	PlanKind               string   `json:"planKind"`
	PhaseRule              string   `json:"phaseRule"`
	StartGate              string   `json:"startGate"`
	ExecutionGates         []string `json:"executionGates"`
	CompletesAfterAnalysis bool     `json:"completesAfterAnalysis"`
	HumanConfirmRequired   bool     `json:"humanConfirmRequired"`
	OpenSpecProfile        string   `json:"openSpecProfile"`
}

func buildFlowFixture(t *testing.T) []byte {
	t.Helper()
	out := map[string]flowFixtureEntry{}
	for _, typ := range AllFlowTypes() {
		f, err := FlowFor(typ)
		if err != nil {
			t.Fatal(err)
		}
		gates := []string{}
		for _, g := range f.ExecutionGates {
			gates = append(gates, string(g))
		}
		out[string(typ)] = flowFixtureEntry{
			AnalysisKind: string(f.AnalysisKind), AnalysisGate: string(f.AnalysisGate), PlanKind: string(f.PlanKind), PhaseRule: string(f.PhaseRule),
			StartGate: string(f.StartGate), ExecutionGates: gates, CompletesAfterAnalysis: f.CompletesAfterAnalysis,
			HumanConfirmRequired: f.HumanConfirmRequired, OpenSpecProfile: string(f.OpenSpecProfile),
		}
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

// TestFlowFixtureMatchesFlowFor fails when FlowFor changed without the frontend fixture. Regenerate it with
// UPDATE_FLOW_FIXTURE=1 go test ./internal/domain -run TestFlowFixtureMatchesFlowFor, then update the frontend fallback table.
func TestFlowFixtureMatchesFlowFor(t *testing.T) {
	want := buildFlowFixture(t)
	path := filepath.FromSlash(flowFixturePath)
	if os.Getenv("UPDATE_FLOW_FIXTURE") == "1" {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the frontend fixture: %v (regenerate with UPDATE_FLOW_FIXTURE=1)", err)
	}
	// Why: the frontend formatter rewrites the file's whitespace; only its content has to match FlowFor.
	if !sameJSON(t, got, want) {
		t.Fatalf("%s is out of date with FlowFor; regenerate it with UPDATE_FLOW_FIXTURE=1 and update frontend/src/shared/request-flow-registry.ts", flowFixturePath)
	}
}

func sameJSON(t *testing.T, a, b []byte) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatalf("the fixture is not valid JSON: %v", err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(x, y)
}
