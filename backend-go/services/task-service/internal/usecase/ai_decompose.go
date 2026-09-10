package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// AIDecomposeInput mirrors the AIDecompose RPC request. TenantID/UserID
// aren't fields here — TenantID comes from context like every other
// usecase in this package (common/tenant.RequireTenantID), and UserID is
// the acting caller's own identity (tenant.UserID), not an
// arbitrary-target-user field the way ResolvePermissionInput.UserID is.
type AIDecomposeInput struct {
	TaskID string
}

// AIDecompose relays a task to the Dev Server Agent's ai.complete method
// (via infra-fleet-service's Relay RPC, see AICompleter) to propose a
// subtask breakdown — review-before-commit: the result is not written to
// task_edges until a subsequent AIApply call (TASK-224).
type AIDecompose struct {
	tasks      TaskRepository
	aiProvider AIProviderContextResolver
	resolver   ProjectExecutionResolver
	relay      AICompleter
	techStack  TechStackDetector
}

func NewAIDecompose(tasks TaskRepository, aiProvider AIProviderContextResolver, resolver ProjectExecutionResolver, relay AICompleter, techStack TechStackDetector) *AIDecompose {
	return &AIDecompose{tasks: tasks, aiProvider: aiProvider, resolver: resolver, relay: relay, techStack: techStack}
}

func (uc *AIDecompose) Execute(ctx context.Context, in AIDecomposeInput) ([]domain.SubtaskProposal, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	userID, _ := tenant.UserID(ctx)

	task, err := uc.tasks.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	providerCtx, err := uc.aiProvider.ResolveContext(ctx, tenantID, userID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_PROVIDER_RESOLVE_FAILED", "failed to resolve AI provider context", err)
	}
	// worktreePath (2nd return) isn't needed here — AIDecompose relays
	// through AICompleter's ai.complete method, which has no worktreePath
	// concept (see AICompleter's doc comment); only SimpleExecutor's
	// agent.execPrompt call needs it (TASK-224 Gap 1).
	connectionID, _, connected, err := uc.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil || !connected {
		// A not-connected project is a real error, never a silent empty
		// proposal list — see TASK-226's regression test for this.
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "TASK_AI_DECOMPOSE_NO_CONNECTION", "task's project has no connected dev server for AI relay", err)
	}
	decCtx := uc.buildContext(ctx, tenantID, task)
	prompt := buildDecomposePrompt(decCtx, providerCtx)
	content, err := uc.relay.Complete(ctx, connectionID, prompt)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_FAILED", "failed to generate subtask proposals via AI relay", err)
	}
	proposals, err := parseSubtaskProposals(content)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_PARSE_FAILED", "AI response was not valid JSON", err)
	}
	return proposals, nil
}

// decomposeContext is BE-SOL-002's 5-source context bundle: the task's own
// title/description/AI context, a coarse tech-stack hint, and the titles of
// any subtasks that already exist (so the AI doesn't propose duplicates).
type decomposeContext struct {
	Title            string
	Description      string
	AIContext        string
	TechStack        []string
	ExistingSubtasks []string
}

// buildContext assembles the 5-source decomposeContext. TechStack/
// ExistingSubtasks are both best-effort: a failure fetching either must
// never fail AIDecompose itself, since neither is essential to producing a
// (possibly less-informed) subtask breakdown.
func (uc *AIDecompose) buildContext(ctx context.Context, tenantID string, task domain.Task) decomposeContext {
	stack, _ := uc.techStack.Detect(ctx, task.ProjectID)
	existing, _ := uc.tasks.ListChildren(ctx, tenantID, task.ID)
	titles := make([]string, 0, len(existing))
	for _, t := range existing {
		titles = append(titles, t.Title)
	}
	return decomposeContext{
		Title:            task.Title,
		Description:      task.Description,
		AIContext:        string(task.AIContext),
		TechStack:        stack,
		ExistingSubtasks: titles,
	}
}

