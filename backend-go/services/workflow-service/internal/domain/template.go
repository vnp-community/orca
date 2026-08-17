package domain

import "errors"

// Scope distinguishes a WorkflowTemplate's inheritance tier — company >
// team > personal, see workflow-service.md §4. Template inheritance
// resolution (walking the parent chain) itself is out of scope for this
// scaffold's narrowed data model (see README "Known gaps"); Scope is kept
// as a plain enum field so the column exists and is validated from day one.
type Scope string

const (
	ScopeCompany  Scope = "company"
	ScopeTeam     Scope = "team"
	ScopePersonal Scope = "personal"
)

// Valid reports whether s is one of the three known scopes.
func (s Scope) Valid() bool {
	switch s {
	case ScopeCompany, ScopeTeam, ScopePersonal:
		return true
	default:
		return false
	}
}

var (
	// ErrTemplateEmptyTenant guards a template with no owning tenant.
	ErrTemplateEmptyTenant = errors.New("domain: tenant_id is required")
	// ErrTemplateEmptyName guards a template with no name.
	ErrTemplateEmptyName = errors.New("domain: name is required")
	// ErrTemplateInvalidScope guards Scope against anything outside the
	// closed company|team|personal enum.
	ErrTemplateInvalidScope = errors.New("domain: invalid scope")
	// ErrTemplateNotFound is the sentinel adapter/postgres returns (wrapped)
	// when a lookup finds no row — usecase maps it to apperrors.KindNotFound.
	ErrTemplateNotFound = errors.New("domain: workflow template not found")
)

// WorkflowTemplate is a reusable, named DAG definition — see
// workflow-service.md §4/§5. DAGJSON is kept as a raw string end-to-end
// (matching the generated proto's dag_json field and the templates table's
// JSONB column) and only parsed via ParseDAG where the structure is
// actually needed (Execute), not on every template read.
type WorkflowTemplate struct {
	ID       string
	TenantID string
	Name     string
	DAGJSON  string
	Scope    Scope
}

// NewWorkflowTemplate constructs a WorkflowTemplate, enforcing the
// invariants a template must satisfy to be meaningful: a tenant, a name, a
// valid scope, and (if non-empty) a dag_json that at least parses and
// passes DAGDefinition.Validate's structural checks (see dag.go).
func NewWorkflowTemplate(id, tenantID, name, dagJSON string, scope Scope) (WorkflowTemplate, error) {
	if tenantID == "" {
		return WorkflowTemplate{}, ErrTemplateEmptyTenant
	}
	if name == "" {
		return WorkflowTemplate{}, ErrTemplateEmptyName
	}
	if !scope.Valid() {
		return WorkflowTemplate{}, ErrTemplateInvalidScope
	}
	dag, err := ParseDAG(dagJSON)
	if err != nil {
		return WorkflowTemplate{}, err
	}
	if err := dag.Validate(); err != nil {
		return WorkflowTemplate{}, err
	}
	return WorkflowTemplate{
		ID:       id,
		TenantID: tenantID,
		Name:     name,
		DAGJSON:  dagJSON,
		Scope:    scope,
	}, nil
}
