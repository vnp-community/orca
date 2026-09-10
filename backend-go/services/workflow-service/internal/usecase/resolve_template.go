package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
//
// TASK-WF-004-01 adds a second pass, strictly AFTER base-selection above —
// base-selection itself is UNCHANGED, per that task's own must-preserve
// constraint. The fold applies every descendant-of-base template's
// Overrides/InjectSteps/RemoveSteps onto base's DAG, in root-to-requested
// (chain) order — farther-from-base overrides apply first, closer ones
// apply last and therefore win, matching BE-SOL-004's stated intent. A
// template using none of the 3 new fields (every template that predates
// this pass) makes every apply* call a true no-op — see this function's
// own regression test in resolve_template_test.go.
func resolveEffectiveTemplate(chain []domain.WorkflowTemplate) (domain.WorkflowTemplate, error) {
	var base domain.WorkflowTemplate
	found := false
	for i := len(chain) - 1; i >= 0; i-- {
		dag, err := domain.ParseDAG(chain[i].DAGJSON)
		if err != nil {
			return domain.WorkflowTemplate{}, err
		}
		if len(dag.Steps) > 0 {
			base, found = chain[i], true
			break
		}
	}
	if !found {
		base = chain[len(chain)-1]
	}

	dag, err := domain.ParseDAG(base.DAGJSON)
	if err != nil {
		return domain.WorkflowTemplate{}, err
	}

	baseIdx := indexOfTemplateID(chain, base.ID)
	folded := false
	for i := baseIdx + 1; i < len(chain); i++ {
		tpl := chain[i]
		if len(tpl.Overrides) == 0 && len(tpl.InjectSteps) == 0 && len(tpl.RemoveSteps) == 0 {
			continue
		}
		folded = true
		dag, err = applyOverrides(dag, tpl.Overrides)
		if err != nil {
			return domain.WorkflowTemplate{}, err
		}
		dag = applyInjections(dag, tpl.InjectSteps)
		dag = applyRemovals(dag, tpl.RemoveSteps)
	}
	if !folded {
		// No descendant-of-base template used any of the 3 new fields —
		// return base completely untouched (not even round-tripped through
		// ParseDAG->Serialize) so a byte-for-byte-DAGJSON-comparing caller
		// sees identical behavior to before this task, per this task's own
		// must-preserve regression requirement.
		return base, nil
	}

	serialized, err := dag.Serialize()
	if err != nil {
		return domain.WorkflowTemplate{}, err
	}
	base.DAGJSON = serialized
	return base, nil
}

// indexOfTemplateID returns the index of the template with id within
// chain, or -1 if absent (can't happen in practice — base is always an
// element of chain, ResolveChain's own contract — but a defensive -1
// keeps the caller's arithmetic from silently misbehaving rather than
// asserting an invariant that should always hold).
func indexOfTemplateID(chain []domain.WorkflowTemplate, id string) int {
	for i, tpl := range chain {
		if tpl.ID == id {
			return i
		}
	}
	return -1
}

// applyOverrides applies a dot-path override map onto dag's steps —
// key format is "<stepId>.<field>", addressing exactly one field on one
// step (deeper nesting into a step's Config, e.g. "<stepId>.config.prompt",
// is not supported by this pass; BE-SOL-004 only requires "changing one
// step's prompt" to work, and Config is StepType-specific raw JSON this
// package deliberately doesn't parse generically — see domain.Step's doc
// comment). An override naming a step id absent from dag.Steps is a
// template-author error, not a silent no-op — surfaced as an error so a
// stale override (e.g. after the base template's step was renamed) is
// caught at resolve time, not silently ignored.
//
// Supported top-level Step fields: "type" and "dependsOn" (as a
// comma-separated string) in addition to nested Config field paths of the
// form "<stepId>.<configField>", applied by unmarshaling Config to
// map[string]any, setting configField, and re-marshaling — Config's exact
// shape is StepType-specific, so this is a best-effort generic set, not a
// typed one.
func applyOverrides(dag domain.DAGDefinition, overrides map[string]any) (domain.DAGDefinition, error) {
	if len(overrides) == 0 {
		return dag, nil
	}
	for path, value := range overrides {
		stepID, field, ok := strings.Cut(path, ".")
		if !ok {
			return dag, fmt.Errorf("resolve_template: override key %q missing a field after the step id", path)
		}
		idx := -1
		for i, s := range dag.Steps {
			if s.ID == stepID {
				idx = i
				break
			}
		}
		if idx == -1 {
			return dag, fmt.Errorf("resolve_template: override references unknown step %q", stepID)
		}
		if err := setStepField(&dag.Steps[idx], field, value); err != nil {
			return dag, fmt.Errorf("resolve_template: override %q: %w", path, err)
		}
	}
	return dag, nil
}