// decomposeJSONSchemaExample is appended to every decompose prompt so the AI
// relay's free-text response comes back as an array parseSubtaskProposals
// can actually parse — BE-SOL-002's structured-proposal design supersedes
// the old plain numbered-list format. depends_on_index entries are indices
// into this SAME array (not real task IDs, which don't exist yet) — see
// domain.SubtaskProposal.DependsOnIndex's doc comment for the exact
// semantics AIApply resolves them with.
const decomposeJSONSchemaExample = `Respond with ONLY a JSON array (no prose, no markdown fences) of objects shaped like:
[{"title": "string", "description": "string", "type": "string", "estimated_hours": 2.5, "depends_on_index": [0], "prompt_template": "string"}]
Every field except "title" is optional. depends_on_index lists indices (into this same array) of subtasks this one depends on.`

// buildDecomposePrompt assembles the ai.complete prompt from decCtx's 5
// context sources and providerCtx. providerCtx is interpolated for
// traceability (which provider/account resolved) — a distinct,
// already-correct piece of context this widening doesn't remove.
func buildDecomposePrompt(decCtx decomposeContext, providerCtx string) string {
	var b strings.Builder
	b.WriteString("Break the following task down into a set of concrete subtasks. ")
	b.WriteString(decomposeJSONSchemaExample)
	b.WriteString("\n\nTask: ")
	b.WriteString(decCtx.Title)
	if decCtx.Description != "" {
		b.WriteString("\nDescription: ")
		b.WriteString(decCtx.Description)
	}
	if decCtx.AIContext != "" {
		b.WriteString("\nContext: ")
		b.WriteString(decCtx.AIContext)
	}
	if len(decCtx.TechStack) > 0 {
		b.WriteString("\nTech stack: ")
		b.WriteString(strings.Join(decCtx.TechStack, ", "))
	}
	if len(decCtx.ExistingSubtasks) > 0 {
		b.WriteString("\nExisting subtasks (do not duplicate these): ")
		b.WriteString(strings.Join(decCtx.ExistingSubtasks, "; "))
	}
	if providerCtx != "" {
		b.WriteString("\nProvider context: ")
		b.WriteString(providerCtx)
	}
	return b.String()
}

// subtaskProposalJSON is the wire shape parseSubtaskProposals unmarshals
// the AI relay's JSON array response into, kept separate from
// domain.SubtaskProposal so the domain package stays free of json struct
// tags (architecture/03's zero-non-domain-imports rule is about imports,
// not tags, but keeping wire-format concerns entirely in the usecase layer
// matches this codebase's existing convention of no `json:"..."` tags
// anywhere under internal/domain).
type subtaskProposalJSON struct {
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Type           string   `json:"type"`
	EstimatedHours *float64 `json:"estimated_hours"`
	DependsOnIndex []int    `json:"depends_on_index"`
	PromptTemplate string   `json:"prompt_template"`
}

// parseSubtaskProposals parses ai.complete's JSON-array response into
// SubtaskProposals. This is an explicit-error contract, unlike the old
// free-text numbered-list parser it replaces (BE-SOL-002's structured
// proposal format): malformed/non-JSON AI output is a real error, never a
// silent partial/empty result — AIDecompose.Execute propagates it rather
// than returning zero proposals as if the AI legitimately proposed nothing.
func parseSubtaskProposals(raw string) ([]domain.SubtaskProposal, error) {
	var parsed []subtaskProposalJSON
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("ai_decompose: AI response is not valid JSON: %w", err)
	}
	out := make([]domain.SubtaskProposal, 0, len(parsed))
	for _, p := range parsed {
		out = append(out, domain.SubtaskProposal{
			Title:          p.Title,
			Description:    p.Description,
			Type:           p.Type,
			EstimatedHours: p.EstimatedHours,
			DependsOnIndex: p.DependsOnIndex,
			PromptTemplate: p.PromptTemplate,
		})
	}
	return out, nil
}
