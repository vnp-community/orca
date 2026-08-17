package domain

import "errors"

// Scope distinguishes a WorkflowTemplate's inheritance tier — company >
// team > personal, see workflow-service.md §4. Template inheritance
// resolution (walking the parent chain) is implemented — see
// ParentTemplateID below and usecase.ResolveTemplate — added 2026-08-17,
// closing the last deferred item of Epic C (docs/execution-plan.md §2/§10).
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
	// ErrTemplateSelfParent guards against a template naming itself as its
	// own parent, the smallest possible inheritance cycle — see
	// workflow-service.md §4: "Constructor rejects a template naming
	// itself as its own parent, directly." Multi-hop cycles can't arise
	// through this service's RPC surface (a parent must already exist,
	// with its own id assigned, before a child can reference it — there is
	// no UpdateTemplate RPC to rewire an existing template's parent after
	// the fact), so ResolveChain's depth cap (5) is the only additional
	// safety net needed, not full graph cycle detection.
	ErrTemplateSelfParent = errors.New("domain: a template cannot be its own parent")
)

// WorkflowTemplate is a reusable, named DAG definition — see
// workflow-service.md §4/§5. DAGJSON is kept as a raw string end-to-end
// (matching the generated proto's dag_json field and the templates table's
// JSONB column) and only parsed via ParseDAG where the structure is
// actually needed (Execute, ResolveTemplate), not on every template read.
type WorkflowTemplate struct {
	ID       string
	TenantID string
	Name     string
	DAGJSON  string
	Scope    Scope
	// ParentTemplateID is empty for a root template. Walked by
	// usecase.ResolveTemplate to compute the effective, inheritance-resolved
	// template — see that usecase's doc comment for the resolution policy.
	ParentTemplateID string
}

// NewWorkflowTemplate constructs a WorkflowTemplate, enforcing the
// invariants a template must satisfy to be meaningful: a tenant, a name, a
// valid scope, a dag_json that at least parses and passes
// DAGDefinition.Validate's structural checks (see dag.go), and — if
// parentTemplateID is set — that it isn't id itself.
func NewWorkflowTemplate(id, tenantID, name, dagJSON string, scope Scope, parentTemplateID string) (WorkflowTemplate, error) {
	if tenantID == "" {
		return WorkflowTemplate{}, ErrTemplateEmptyTenant
	}
	if name == "" {
		return WorkflowTemplate{}, ErrTemplateEmptyName
	}
	if !scope.Valid() {
		return WorkflowTemplate{}, ErrTemplateInvalidScope
	}
	if parentTemplateID != "" && parentTemplateID == id {
		return WorkflowTemplate{}, ErrTemplateSelfParent
	}
	dag, err := ParseDAG(dagJSON)
	if err != nil {
		return WorkflowTemplate{}, err
	}
	if err := dag.Validate(); err != nil {
		return WorkflowTemplate{}, err
	}
	return WorkflowTemplate{
		ID:               id,
		TenantID:         tenantID,
		Name:             name,
		DAGJSON:          dagJSON,
		Scope:            scope,
		ParentTemplateID: parentTemplateID,
	}, nil
}
