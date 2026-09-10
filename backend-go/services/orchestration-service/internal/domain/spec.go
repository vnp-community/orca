// Package domain — spec.go: pure DAG-node materialization for
// StartCoordinatorRun's spec_json payload. Kept in its own file since
// orchestration.go's existing doc comment scopes that file to the entity
// definitions themselves, not caller-driven expansion logic.
package domain

import (
	"encoding/json"
	"errors"
	"fmt"
)

// SpecNode is the caller-supplied shape StartCoordinatorRunRequest.spec_json
// must parse into — the CLOSED contract between task-service's
// buildOrchestrationSpec (SOL-TG-04) and this service's ExpandSpec. Kept
// deliberately minimal: task-service's own richer per-task fields
// (prompt_template, description) travel inside Spec (opaque JSONB on the
// resulting OrchestrationTask), not as SpecNode fields — this service
// never interprets task-service's authoring content, per
// orchestration-service.md's "does not decide decomposition strategy."
type SpecNode struct {
	TempID string          `json:"tempId"`
	Title  string          `json:"title"`
	Spec   json.RawMessage `json:"spec"`
	Deps   []string        `json:"deps"` // TempIDs of sibling nodes, same closed set
}

// ErrEmptySpec / ErrDuplicateTempID / ErrDanglingDep guard ExpandSpec's
// invariants — a malformed spec must fail StartCoordinatorRun closed
// rather than create a DAG that can never promote (a dangling dep
// reference for a node that also has status pending would sit stuck
// forever, silently, which is worse than a rejected request).
var (
	ErrEmptySpec       = errors.New("domain: spec must contain at least one node")
	ErrDuplicateTempID = errors.New("domain: duplicate tempId in spec")
	ErrDanglingDep     = errors.New("domain: dep references an unknown tempId")
)

// ExpandSpec parses spec_json into SpecNodes and materializes them into
// OrchestrationTasks scoped to coordinatorRunID/tenantID — real IDs are NOT
// minted here (the repository mints them on INSERT, matching
// OrchestrationTaskRepository.Create's existing id-if-empty convention,
// repository.go:44-46); this function instead resolves each node's Deps
// from TempID strings so the repository layer (TASK-TASKV1-005-05) can
// resolve them to real minted ids inside its own INSERT transaction. Root
// node (index 0 in the returned slice) is fixed as nodes[0] by convention
// — task-service's buildOrchestrationSpec always emits the root task
// first (SOL-TG-04's own sketch: "origin_task_id = rootTaskID").
func ExpandSpec(tenantID, coordinatorRunID, originTaskID string, specJSON json.RawMessage) ([]OrchestrationTask, error) {
	var nodes []SpecNode
	if err := json.Unmarshal(specJSON, &nodes); err != nil {
		return nil, fmt.Errorf("domain: invalid spec_json: %w", err)
	}
	if len(nodes) == 0 {
		return nil, ErrEmptySpec
	}
	seen := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		if _, dup := seen[n.TempID]; dup {
			return nil, ErrDuplicateTempID
		}
		seen[n.TempID] = struct{}{}
	}
	tasks := make([]OrchestrationTask, 0, len(nodes))
	for i, n := range nodes {
		for _, d := range n.Deps {
			if _, ok := seen[d]; !ok {
				return nil, ErrDanglingDep
			}
		}
		origin := ""
		if i == 0 {
			origin = originTaskID // root row only, per §4's field doc comment
		}
		status := TaskStatusPending
		if len(n.Deps) == 0 {
			status = TaskStatusReady // no deps -> immediately dispatchable, matches DepsSatisfied(nil) == true
		}
		tasks = append(tasks, OrchestrationTask{
			TenantID:         tenantID,
			CoordinatorRunID: coordinatorRunID,
			OriginTaskID:     origin,
			TaskTitle:        n.Title,
			Spec:             n.Spec,
			Status:           status,
			Deps:             n.Deps, // still TempIDs here; repository.CreateWithTasks resolves TempID->real id in the same transaction
		})
	}
	return tasks, nil
}
