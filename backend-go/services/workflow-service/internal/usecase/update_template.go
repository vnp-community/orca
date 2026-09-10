package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

type UpdateTemplateInput struct {
	ID, Name, DAGJSON, ParentTemplateID string
	Scope                               domain.Scope
	ExpectedVersion                     int32
}

type UpdateTemplate struct {
	templates TemplateRepository
}

func NewUpdateTemplate(templates TemplateRepository) *UpdateTemplate {
	return &UpdateTemplate{templates: templates}
}

func (uc *UpdateTemplate) Execute(ctx context.Context, in UpdateTemplateInput) (domain.WorkflowTemplate, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}
	existing, err := uc.templates.GetTemplate(ctx, tenantID, in.ID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "template does not exist", nil)
		}
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_LOOKUP_FAILED", "failed to look up template", err)
	}

	next, err := domain.NewWorkflowTemplate(in.ID, tenantID, in.Name, in.DAGJSON, in.Scope, in.ParentTemplateID)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
	}

	// Cycle re-validation — template.go's ErrTemplateSelfParent doc comment
	// used to reason this was unreachable because no UpdateTemplate existed
	// to rewire a parent after creation. That premise no longer holds: walk
	// the NEW parent's own chain and reject if in.ID appears in it (a
	// multi-hop cycle), not just the direct self-parent case
	// NewWorkflowTemplate already checks.
	if in.ParentTemplateID != "" {
		chain, err := uc.templates.ResolveChain(ctx, tenantID, in.ParentTemplateID, maxTemplateChainDepth)
		if err != nil {
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_CHAIN_FAILED", "failed to resolve parent chain", err)
		}
		for _, ancestor := range chain {
			if ancestor.ID == in.ID {
				return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_TEMPLATE_CYCLE", "update would create a cyclic parent chain", nil)
			}
		}
	}

	// isBreaking/hasActiveUsage/bump (TASK-WF-004-03): a DIFFERENT concern
	// from the DefinitionSnapshot-freezes-at-Execute-time reasoning above —
	// this doesn't guard against anything, it only decides whether this
	// write should also bump templates.version, as a signal that a
	// breaking edit landed while executions were actively using this
	// template (surfaced to a caller diffing versions, not enforced here).
	isBreaking, err := isBreakingChange(existing, next)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
	}
	hasActiveUsage, err := uc.templates.HasActiveExecutionsUsingTemplate(ctx, tenantID, in.ID)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_USAGE_CHECK_FAILED", "failed to check active template usage", err)
	}
	bump := isBreaking && hasActiveUsage

	updated, err := uc.templates.Update(ctx, next, in.ExpectedVersion, bump)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateVersionConflict) {
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_TEMPLATE_VERSION_CONFLICT", "template was modified by another request", err)
		}
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_UPDATE_TEMPLATE_FAILED", "failed to update template", err)
	}
	// No HasActiveExecutions-style write-blocking guard needed —
	// DefinitionSnapshot freezes at Execute time (workflow-service.md §4),
	// so this update can never retroactively change a running execution's
	// behavior. HasActiveExecutionsUsingTemplate above serves the
	// unrelated version-bump decision, not a guard against this write.
	return updated, nil
}

// isBreakingChange compares existing vs. next's parsed DAGs: a step
// removed, a step's type changed, or a step's dependsOn edges changed —
// the proposed starting definition. Product sign-off on this exact
// definition is a merge blocker per this task's own header note, not
// settled by this implementation.
func isBreakingChange(existing, next domain.WorkflowTemplate) (bool, error) {
	oldDAG, err := domain.ParseDAG(existing.DAGJSON)
	if err != nil {
		return false, err
	}
	newDAG, err := domain.ParseDAG(next.DAGJSON)
	if err != nil {
		return false, err
	}
	return diffIsBreaking(oldDAG, newDAG), nil
}

// diffIsBreaking implements isBreakingChange's actual diff: builds a
// step-id-keyed index of oldDAG, then for every old step checks whether it
// survives into newDAG unchanged in Type and DependsOn (order-independent
// set comparison — reordering dependsOn entries is not itself a breaking
// change). A step present in newDAG but absent from oldDAG (a pure
// addition) is never breaking on its own.
func diffIsBreaking(oldDAG, newDAG domain.DAGDefinition) bool {
	newByID := make(map[string]domain.Step, len(newDAG.Steps))
	for _, s := range newDAG.Steps {
		newByID[s.ID] = s
	}
	for _, oldStep := range oldDAG.Steps {
		newStep, stillExists := newByID[oldStep.ID]
		if !stillExists {
			return true // step removed
		}
		if oldStep.Type != newStep.Type {
			return true // step's type changed
		}
		if !sameDependsOnSet(oldStep.DependsOn, newStep.DependsOn) {
			return true // step's dependency edges changed
		}
	}
	return false
}

// sameDependsOnSet compares two DependsOn lists as sets (order-independent,
// duplicates collapsed) — DAGDefinition.Validate already rejects duplicate
// step ids, but dependsOn entries aren't similarly deduplicated elsewhere,
// so this normalizes rather than assuming.
func sameDependsOnSet(a, b []string) bool {
	setA := make(map[string]struct{}, len(a))
	for _, id := range a {
		setA[id] = struct{}{}
	}
	setB := make(map[string]struct{}, len(b))
	for _, id := range b {
		setB[id] = struct{}{}
	}
	if len(setA) != len(setB) {
		return false
	}
	for id := range setA {
		if _, ok := setB[id]; !ok {
			return false
		}
	}
	return true
}
