package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// maxTemplateChainDepth mirrors workflow-service.md §6's recursive-CTE
// depth cap exactly ("shallow and depth-bounded (5)").
const maxTemplateChainDepth = 5

type ResolveTemplateInput struct {
	TemplateID string
}

type ResolveTemplateOutput struct {
	// Template is the EFFECTIVE, post-inheritance template — see this
	// type's construction in Execute for the resolution policy.
	Template domain.WorkflowTemplate
	// Chain is every template ResolveChain walked, root-first (index 0 =
	// topmost ancestor, last = the requested template itself) — for
	// callers that want to show the inheritance path, not just the answer.
	Chain []domain.WorkflowTemplate
}

// ResolveTemplate walks TemplateID's parent_template_id chain and returns
// the effective template — the other half of the last item Epic C left
// deferred (docs/execution-plan.md §2/§10), implemented together with
// ListTemplates, 2026-08-17.
//
// Resolution policy (a deliberate, documented choice — workflow-service.md
// §6 specifies the recursive-query shape but not a merge policy): walk
// from TemplateID up its ancestors, depth<=5, and return the CLOSEST
// (most-specific-first) template in that chain whose dag_json defines at
// least one step. This means a personal template that exists only to opt
// into its team/company parent's steps (an empty dag_json) correctly
// inherits from that parent, rather than resolving to "no steps." A
// template with its own steps always wins over any ancestor, regardless
// of scope tier — Scope (company/team/personal) is a classification field
// on each row, not itself part of the resolution algorithm (§4 doesn't
// specify company overriding team overriding personal or vice versa; only
// specificity-in-the-chain matters here).
type ResolveTemplate struct {
	repo TemplateRepository
}

func NewResolveTemplate(repo TemplateRepository) *ResolveTemplate {
	return &ResolveTemplate{repo: repo}
}

func (uc *ResolveTemplate) Execute(ctx context.Context, in ResolveTemplateInput) (ResolveTemplateOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ResolveTemplateOutput{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}
	if in.TemplateID == "" {
		return ResolveTemplateOutput{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_TEMPLATE_ID_REQUIRED", "template_id is required", nil)
	}

	chain, err := uc.repo.ResolveChain(ctx, tenantID, in.TemplateID, maxTemplateChainDepth)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return ResolveTemplateOutput{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "workflow template not found", err)
		}
		return ResolveTemplateOutput{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_RESOLVE_FAILED", "failed to resolve workflow template chain", err)
	}
	if len(chain) == 0 {
		// Defensive — ResolveChain's own contract says it returns
		// ErrTemplateNotFound instead of an empty chain, but an empty
		// result handled here as NotFound (rather than an index panic
		// below) keeps this usecase safe even if an adapter's contract is
		// violated in the future.
		return ResolveTemplateOutput{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "workflow template not found", nil)
	}

	effective, err := resolveEffectiveTemplate(chain)
	if err != nil {
		return ResolveTemplateOutput{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
	}

	return ResolveTemplateOutput{Template: effective, Chain: chain}, nil
}

// resolveEffectiveTemplate implements ResolveTemplate's documented policy:
// walk chain from its LAST element (the requested template, most specific)
// back toward chain[0] (the topmost ancestor), returning the first
// template whose dag_json defines at least one step. If none in the chain
// has any steps, the requested template itself (chain's last element) is
// returned as-is — an empty-but-valid template, not an error: a template
// with genuinely no steps anywhere in its chain is a real, if useless,
// answer, not a failure to resolve.
func resolveEffectiveTemplate(chain []domain.WorkflowTemplate) (domain.WorkflowTemplate, error) {
	for i := len(chain) - 1; i >= 0; i-- {
		dag, err := domain.ParseDAG(chain[i].DAGJSON)
		if err != nil {
			return domain.WorkflowTemplate{}, err
		}
		if len(dag.Steps) > 0 {
			return chain[i], nil
		}
	}
	return chain[len(chain)-1], nil
}
