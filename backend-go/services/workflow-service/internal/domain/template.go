package domain

import (
	"encoding/json"
	"errors"
)

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
	// itself as its own parent, directly." UpdateTemplate (added after this
	// comment was first written) CAN rewire an existing template's parent,
	// so a multi-hop cycle is reachable through this service's RPC surface
	// now — see usecase.UpdateTemplate's cycle re-validation, which walks
	// the new parent's ResolveChain and rejects if id appears in it. This
	// constructor-level check still catches the direct self-parent case on
	// every construction path (Create AND Update), independent of that
	// re-validation.
	ErrTemplateSelfParent = errors.New("domain: a template cannot be its own parent")
	// ErrTemplateVersionConflict is the sentinel adapter/postgres returns
	// (wrapped) when UpdateTemplate's conditional UPDATE affects zero rows
	// because templates.version has moved since the caller read it —
	// usecase maps this to apperrors.KindFailedPrecondition.
	ErrTemplateVersionConflict = errors.New("domain: template was modified by another request")
	// ErrTemplateEmptyOwner guards a template with no authoring owner —
	// workflow-service.md §4 names OwnerID as required.
	ErrTemplateEmptyOwner = errors.New("workflow: template owner id must not be empty")
	// ErrTemplateInvalidOverrides guards OverridesJSON against malformed JSON
	// at construction time — semantic validation (that overridden step ids
	// exist) happens later, at ResolveTemplate time.
	ErrTemplateInvalidOverrides = errors.New("domain: overrides must be valid JSON")
	// ErrTemplateInvalidInjectSteps guards InjectStepsJSON against malformed
	// JSON at construction time.
	ErrTemplateInvalidInjectSteps = errors.New("domain: inject_steps must be valid JSON")
	// ErrTemplateInvalidRemoveSteps guards RemoveStepsJSON against malformed
	// JSON at construction time.
	ErrTemplateInvalidRemoveSteps = errors.New("domain: remove_steps must be valid JSON")
	// ErrTemplateInvalidVisibility guards Visibility against anything
	// outside the closed private|team|company|public enum.
	ErrTemplateInvalidVisibility = errors.New("domain: invalid visibility")
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
	// Version is bumped by UpdateTemplate on every write; 1 at creation —
	// backs the version-bump-on-write optimistic concurrency check
	// (SOL-030), mirroring SOL-001's AccessPolicy pattern.
	Version int32

	OwnerID     string   // required — the authoring user, workflow-service.md §4
	Description string   // optional
	Tags        []string // optional, GIN-indexed for BUG-WF-03's library search

	// Inherit-mode merge instructions, applied against the resolved parent
	// chain by resolveEffectiveTemplate. Ignored when ParentTemplateID is
	// empty.
	OverridesJSON   string // map[stepId]json.RawMessage — shallow per-field merge onto that step's Config
	InjectStepsJSON string // []domain.Step — appended after remove_steps is applied
	RemoveStepsJSON string // []string — step ids to drop from the parent's resolved steps

	UsageCount int32 // incremented by workflow-service.Execute, read by the version-bump policy

	// ClonedFromTemplateID is a provenance-only pointer (Clone mode
	// deliberately has no live ParentTemplateID) — never walked by
	// ResolveChain, never affects resolution.
	ClonedFromTemplateID string

	// Visibility is the sharing tier BUG-WF-03's escalate-forward publish
	// state machine governs — see Visibility.CanEscalateTo. Defaults to
	// VisibilityPrivate at construction.
	Visibility Visibility
	// ShareToken is set once Visibility reaches VisibilityPublic (empty
	// otherwise) — see usecase.PublishTemplate/GetShareLinkPreview.
	ShareToken string
	// RatingSum/RatingCount back AverageRating — the average itself is a
	// derived value, never stored (matches rating_sum/rating_count's
	// migration comment: "average computed at read time, not stored").
	RatingSum   int32
	RatingCount int32
}

// AverageRating is a derived value, not a stored field — matches
// rating_sum/rating_count's "don't persist a derived value" posture. 0
// when RatingCount is 0 (no divide-by-zero panic), which is also the
// correct "no ratings yet" answer.
func (t WorkflowTemplate) AverageRating() float64 {
	if t.RatingCount == 0 {
		return 0
	}
	return float64(t.RatingSum) / float64(t.RatingCount)
}

// NewWorkflowTemplate constructs a WorkflowTemplate, enforcing the
// invariants a template must satisfy to be meaningful: a tenant, a name, a
// valid scope, a dag_json that at least parses and passes
// DAGDefinition.Validate's structural checks (see dag.go), and — if
// parentTemplateID is set — that it isn't id itself.
func NewWorkflowTemplate(id, tenantID, name, dagJSON string, scope Scope, parentTemplateID, ownerID string, opts ...TemplateOption) (WorkflowTemplate, error) {
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
	if ownerID == "" {
		return WorkflowTemplate{}, ErrTemplateEmptyOwner
	}
	dag, err := ParseDAG(dagJSON)
	if err != nil {
		return WorkflowTemplate{}, err
	}
	if err := dag.Validate(); err != nil {
		return WorkflowTemplate{}, err
	}
	t := WorkflowTemplate{
		ID:               id,
		TenantID:         tenantID,
		Name:             name,
		DAGJSON:          dagJSON,
		Scope:            scope,
		ParentTemplateID: parentTemplateID,
		Version:          1,
		OwnerID:          ownerID,
		Visibility:       VisibilityPrivate,
	}
	for _, opt := range opts {
		if err := opt(&t); err != nil {
			return WorkflowTemplate{}, err
		}
	}
	return t, nil
}