// setStepField applies one override value onto step — either a known
// top-level Step field, or (any other field name) a key inside step's
// Config JSON object, set via a generic map[string]any round-trip.
func setStepField(step *domain.Step, field string, value any) error {
	switch field {
	case "dependsOn":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("dependsOn override must be a string")
		}
		if s == "" {
			step.DependsOn = nil
			return nil
		}
		parts := strings.Split(s, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		step.DependsOn = parts
		return nil
	default:
		var cfg map[string]any
		if len(step.Config) > 0 {
			if err := json.Unmarshal(step.Config, &cfg); err != nil {
				return fmt.Errorf("step config is not a JSON object, cannot apply field override %q: %w", field, err)
			}
		}
		if cfg == nil {
			cfg = make(map[string]any)
		}
		cfg[field] = value
		raw, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("re-marshal step config after override: %w", err)
		}
		step.Config = raw
		return nil
	}
}

// applyInjections inserts each injection.Step into dag.Steps adjacent to
// injection.AnchorStepID, per injection.Position ("before"/"after"). An
// injection whose AnchorStepID doesn't exist in dag.Steps is skipped
// (not an error) — an injected step is additive, optional enrichment from
// a descendant template; a stale anchor (e.g. the parent later renamed
// that step) shouldn't hard-fail resolution the way a stale override does
// (overrides name a step the descendant explicitly expects to already
// exist and mutate; an injection just adds something new, so "anchor
// vanished" degrading to "injection skipped" is the safer default).
func applyInjections(dag domain.DAGDefinition, injections []domain.StepInjection) domain.DAGDefinition {
	for _, inj := range injections {
		anchorIdx := -1
		for i, s := range dag.Steps {
			if s.ID == inj.AnchorStepID {
				anchorIdx = i
				break
			}
		}
		if anchorIdx == -1 {
			continue
		}
		insertAt := anchorIdx + 1 // "after" (default)
		if inj.Position == "before" {
			insertAt = anchorIdx
		}
		steps := make([]domain.Step, 0, len(dag.Steps)+1)
		steps = append(steps, dag.Steps[:insertAt]...)
		steps = append(steps, inj.Step)
		steps = append(steps, dag.Steps[insertAt:]...)
		dag.Steps = steps
	}
	return dag
}

// applyRemovals removes every step named in removeStepIDs from dag.Steps
// AND strips those ids from every remaining step's DependsOn — pruning
// dangling dependsOn entries silently rather than leaving them for
// DAGDefinition.Validate's ErrStepDependencyNotFound to catch later.
// Chosen over "removal + missing dependency = template author error"
// because a removeSteps entry is explicitly declaring "this step should
// no longer exist in the resolved DAG," which implies its edges should go
// with it — a descendant template author removing a step already
// expresses clear intent; forcing them to also separately edit every
// sibling's dependsOn (on a parent template they may not own) would make
// this feature much less usable for its stated purpose.
func applyRemovals(dag domain.DAGDefinition, removeStepIDs []string) domain.DAGDefinition {
	if len(removeStepIDs) == 0 {
		return dag
	}
	toRemove := make(map[string]bool, len(removeStepIDs))
	for _, id := range removeStepIDs {
		toRemove[id] = true
	}

	kept := make([]domain.Step, 0, len(dag.Steps))
	for _, s := range dag.Steps {
		if toRemove[s.ID] {
			continue
		}
		if len(s.DependsOn) > 0 {
			prunedDeps := make([]string, 0, len(s.DependsOn))
			for _, dep := range s.DependsOn {
				if !toRemove[dep] {
					prunedDeps = append(prunedDeps, dep)
				}
			}
			s.DependsOn = prunedDeps
		}
		kept = append(kept, s)
	}
	dag.Steps = kept
	return dag
}
