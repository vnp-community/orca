//go:build e2e

package e2e

import (
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

var (
	passedMu sync.Mutex
	passed   = map[string]bool{}
)

func recordScenarioResult(id string, ok bool) {
	passedMu.Lock()
	passed[id] = ok
	passedMu.Unlock()
}

// missingTypes lists the request types that no scenario covers. With results it counts only scenarios that
// passed, so a type whose scenario is red is as uncovered as one with no scenario.
func missingTypes(scs []Scenario, types []domain.RequestType, results map[string]bool) []string {
	covered := map[string]bool{}
	for _, sc := range scs {
		if results != nil && !results[sc.ID] {
			continue
		}
		covered[sc.Type] = true
	}
	var missing []string
	for _, ty := range types {
		if !covered[string(ty)] {
			missing = append(missing, string(ty))
		}
	}
	sort.Strings(missing)
	return missing
}

// TestEveryRequestTypeHasScenario reads the 11 types from the registry (domain.AllFlowTypes, backed by FlowFor), so a
// twelfth type without a scenario turns it red. When TestScenarios ran in this process it also demands the scenario passed.
func TestEveryRequestTypeHasScenario(t *testing.T) {
	passedMu.Lock()
	results := map[string]bool{}
	for k, v := range passed {
		results[k] = v
	}
	passedMu.Unlock()
	if len(results) == 0 {
		results = nil // TestScenarios was filtered out: presence only
	}
	types := domain.AllFlowTypes()
	if len(types) != len(domain.AllRequestTypes()) {
		t.Fatalf("flow registry lists %d types, request types are %d", len(types), len(domain.AllRequestTypes()))
	}
	if missing := missingTypes(allScenarios(), types, results); len(missing) > 0 {
		t.Fatalf("request types without a passing scenario: %s", strings.Join(missing, ", "))
	}
}

func TestTypeMatrixTurnsRedForATwelfthType(t *testing.T) {
	extended := append(domain.AllFlowTypes(), domain.RequestType("twelfth_type"))
	missing := missingTypes(allScenarios(), extended, nil)
	if len(missing) != 1 || missing[0] != "twelfth_type" {
		t.Fatalf("a type without a scenario must be reported, got %v", missing)
	}
	// A scenario that failed does not count as coverage.
	scs := []Scenario{{ID: "X1", Type: "bug"}}
	if got := missingTypes(scs, []domain.RequestType{"bug"}, map[string]bool{"X1": false}); len(got) != 1 {
		t.Fatalf("a failed scenario must not cover its type, got %v", got)
	}
}

// Each flow family of CR-REQ-025 section 2.3 needs at least one scenario, and it must have passed.
func TestEveryFlowGroupHasAPassingScenario(t *testing.T) {
	passedMu.Lock()
	defer passedMu.Unlock()
	byGroup := map[Group]int{}
	for _, sc := range allScenarios() {
		if len(passed) == 0 || passed[sc.ID] {
			byGroup[sc.Group]++
		}
	}
	for _, g := range []Group{GroupFull, GroupShort, GroupHotfix, GroupSpike, GroupCrossCut} {
		if byGroup[g] == 0 {
			t.Errorf("flow group %q has no passing scenario", g)
		}
	}
}
