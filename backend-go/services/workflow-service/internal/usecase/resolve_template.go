package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

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
	return uc.Resolve(ctx, tenantID, in.TemplateID)
}

// Resolve is Execute's tenant-parameterized core — bypasses ctx's
// tenant-scoping so usecase.ImportSharedTemplate can resolve a template
// chain against the SOURCE template's own tenant_id (read off the
// share-token lookup), not the importing caller's tenant. Every
// same-tenant caller should go through Execute instead; this exists only
// for that one deliberate cross-tenant case (see ImportSharedTemplate's
// doc comment) — do not weaken Execute's ctx-derived tenantID to a
// caller-supplied param for the normal path.
func (uc *ResolveTemplate) Resolve(ctx context.Context, tenantID, templateID string) (ResolveTemplateOutput, error) {
	if templateID == "" {
		return ResolveTemplateOutput{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_TEMPLATE_ID_REQUIRED", "template_id is required", nil)
	}

	chain, err := uc.repo.ResolveChain(ctx, tenantID, templateID, maxTemplateChainDepth)
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

	effectiveSteps, err := resolveEffectiveTemplate(chain)
	if err != nil {
		return ResolveTemplateOutput{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
	}

	// The effective template carries the REQUESTED template's own identity
	// (id, tenant, owner, scope, ...) — ResolveTemplate answers "what does
	// this template effectively look like," not "which ancestor row won" —
	// with dag_json replaced by the field-level-merged result.
	dagJSON, err := json.Marshal(domain.DAGDefinition{Steps: effectiveSteps})
	if err != nil {
		return ResolveTemplateOutput{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_MARSHAL_FAILED", "failed to marshal resolved dag", err)
	}
	effective := chain[len(chain)-1]
	effective.DAGJSON = string(dagJSON)

	return ResolveTemplateOutput{Template: effective, Chain: chain}, nil
}

// resolveEffectiveTemplate walks chain root-first (chain[0] = topmost
// ancestor, per ResolveChain's existing contract) and folds each level's
// own steps/overrides/inject_steps/remove_steps onto an accumulator —
// BUG-WF-01's field-level deepMerge, replacing the old
// nearest-non-empty-ancestor-wins policy. A chain with none of the three
// Inherit-mode fields set anywhere at any level resolves identically to
// the old policy: each level's own non-empty steps still fully replace the
// accumulator (same "own steps win" rule, now scoped per-level instead of
// picked wholesale from one ancestor).
func resolveEffectiveTemplate(chain []domain.WorkflowTemplate) ([]domain.Step, error) {
	acc := parseSteps(chain[0].DAGJSON) // topmost ancestor's own definition, may be empty
	for _, level := range chain[1:] {
		if steps := parseSteps(level.DAGJSON); len(steps) > 0 {
			acc = steps // own steps fully replace — same rule the old policy
			// had, now scoped per-level instead of whole-chain
		}
		acc = removeSteps(acc, level.RemoveStepsJSON)
		acc = applyOverrides(acc, level.OverridesJSON)
		acc = append(acc, parseInjectSteps(level.InjectStepsJSON)...)
	}
	dag := domain.DAGDefinition{Steps: acc}
	if err := dag.Validate(); err != nil {
		return nil, err // mirrors the existing parse-failure error mapping above
	}
	return acc, nil
}

// parseSteps is a best-effort DAGJSON->[]Step parse: DAGJSON is validated
// (parseable + Validate()-clean) at every write via domain.NewWorkflowTemplate,
// so a parse failure here would mean corrupted data, not a normal runtime
// condition — treated as "no steps" rather than propagated, matching how
// an empty dag_json already means "no steps" (see domain.ParseDAG).
func parseSteps(dagJSON string) []domain.Step {
	dag, err := domain.ParseDAG(dagJSON)
	if err != nil {
		return nil
	}
	return dag.Steps
}

// parseStepsByID is parseSteps indexed by step id — the lookup shape
// isBreakingChange (update_template.go) needs to answer "does this id
// still exist, and with the same Type" in O(1) per old step.
func parseStepsByID(dagJSON string) map[string]domain.Step {
	steps := parseSteps(dagJSON)
	byID := make(map[string]domain.Step, len(steps))
	for _, s := range steps {
		byID[s.ID] = s
	}
	return byID
}

// parseInjectSteps parses an Inherit-mode inject_steps instruction
// ([]domain.Step, JSON) — malformed/empty parses to no steps, same
// best-effort convention as parseSteps.
func parseInjectSteps(injectStepsJSON string) []domain.Step {
	if strings.TrimSpace(injectStepsJSON) == "" {
		return nil
	}
	var steps []domain.Step
	if err := json.Unmarshal([]byte(injectStepsJSON), &steps); err != nil {
		return nil
	}
	return steps
}

// removeSteps drops any step whose id appears in removeStepsJSON
// (an Inherit-mode instruction: []string of step ids), and also strips
// that id from every remaining step's DependsOn — a removed step's
// dependents lose that edge rather than becoming permanently unsatisfiable
// (dangling-dependency Validate() failure).
func removeSteps(steps []domain.Step, removeStepsJSON string) []domain.Step {
	if strings.TrimSpace(removeStepsJSON) == "" {
		return steps
	}
	var ids []string
	if err := json.Unmarshal([]byte(removeStepsJSON), &ids); err != nil || len(ids) == 0 {
		return steps
	}
	removed := make(map[string]bool, len(ids))
	for _, id := range ids {
		removed[id] = true
	}

	out := make([]domain.Step, 0, len(steps))
	for _, s := range steps {
		if removed[s.ID] {
			continue
		}
		if len(s.DependsOn) > 0 {
			deps := make([]string, 0, len(s.DependsOn))
			for _, dep := range s.DependsOn {
				if !removed[dep] {
					deps = append(deps, dep)
				}
			}
			s.DependsOn = deps
		}
		out = append(out, s)
	}
	return out
}

// applyOverrides unmarshals overridesJSON as map[string]json.RawMessage
// (step id -> partial JSON object) and, for each step whose id has an
// entry, merges that object's top-level keys into the step's own Config —
// a one-level-deep merge, not a recursive deep-merge into nested config
// structures (nested-key override is a documented non-goal).
func applyOverrides(steps []domain.Step, overridesJSON string) []domain.Step {
	if strings.TrimSpace(overridesJSON) == "" {
		return steps
	}
	var overrides map[string]json.RawMessage
	if err := json.Unmarshal([]byte(overridesJSON), &overrides); err != nil || len(overrides) == 0 {
		return steps
	}

	out := make([]domain.Step, len(steps))
	copy(out, steps)
	for i, s := range out {
		raw, ok := overrides[s.ID]
		if !ok {
			continue
		}
		out[i].Config = mergeConfigOneLevel(s.Config, raw)
	}
	return out
}

// mergeConfigOneLevel merges overrideRaw's top-level keys onto base,
// overwriting on conflict — a shallow merge; nested objects within a
// shared key are replaced wholesale, not recursively merged. Falls back to
// base unchanged if overrideRaw isn't a JSON object (e.g. malformed —
// unreachable in practice since domain.WithOverrides already rejects
// malformed JSON at construction time, but defensive here too).
func mergeConfigOneLevel(base, overrideRaw json.RawMessage) json.RawMessage {
	var overrideMap map[string]json.RawMessage
	if err := json.Unmarshal(overrideRaw, &overrideMap); err != nil {
		return base
	}

	merged := map[string]json.RawMessage{}
	if len(base) > 0 {
		_ = json.Unmarshal(base, &merged)
	}
	for k, v := range overrideMap {
		merged[k] = v
	}

	out, err := json.Marshal(merged)
	if err != nil {
		return base
	}
	return out
}
