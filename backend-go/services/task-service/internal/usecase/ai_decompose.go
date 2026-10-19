package usecase

import (
	"context"
	"encoding/json"

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

// AIDecomposeResult carries both the parsed proposals AND the raw AI
// response — RawResponse is persisted to Task.AIPlanJSON by the grpc
// wiring layer (TASK-TG-02-06) so a parse failure still leaves the caller
// something to inspect, and so AIApply's echoed raw_ai_response has a
// source.
type AIDecomposeResult struct {
	Proposals   []domain.SubtaskProposal
	RawResponse string
}

// AIDecompose relays a task to the Dev Server Agent's ai.complete method
// (via infra-fleet-service's Relay RPC, see AICompleter) to propose a
// structured subtask breakdown — review-before-commit: the result is not
// written to task_edges until a subsequent AIApply call (TASK-224). Builds
// a five-source context bundle (task detail, project, tech stack, existing
// subtasks, team velocity) per SOL-TG-02.
type AIDecompose struct {
	tasks      TaskRepository
	edges      EdgeRepository
	aiProvider AIProviderContextResolver
	resolver   ProjectExecutionResolver
	projectCtx ProjectInfoResolver
	techStack  TechStackDetector
	velocity   VelocityResolver
	relay      AICompleter
}

func NewAIDecompose(
	tasks TaskRepository, edges EdgeRepository, aiProvider AIProviderContextResolver,
	resolver ProjectExecutionResolver, projectCtx ProjectInfoResolver,
	techStack TechStackDetector, velocity VelocityResolver, relay AICompleter,
) *AIDecompose {
	return &AIDecompose{tasks: tasks, edges: edges, aiProvider: aiProvider, resolver: resolver, projectCtx: projectCtx, techStack: techStack, velocity: velocity, relay: relay}
}

func (uc *AIDecompose) Execute(ctx context.Context, in AIDecomposeInput) (AIDecomposeResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return AIDecomposeResult{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	userID, _ := tenant.UserID(ctx)

	task, err := uc.tasks.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return AIDecomposeResult{}, taskRepoError(err, "task not found", "TASK_AI_DECOMPOSE_TASK_LOAD_FAILED", "failed to load task")
	}
	providerCtx, err := uc.aiProvider.ResolveContext(ctx, tenantID, userID)
	if err != nil {
		return AIDecomposeResult{}, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_PROVIDER_RESOLVE_FAILED", "failed to resolve AI provider context", err)
	}
	// worktreePath/worktreeID (2nd/3rd returns) aren't needed here —
	// AIDecompose relays through AICompleter's ai.complete method, which has
	// no worktreePath concept (see AICompleter's doc comment); only
	// SimpleExecutor's agent.execPrompt call needs worktreePath (TASK-224
	// Gap 1) and TechStackDetector needs worktreeID (its own resolve call
	// below, not this one).
	connectionID, _, _, _, connected, err := uc.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil || !connected {
		// A not-connected project is a real error, never a silent empty
		// proposal list — see TASK-226's regression test for this.
		return AIDecomposeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_AI_DECOMPOSE_NO_CONNECTION", "task's project has no connected dev server for AI relay", err)
	}

	existingEdges, err := uc.edges.ListFrom(ctx, tenantID, in.TaskID, domain.EdgeKindParentChild)
	if err != nil {
		return AIDecomposeResult{}, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_SUBTASK_LOOKUP_FAILED", "failed to list existing subtasks", err)
	}
	existingTitles := make([]string, 0, len(existingEdges))
	for _, e := range existingEdges {
		if t, err := uc.tasks.Get(ctx, tenantID, e.ToTaskID); err == nil {
			existingTitles = append(existingTitles, t.Title)
		}
	}

	projectName, repoURL, err := uc.projectCtx.Resolve(ctx, tenantID, task.ProjectID)
	if err != nil {
		return AIDecomposeResult{}, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_PROJECT_RESOLVE_FAILED", "failed to resolve project context", err)
	}
	// Best-effort: a tech-stack detection failure degrades prompt richness,
	// never blocks decompose.
	stack, _ := uc.techStack.Detect(ctx, tenantID, task.ProjectID)
	velocityTasks, _ := uc.velocity.RecentCompletedTasks(ctx, tenantID, task.ProjectID, 10)

	prompt, err := renderDecomposePrompt(task, providerCtx, projectName, repoURL, stack, existingTitles, velocityTasks)
	if err != nil {
		return AIDecomposeResult{}, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_PROMPT_BUILD_FAILED", "failed to build the decompose prompt", err)
	}
	content, err := uc.relay.Complete(ctx, connectionID, prompt)
	if err != nil {
		return AIDecomposeResult{}, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_FAILED", "failed to generate subtask proposals via AI relay", err)
	}
	proposals, err := parseSubtaskProposalsJSON(content)
	if err != nil {
		return AIDecomposeResult{}, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_INVALID_JSON", "AI response was not valid JSON", err)
	}
	return AIDecomposeResult{Proposals: proposals, RawResponse: content}, nil
}

// subtaskProposalsWire mirrors the prompt's requested JSON shape 1:1.
type subtaskProposalsWire struct {
	Subtasks []struct {
		Title          string   `json:"title"`
		Description    string   `json:"description"`
		Type           string   `json:"type"`
		EstimatedHours *float64 `json:"estimated_hours"`
		DependsOn      []int    `json:"depends_on"`
		PromptTemplate string   `json:"prompt_template"`
	} `json:"subtasks"`
	Notes string `json:"notes"`
}

// parseSubtaskProposalsJSON unmarshals the AI's structured JSON response —
// replaces the prior numbered-list parser entirely (no live wire-format
// consumer to keep backward compatible, per BUG-034).
func parseSubtaskProposalsJSON(content string) ([]domain.SubtaskProposal, error) {
	var wire subtaskProposalsWire
	if err := json.Unmarshal([]byte(content), &wire); err != nil {
		return nil, err
	}
	out := make([]domain.SubtaskProposal, 0, len(wire.Subtasks))
	for _, s := range wire.Subtasks {
		out = append(out, domain.SubtaskProposal{
			Title: s.Title, Description: s.Description, Type: s.Type,
			EstimatedHours: s.EstimatedHours, DependsOnIndices: s.DependsOn, PromptTemplate: s.PromptTemplate,
		})
	}
	return out, nil
}
