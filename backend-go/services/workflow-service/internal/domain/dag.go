package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	// ErrEmptyStepID is returned by DAGDefinition.Validate when a step has
	// no id — every step needs a stable id both as a dependsOn target and
	// as the step_executions.step_id join key (see workflow-service.md §5).
	ErrEmptyStepID = errors.New("domain: step id must not be empty")
	// ErrDuplicateStepID guards dependsOn resolution: two steps sharing an
	// id would make "which step does this edge point at" ambiguous.
	ErrDuplicateStepID = errors.New("domain: duplicate step id")
	// ErrSelfReferencingStep is Validate's cheap direct-cycle check — a
	// single step depending on itself, checked before BuildWaves' full
	// Kahn's-algorithm pass runs at all, since it's a real and common
	// authoring mistake worth catching without walking the whole graph.
	ErrSelfReferencingStep = errors.New("domain: step cannot depend on itself")
	// ErrStepDependencyNotFound is returned when a dependsOn entry names a
	// step id that doesn't exist in the same definition.
	ErrStepDependencyNotFound = errors.New("domain: step depends on an unknown step id")
	// ErrCyclicDependency is returned by BuildWaves when Kahn's algorithm
	// terminates with steps still unprocessed — a multi-node cycle (e.g.
	// a->b->c->a) that Validate's pairwise self-reference/unknown-dependency
	// checks can't catch, since every edge in such a cycle resolves to a
	// real, distinct step. Mirrors TS's WorkflowCycleError, see
	// workflow-service.md §4.
	ErrCyclicDependency = errors.New("domain: dag contains a cyclic dependency")
)

// Step is one node in a workflow's flat step list — dependsOn edges make it
// a DAG, per workflow-service.md §4. Config is kept as raw JSON: its shape
// is StepType-specific and only the matching StepExecutor needs to parse it.
type Step struct {
	ID        string          `json:"id"`
	Type      StepType        `json:"type"`
	Config    json.RawMessage `json:"config,omitempty"`
	DependsOn []string        `json:"dependsOn,omitempty"`
}

// DAGDefinition is the parsed shape of a WorkflowTemplate's DAGJSON (or a
// WorkflowExecution's frozen snapshot of the same) — see
// workflow-service.md §4/§5. Not persisted directly; it's a transient
// in-memory view over the JSONB column.
type DAGDefinition struct {
	Steps []Step `json:"steps"`
}

// ParseDAG unmarshals a template/execution's dag_json column into a
// DAGDefinition. An empty or blank string parses to an empty step list
// (a template with no steps yet is a valid, if useless, template) rather
// than an error.
func ParseDAG(dagJSON string) (DAGDefinition, error) {
	if strings.TrimSpace(dagJSON) == "" {
		return DAGDefinition{Steps: []Step{}}, nil
	}
	var d DAGDefinition
	if err := json.Unmarshal([]byte(dagJSON), &d); err != nil {
		return DAGDefinition{}, fmt.Errorf("domain: parse dag_json: %w", err)
	}
	return d, nil
}

// Validate checks the structural invariants BuildWaves' full topological
// sort needs anyway: every step id is non-empty and unique, and every
// dependsOn edge resolves to an existing, different step. It only catches
// the direct self-reference case as a cycle — general (multi-node) cycle
// detection is BuildWaves' job (see its doc comment), since it requires
// walking the whole graph rather than a pairwise check. Callers should run
// Validate before BuildWaves: BuildWaves assumes Validate's invariants
// already hold and does not re-check them.
func (d DAGDefinition) Validate() error {
	seen := make(map[string]bool, len(d.Steps))
	for _, s := range d.Steps {
		if s.ID == "" {
			return ErrEmptyStepID
		}
		if seen[s.ID] {
			return fmt.Errorf("%w: %s", ErrDuplicateStepID, s.ID)
		}
		seen[s.ID] = true
	}
	for _, s := range d.Steps {
		for _, dep := range s.DependsOn {
			if dep == s.ID {
				return fmt.Errorf("%w: %s", ErrSelfReferencingStep, s.ID)
			}
			if !seen[dep] {
				return fmt.Errorf("%w: step %q depends on %q", ErrStepDependencyNotFound, s.ID, dep)
			}
		}
	}
	return nil
}

// BuildWaves computes wave-ordered layers of Steps via Kahn's algorithm —
// identical in shape to TS's DAGBuilder.buildWaves(), see workflow-service.md
// §4/§7: track each step's in-degree (count of unsatisfied dependsOn
// edges), repeatedly collect every step whose in-degree has reached zero
// into the next wave, then decrement the in-degree of each collected
// step's dependents. usecase's wave-dispatch engine dispatches every step
// in a wave concurrently and gates wave N+1 on wave N fully completing.
//
// Steps within a wave preserve DAGDefinition's original step order, for
// deterministic dispatch order given the same input. Assumes Validate has
// already run: BuildWaves does not re-check empty/duplicate ids or
// dangling dependencies, only the general (multi-node) cycle case Validate
// can't catch — see ErrCyclicDependency.
func (d DAGDefinition) BuildWaves() ([][]Step, error) {
	if len(d.Steps) == 0 {
		return nil, nil
	}

	inDegree := make(map[string]int, len(d.Steps))
	dependents := make(map[string][]string, len(d.Steps))
	for _, s := range d.Steps {
		if _, ok := inDegree[s.ID]; !ok {
			inDegree[s.ID] = 0
		}
		for _, dep := range s.DependsOn {
			inDegree[s.ID]++
			dependents[dep] = append(dependents[dep], s.ID)
		}
	}

	processed := make(map[string]bool, len(d.Steps))
	var waves [][]Step
	for len(processed) < len(d.Steps) {
		var wave []Step
		for _, s := range d.Steps {
			if !processed[s.ID] && inDegree[s.ID] == 0 {
				wave = append(wave, s)
			}
		}
		if len(wave) == 0 {
			// Every remaining step still has an unsatisfied dependency, but
			// none of those dependencies are ever going to reach zero —
			// the only way that happens is a cycle among the remaining
			// steps. Name them all, sorted for a deterministic message.
			var stuck []string
			for _, s := range d.Steps {
				if !processed[s.ID] {
					stuck = append(stuck, s.ID)
				}
			}
			sort.Strings(stuck)
			return nil, fmt.Errorf("%w: %s", ErrCyclicDependency, strings.Join(stuck, ", "))
		}

		for _, s := range wave {
			processed[s.ID] = true
		}
		for _, s := range wave {
			for _, dependent := range dependents[s.ID] {
				inDegree[dependent]--
			}
		}
		waves = append(waves, wave)
	}
	return waves, nil
}
