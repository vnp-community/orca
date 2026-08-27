package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// errNotOrdinal is parseOrdinal's own sentinel — deliberately distinct from
// fakes_test.go's errNotFound (a test-only identifier that doesn't exist in
// a non-test build).
var errNotOrdinal = errors.New("usecase: not a numeric ordinal")

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
}

func NewAIDecompose(tasks TaskRepository, aiProvider AIProviderContextResolver, resolver ProjectExecutionResolver, relay AICompleter) *AIDecompose {
	return &AIDecompose{tasks: tasks, aiProvider: aiProvider, resolver: resolver, relay: relay}
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
	prompt := buildDecomposePrompt(task, providerCtx)
	content, err := uc.relay.Complete(ctx, connectionID, prompt)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_FAILED", "failed to generate subtask proposals via AI relay", err)
	}
	return parseSubtaskProposals(content), nil
}

// buildDecomposePrompt assembles the ai.complete prompt from task and
// providerCtx. providerCtx is currently only interpolated for traceability
// (which provider/account resolved) — no prompt-engineering work beyond
// that is in this task's scope.
func buildDecomposePrompt(task domain.Task, providerCtx string) string {
	var b strings.Builder
	b.WriteString("Break the following task down into a numbered list of concrete subtasks. ")
	b.WriteString("Respond with one subtask per line, formatted as \"<n>. <title>\".\n\n")
	b.WriteString("Task: ")
	b.WriteString(task.Title)
	if providerCtx != "" {
		b.WriteString("\nProvider context: ")
		b.WriteString(providerCtx)
	}
	return b.String()
}

// parseSubtaskProposals parses ai.complete's free-text numbered-list
// response ("1. Title\n2. Title") into SubtaskProposals. Best-effort: no
// live agent in this environment to confirm ai.complete's response format
// for this specific prompt shape against, so this parses the plain
// numbered-list format the prompt above explicitly asks for rather than
// assuming a structured (e.g. JSON) response.
func parseSubtaskProposals(content string) []domain.SubtaskProposal {
	lines := strings.Split(content, "\n")
	out := make([]domain.SubtaskProposal, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Strip a leading "<n>." or "<n>)" ordinal marker, if present.
		if idx := strings.IndexAny(line, ".)"); idx > 0 && idx <= 3 {
			if _, err := parseOrdinal(line[:idx]); err == nil {
				line = strings.TrimSpace(line[idx+1:])
			}
		}
		if line == "" {
			continue
		}
		out = append(out, domain.SubtaskProposal{Title: line})
	}
	return out
}

func parseOrdinal(s string) (int, error) {
	n := 0
	if s == "" {
		return 0, errNotOrdinal
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errNotOrdinal
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
