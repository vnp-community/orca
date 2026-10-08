//go:build e2e

package e2e

import (
	"testing"
)

// TestScenarios runs E01 to E16 and E19. Each scenario gets its own tenant, so they run in parallel against
// the one service. Stages that wait on RPCs from other tasks report SKIP with the task named; they are not hidden.
func TestScenarios(t *testing.T) {
	for _, sc := range allScenarios() {
		sc := sc
		t.Run(sc.ID+"_"+sc.Type, func(t *testing.T) {
			t.Parallel()
			t.Cleanup(func() { recordScenarioResult(sc.ID, !t.Failed()) })
			runScenario(t, sc)
		})
	}
}
