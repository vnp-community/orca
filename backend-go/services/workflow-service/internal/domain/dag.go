package domain

import (
	"encoding/json"
	"errors"
	"fmt"
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
	// ErrSelfReferencingStep is the minimum cycle check this scaffold
	// implements — see workflow-service.md §4's BuildWaves note: a full
	// Kahn's-algorithm topological sort (multi-node cycles) is not
	// implemented here, only the direct self-reference case, which is
	// nonetheless a real and common authoring mistake to catch.
	ErrSelfReferencingStep = errors.New("domain: step cannot depend on itself")
	// ErrStepDependencyNotFound is returned when a dependsOn entry names a
	// step id that doesn't exist in the same definition.
	ErrStepDependencyNotFound = errors.New("domain: step depends on an unknown step id")
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
// sort would need anyway: every step id is non-empty and unique, and every
// dependsOn edge resolves to an existing, different step. It deliberately
// does NOT implement Kahn's-algorithm wave computation or general
// (multi-node) cycle detection — see workflow-service.md §4 and this
// service's README "Known gaps": that's flagged as not implemented in this
// scaffold, not silently skipped.
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
