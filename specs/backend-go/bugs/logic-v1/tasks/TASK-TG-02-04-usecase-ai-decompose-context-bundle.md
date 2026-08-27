# TASK-TG-02-04: `AIDecompose` — five-source context bundle + structured JSON parsing

**From Solution:** SOL-TG-02
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/ai_decompose.go`
**Depends on:** TASK-TG-01-04 (Task.Description/AIContext/EstimatedHours columns), TASK-TG-02-02 (domain SubtaskProposal widen), TASK-TG-02-03 (TechStackDetector)
**Status:** `[x]` DONE — AIDecompose rewritten for the 5-source context bundle + structured JSON parsing; new ProjectContextResolver (project-service adapter) and VelocityResolver (repo-local, RecentCompletedTasks) added and wired in main.go; go test ./internal/usecase/... -run TestAIDecompose passes (prompt-source-inclusion, best-effort tech-stack-failure, malformed-JSON cases).

---

## Context

`buildDecomposePrompt` today only interpolates `task.Title` and
`providerCtx`. The response is parsed as a numbered plain-text list
(`parseSubtaskProposals`). This task widens the prompt to a 5-source bundle
(task detail, project, tech stack, existing subtasks, team velocity) and
switches the wire format to structured JSON — replacing the numbered-list
parser entirely, since `AIDecompose`/`AIApply` have no live wscompat/REST
callers yet (per BUG-034) so there's no backward-compatibility concern.

## Changes to make

Add `ProjectContextResolver` and `VelocityResolver` ports to
`backend-go/services/task-service/internal/usecase/ports.go`:

```go
// ProjectContextResolver resolves a project's name/repo URL via
// project-service — task-service never reads project-service's tables
// directly.
type ProjectContextResolver interface {
	Resolve(ctx context.Context, tenantID, projectID string) (name, repoURL string, err error)
}

// VelocityResolver returns the last n Done tasks in a project (title +
// actual hours), used to give the AI a sense of this team's real pace.
type VelocityResolver interface {
	RecentCompletedTasks(ctx context.Context, tenantID, projectID string, n int) ([]domain.Task, error)
}
```

Rewrite `backend-go/services/task-service/internal/usecase/ai_decompose.go`:

```go
package usecase

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type AIDecomposeInput struct {
	TaskID string
}

type AIDecomposeResult struct {
	Proposals   []domain.SubtaskProposal
	RawResponse string
}

// AIDecompose relays a task to the Dev Server Agent's ai.complete method
// (via infra-fleet-service's Relay RPC) to propose a structured subtask
// breakdown — review-before-commit: the result is not written to
// task_edges until a subsequent AIApply call.
type AIDecompose struct {
	tasks      TaskRepository
	edges      EdgeRepository
	aiProvider AIProviderContextResolver
	resolver   ProjectExecutionResolver
	projectCtx ProjectContextResolver
	techStack  TechStackDetector
	velocity   VelocityResolver
	relay      AICompleter
}

func NewAIDecompose(
	tasks TaskRepository, edges EdgeRepository, aiProvider AIProviderContextResolver,
	resolver ProjectExecutionResolver, projectCtx ProjectContextResolver,
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
		return AIDecomposeResult{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	providerCtx, err := uc.aiProvider.ResolveContext(ctx, tenantID, userID)
	if err != nil {
		return AIDecomposeResult{}, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_PROVIDER_RESOLVE_FAILED", "failed to resolve AI provider context", err)
	}
	connectionID, _, connected, err := uc.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil || !connected {
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

	prompt := buildDecomposePrompt(task, providerCtx, projectName, repoURL, stack, existingTitles, velocityTasks)
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

func buildDecomposePrompt(task domain.Task, providerCtx, projectName, repoURL string, stack domain.TechStack, existingTitles []string, velocity []domain.Task) string {
	var b strings.Builder
	b.WriteString("Break the following task down into subtasks. Respond with JSON matching:\n")
	b.WriteString(`{"subtasks":[{"title","description","type","estimated_hours","depends_on":[<indices>],"prompt_template"}],"notes":"<string>"}` + "\n\n")
	b.WriteString("Task: " + task.Title + "\n")
	if task.Description != "" {
		b.WriteString("Description: " + task.Description + "\n")
	}
	if task.AIContext != "" {
		b.WriteString("AI Context: " + task.AIContext + "\n")
	}
	if projectName != "" {
		b.WriteString("Project: " + projectName + " (" + repoURL + ")\n")
	}
	if len(stack.Languages) > 0 {
		b.WriteString("Tech stack: " + strings.Join(stack.Languages, ", ") + "; " + strings.Join(stack.Frameworks, ", ") + "\n")
	}
	if len(existingTitles) > 0 {
		b.WriteString("Existing subtasks (do not duplicate): " + strings.Join(existingTitles, "; ") + "\n")
	}
	if len(velocity) > 0 {
		var items []string
		for _, v := range velocity {
			items = append(items, v.Title)
		}
		b.WriteString("Recent team velocity: " + strings.Join(items, "; ") + "\n")
	}
	if providerCtx != "" {
		b.WriteString("Provider context: " + providerCtx + "\n")
	}
	return b.String()
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
```

Delete `parseSubtaskProposals`/`parseOrdinal`/`errNotOrdinal` (the
numbered-list parser this replaces) from `ai_decompose.go`.

Update `backend-go/services/task-service/internal/adapter/grpc/server.go`'s
`AIDecompose` handler and `toProtoSubtaskProposals`/`toDomainSubtaskProposals`
to carry the new `AIDecomposeResult{Proposals, RawResponse}` shape and every
new `SubtaskProposal` field (`type`, `estimated_hours`, `depends_on_indices`,
`prompt_template`) — this is finished together with `TASK-TG-02-06`'s grpc
wiring pass; stub the handler to compile here if `TASK-TG-02-06` hasn't
landed yet.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run TestAIDecompose -v
```

Expected: `ai_decompose_test.go` (fake `ProjectContextResolver`/
`TechStackDetector`/`VelocityResolver`) — prompt-building test asserts every
context source's value appears in the built prompt string;
`TechStackDetector` returning an error doesn't fail `Execute` (best-effort
assertion); malformed JSON from `AICompleter` surfaces
`TASK_AI_DECOMPOSE_INVALID_JSON`, not a generic error.