// validOptionalJSON reports whether s is empty (unset) or parses as valid
// JSON — used to check the Inherit-mode merge-instruction fields at
// construction time. Semantic validation (that overridden step ids exist,
// that inject/remove entries are well-formed steps) happens later, at
// ResolveTemplate time — matching how DAGJSON's own semantic checks live in
// dag.Validate rather than here.
func validOptionalJSON(s string) bool {
	if s == "" {
		return true
	}
	return json.Valid([]byte(s))
}

// TemplateOption configures optional authoring/Inherit-mode fields on
// WorkflowTemplate at construction time — kept out of NewWorkflowTemplate's
// required positional parameters (which cover only the invariants every
// template must satisfy) so callers that don't need them aren't forced to
// thread empty strings through.
type TemplateOption func(*WorkflowTemplate) error

// WithDescription sets the optional human-readable description.
func WithDescription(description string) TemplateOption {
	return func(t *WorkflowTemplate) error {
		t.Description = description
		return nil
	}
}

// WithTags sets the optional library-search tags.
func WithTags(tags []string) TemplateOption {
	return func(t *WorkflowTemplate) error {
		t.Tags = tags
		return nil
	}
}

// WithUsageCount sets the usage counter (used when reconstructing a
// WorkflowTemplate from a persisted row; new templates start at 0, the
// struct's zero value).
func WithUsageCount(usageCount int32) TemplateOption {
	return func(t *WorkflowTemplate) error {
		t.UsageCount = usageCount
		return nil
	}
}

// WithClonedFrom sets the Clone-mode provenance pointer.
func WithClonedFrom(clonedFromTemplateID string) TemplateOption {
	return func(t *WorkflowTemplate) error {
		t.ClonedFromTemplateID = clonedFromTemplateID
		return nil
	}
}

// WithVisibility overrides the default VisibilityPrivate — used by
// adapter/postgres when reconstructing a persisted row (visibility may
// already have been escalated) rather than by CreateTemplate, which always
// starts a new template at VisibilityPrivate (see usecase.PublishTemplate
// for the only path that changes it after creation).
func WithVisibility(visibility Visibility) TemplateOption {
	return func(t *WorkflowTemplate) error {
		if !visibility.Valid() {
			return ErrTemplateInvalidVisibility
		}
		t.Visibility = visibility
		return nil
	}
}

// WithShareToken sets the share token — non-empty only once Visibility has
// reached VisibilityPublic (see usecase.PublishTemplate).
func WithShareToken(shareToken string) TemplateOption {
	return func(t *WorkflowTemplate) error {
		t.ShareToken = shareToken
		return nil
	}
}

// WithRating sets the aggregate rating fields — used by adapter/postgres
// when reconstructing a persisted row; RateTemplate mutates these via a
// direct SQL increment, not through this constructor path.
func WithRating(sum, count int32) TemplateOption {
	return func(t *WorkflowTemplate) error {
		t.RatingSum = sum
		t.RatingCount = count
		return nil
	}
}

// WithOverrides sets the Inherit-mode per-step override instructions,
// rejecting malformed JSON at construction time. Semantic validation (that
// the overridden step ids actually exist in the resolved parent chain)
// happens later, at ResolveTemplate time — see TASK-WF-01-04.
func WithOverrides(overridesJSON string) TemplateOption {
	return func(t *WorkflowTemplate) error {
		if !validOptionalJSON(overridesJSON) {
			return ErrTemplateInvalidOverrides
		}
		t.OverridesJSON = overridesJSON
		return nil
	}
}

// WithInjectSteps sets the Inherit-mode injected-steps instruction,
// rejecting malformed JSON at construction time.
func WithInjectSteps(injectStepsJSON string) TemplateOption {
	return func(t *WorkflowTemplate) error {
		if !validOptionalJSON(injectStepsJSON) {
			return ErrTemplateInvalidInjectSteps
		}
		t.InjectStepsJSON = injectStepsJSON
		return nil
	}
}

// WithRemoveSteps sets the Inherit-mode removed-steps instruction,
// rejecting malformed JSON at construction time.
func WithRemoveSteps(removeStepsJSON string) TemplateOption {
	return func(t *WorkflowTemplate) error {
		if !validOptionalJSON(removeStepsJSON) {
			return ErrTemplateInvalidRemoveSteps
		}
		t.RemoveStepsJSON = removeStepsJSON
		return nil
	}
}
