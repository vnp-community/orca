package domain

import (
	"encoding/json"
	"errors"
)

var (
	// ErrGroupSelfParent guards against a group naming itself as its own
	// parent, the smallest possible inheritance cycle — mirrors
	// workflow-service's WorkflowTemplate.ErrTemplateSelfParent precedent.
	// Multi-hop cycles can't arise through this service's RPC surface either:
	// UpdateProjectGroup deliberately never rewires parent_group_id (see that
	// usecase's doc comment), so a parent must already exist, with its own id
	// assigned, before a child can reference it — no full graph
	// cycle-detection is needed, matching template.go's identical reasoning
	// (docs/execution-plan.md §11 precedent).
	ErrGroupSelfParent = errors.New("domain: a group cannot be its own parent")
	// ErrProjectGroupNotFound is the sentinel adapter/postgres returns
	// (wrapped) when a lookup/mutation targets a group that doesn't exist —
	// usecase/ maps this to apperrors.KindNotFound.
	ErrProjectGroupNotFound = errors.New("domain: project group not found")
)

// ProjectGroup is a folder-style organizational node for projects —
// self-referential via ParentGroupID, tenant-scoped. See
// project-service.md §4. This is the slice of that fuller model (the
// nullable project_id linking a group to a specific project) the current
// proto surface actually exercises — ProjectGroup here is a pure
// organizational tree, not yet tied to individual projects.
type ProjectGroup struct {
	ID       string
	TenantID string
	Name     string
	// ParentGroupID is empty for a root-of-tree group.
	ParentGroupID string
	// ProjectID is empty for a pure organizational folder node; set only
	// for a project's own leaf group (see UpsertLeafGroupForProject).
	ProjectID string
}

// NewProjectGroup constructs a ProjectGroup, enforcing the invariants a
// record must satisfy to be meaningful.
func NewProjectGroup(id, tenantID, name, parentGroupID string) (ProjectGroup, error) {
	if tenantID == "" {
		return ProjectGroup{}, ErrEmptyTenantID
	}
	if name == "" {
		return ProjectGroup{}, ErrEmptyName
	}
	if parentGroupID != "" && parentGroupID == id {
		return ProjectGroup{}, ErrGroupSelfParent
	}
	return ProjectGroup{ID: id, TenantID: tenantID, Name: name, ParentGroupID: parentGroupID}, nil
}

// NestedRepoCandidate is one filesystem entry ScanNested found under a
// scanned root path — mirrors project.proto's NestedRepoCandidate.
type NestedRepoCandidate struct {
	Path          string
	SuggestedName string
	IsGitRepo     bool
}

// nestedRepoCandidateWire is ParseNestedRepoCandidates's JSON decoding
// shape for one Dev Server Agent-reported candidate — snake_case keys,
// matching this codebase's other Agent JSON-RPC payloads (e.g.
// infra-fleet-service's ScanWorkspacePorts result shape). NOT yet
// confirmed against a real Agent handler — see this task's Context.
type nestedRepoCandidateWire struct {
	Path          string `json:"path"`
	SuggestedName string `json:"suggested_name"`
	IsGitRepo     bool   `json:"is_git_repo"`
}

// ParseNestedRepoCandidates decodes a Relay call's result_json into
// candidates — pure domain-layer JSON->struct mapping, no I/O (usecase.ScanNested
// is the one caller).
func ParseNestedRepoCandidates(resultJSON []byte) ([]NestedRepoCandidate, error) {
	var wire struct {
		Candidates []nestedRepoCandidateWire `json:"candidates"`
	}
	if err := json.Unmarshal(resultJSON, &wire); err != nil {
		return nil, err
	}
	out := make([]NestedRepoCandidate, 0, len(wire.Candidates))
	for _, c := range wire.Candidates {
		out = append(out, NestedRepoCandidate{Path: c.Path, SuggestedName: c.SuggestedName, IsGitRepo: c.IsGitRepo})
	}
	return out, nil
}
