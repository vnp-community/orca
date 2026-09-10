package domain

import "testing"

func TestExecutionEngine_ConstantsAreDistinctAndNonEmpty(t *testing.T) {
	values := []ExecutionEngine{EngineDirectAgent, EngineOrchestration, EngineWorkflow}
	seen := make(map[ExecutionEngine]bool, len(values))
	for _, v := range values {
		if v == "" {
			t.Fatalf("ExecutionEngine constant must not be empty")
		}
		if seen[v] {
			t.Fatalf("duplicate ExecutionEngine value: %q", v)
		}
		seen[v] = true
	}
}
