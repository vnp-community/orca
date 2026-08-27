# TASK-TG-02-06: `GenerateAgentPrompt` usecase + gRPC wiring for the widened AI RPCs

**From Solution:** SOL-TG-02
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/generate_agent_prompt.go` (new), `backend-go/services/task-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-TG-02-01, TASK-TG-02-04, TASK-TG-02-05
**Status:** `[ ]` TODO

---

## Context

`GenerateAgentPrompt` is the second AI flow — a smaller sibling of
`AIDecompose` that asks the AI to write one ready-to-use agent prompt for a
task, optionally saving it to `Task.PromptTemplate` immediately
(`Save=true`) or leaving it for the caller to preview/edit before a
subsequent `UpdateTask` call. This task also finishes wiring
`AIDecompose`/`AIApply`'s grpc handlers for their widened request/response
shapes (deferred from `TASK-TG-02-04`/`TASK-TG-02-05` since both needed the
usecase changes to land first).

## Changes to make

Create `backend-go/services/task-service/internal/usecase/generate_agent_prompt.go`:

```go
package usecase

import (
	"context"
	"strings"

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

	task, err := uc.tasks.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return "", apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	providerCtx, err := uc.aiProvider.ResolveContext(ctx, tenantID, userID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_GENERATE_PROMPT_PROVIDER_RESOLVE_FAILED", "failed to resolve AI provider context", err)
	}
	connectionID, _, connected, err := uc.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil || !connected {
		return "", apperrors.New(apperrors.KindFailedPrecondition, "TASK_GENERATE_PROMPT_NO_CONNECTION", "task's project has no connected dev server for AI relay", err)
	}

	prompt := buildPromptGenerationPrompt(task, providerCtx)
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

func buildPromptGenerationPrompt(task interface {
	GetTitle() string
}, providerCtx string) string {
	var b strings.Builder
	b.WriteString("Write one ready-to-use agent prompt for completing this task, given its description and context.\n\n")
	b.WriteString("Task: " + task.GetTitle() + "\n")
	if providerCtx != "" {
		b.WriteString("Provider context: " + providerCtx + "\n")
	}
	return b.String()
}
```

(The `interface{ GetTitle() string }` parameter above is illustrative —
replace with `task domain.Task` and reference `task.Title`/`task.Description`/
`task.AIContext` directly, matching `buildDecomposePrompt`'s plain-text
convention exactly.)

Add the usecase to `backend-go/services/task-service/cmd/server/main.go`'s
composition root and to `taskgrpc.New`'s parameter list.

In `backend-go/services/task-service/internal/adapter/grpc/server.go`, add:

```go
func (s *Server) GenerateAgentPrompt(ctx context.Context, req *taskv1.GenerateAgentPromptRequest) (*taskv1.GenerateAgentPromptResponse, error) {
	prompt, err := s.generateAgentPrompt.Execute(ctx, usecase.GenerateAgentPromptInput{TaskID: req.GetTaskId(), Save: req.GetSave()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GenerateAgentPromptResponse{Prompt: prompt}, nil
}
```

Update `AIDecompose`/`AIApply` handlers for the widened shapes:

```go
func (s *Server) AIDecompose(ctx context.Context, req *taskv1.AIDecomposeRequest) (*taskv1.AIDecomposeResponse, error) {
	result, err := s.aiDecompose.Execute(ctx, usecase.AIDecomposeInput{TaskID: req.GetTaskId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.AIDecomposeResponse{Proposals: toProtoSubtaskProposals(result.Proposals), RawResponse: result.RawResponse}, nil
}

func (s *Server) AIApply(ctx context.Context, req *taskv1.AIApplyRequest) (*taskv1.AIApplyResponse, error) {
	created, err := s.aiApply.Execute(ctx, usecase.AIApplyInput{
		TaskID: req.GetTaskId(), Proposals: toDomainSubtaskProposals(req.GetProposals()), RawAIResponse: req.GetRawAiResponse(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(created))
	for _, t := range created {
		out = append(out, toProtoTask(t))
	}
	return &taskv1.AIApplyResponse{CreatedSubtasks: out}, nil
}

func toProtoSubtaskProposals(proposals []domain.SubtaskProposal) []*taskv1.SubtaskProposal {
	out := make([]*taskv1.SubtaskProposal, 0, len(proposals))
	for _, p := range proposals {
		proto := &taskv1.SubtaskProposal{Title: p.Title, Description: p.Description, Type: p.Type, PromptTemplate: p.PromptTemplate}
		if p.EstimatedHours != nil {
			proto.EstimatedHours = wrapperspb.Double(*p.EstimatedHours)
		}
		for _, idx := range p.DependsOnIndices {
			proto.DependsOnIndices = append(proto.DependsOnIndices, int32(idx))
		}
		out = append(out, proto)
	}
	return out
}

func toDomainSubtaskProposals(proposals []*taskv1.SubtaskProposal) []domain.SubtaskProposal {
	out := make([]domain.SubtaskProposal, 0, len(proposals))
	for _, p := range proposals {
		sp := domain.SubtaskProposal{Title: p.GetTitle(), Description: p.GetDescription(), Type: p.GetType(), PromptTemplate: p.GetPromptTemplate()}
		if p.GetEstimatedHours() != nil {
			v := p.GetEstimatedHours().GetValue()
			sp.EstimatedHours = &v
		}
		for _, idx := range p.GetDependsOnIndices() {
			sp.DependsOnIndices = append(sp.DependsOnIndices, int(idx))
		}
		out = append(out, sp)
	}
	return out
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run TestGenerateAgentPrompt -v
go test ./services/task-service/internal/adapter/grpc/... -v
```

Expected: `generate_agent_prompt_test.go` — `Save=false` never calls
`UpdatePromptTemplate`; `Save=true` does, with the generated text. gRPC
server tests cover `GenerateAgentPrompt` and the widened `AIDecompose`/
`AIApply` round-trip (proposal `type`/`estimated_hours`/`depends_on_indices`/
`prompt_template` survive proto <-> domain conversion).
