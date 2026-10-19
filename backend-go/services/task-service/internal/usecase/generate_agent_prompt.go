package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type GenerateAgentPromptInput struct {
	TaskID string
	Save   bool
}

// GenerateAgentPrompt asks the AI to write one ready-to-use agent prompt
// for completing a task — structurally a smaller sibling of AIDecompose.
// Save is caller-controlled: false previews without persisting, true
// writes the generated text to Task.PromptTemplate immediately.
type GenerateAgentPrompt struct {
	tasks      TaskRepository
	aiProvider AIProviderContextResolver
	resolver   ProjectExecutionResolver
	relay      AICompleter
	// permission, when set, requires read access to preview and write access
	// to save. Set in production wiring; left nil only by tests that exercise
	// prompt building on its own.
	permission *ResolvePermission
}

// WithPermissionCheck makes Execute enforce the caller's grant on the task.
func (uc *GenerateAgentPrompt) WithPermissionCheck(p *ResolvePermission) *GenerateAgentPrompt {
	uc.permission = p
	return uc
}

func NewGenerateAgentPrompt(tasks TaskRepository, aiProvider AIProviderContextResolver, resolver ProjectExecutionResolver, relay AICompleter) *GenerateAgentPrompt {
	return &GenerateAgentPrompt{tasks: tasks, aiProvider: aiProvider, resolver: resolver, relay: relay}
}

func (uc *GenerateAgentPrompt) Execute(ctx context.Context, in GenerateAgentPromptInput) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	userID, _ := tenant.UserID(ctx)

	// Before any lookup: a caller without access must not learn whether the
	// task exists, and Save writes PromptTemplate, which the agent later runs.
	if uc.permission != nil {
		action := "read"
		if in.Save {
			action = "write"
		}
		if _, err := uc.permission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: userID, Action: action}); err != nil {
			return "", err
		}
	}

	task, err := uc.tasks.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return "", taskRepoError(err, "task not found", "TASK_GENERATE_PROMPT_TASK_LOAD_FAILED", "failed to load task")
	}
	providerCtx, err := uc.aiProvider.ResolveContext(ctx, tenantID, userID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_GENERATE_PROMPT_PROVIDER_RESOLVE_FAILED", "failed to resolve AI provider context", err)
	}
	connectionID, _, _, _, connected, err := uc.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil || !connected {
		return "", apperrors.New(apperrors.KindFailedPrecondition, "TASK_GENERATE_PROMPT_NO_CONNECTION", "task's project has no connected dev server for AI relay", err)
	}

	prompt, err := renderPromptGenerationPrompt(task, providerCtx)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_GENERATE_PROMPT_BUILD_FAILED", "failed to build the agent prompt request", err)
	}
	generated, err := uc.relay.Complete(ctx, connectionID, prompt)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_GENERATE_PROMPT_FAILED", "failed to generate agent prompt via AI relay", err)
	}
	if in.Save {
		if err := uc.tasks.UpdatePromptTemplate(ctx, tenantID, in.TaskID, generated); err != nil {
			return "", apperrors.New(apperrors.KindInternal, "TASK_GENERATE_PROMPT_SAVE_FAILED", "failed to save generated prompt", err)
		}
	}
	return generated, nil
}
