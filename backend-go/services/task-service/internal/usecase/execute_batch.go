package usecase

import (
	"context"
	"sync"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type ExecuteBatchInput struct {
	TaskIDs        []string
	MaxConcurrency int
	StopOnFailure  bool
	RequestID      string
}

type ExecuteBatchResult struct {
	Completed []string
	Failed    map[string]error
}

// ExecuteBatch dispatches a set of tasks in dependency-respecting waves
// (domain.TopologicalWaves, over depends_on edges scoped to in.TaskIDs) —
// siblings within a wave run with bounded concurrency (in.MaxConcurrency);
// a wave only starts once every task in the previous wave has finished
// (successfully or not — a failed dependency's dependents still get a
// chance to run unless StopOnFailure). Reuses ExecuteTask.Execute per task
// rather than a second dispatch implementation — every one of
// TASK-TG-04-01's permission/worktree/completion behaviors apply
// per-task exactly as they do for a standalone Execute call.
type ExecuteBatch struct {
	edges       EdgeRepository
	executeTask *ExecuteTask
}

func NewExecuteBatch(edges EdgeRepository, executeTask *ExecuteTask) *ExecuteBatch {
	return &ExecuteBatch{edges: edges, executeTask: executeTask}
}

func (uc *ExecuteBatch) Execute(ctx context.Context, in ExecuteBatchInput) (ExecuteBatchResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ExecuteBatchResult{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	allDeps, err := uc.edges.ListByKind(ctx, tenantID, domain.EdgeKindDependsOn)
	if err != nil {
		return ExecuteBatchResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_BATCH_EDGE_LOOKUP_FAILED", "failed to list dependency edges", err)
	}
	waves := domain.TopologicalWaves(allDeps, in.TaskIDs)

	maxConcurrency := in.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 4 // config value in production wiring, not hardcoded — see main.go
	}

	result := ExecuteBatchResult{Failed: map[string]error{}}
	for _, wave := range waves {
		sem := make(chan struct{}, maxConcurrency)
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, taskID := range wave {
			wg.Add(1)
			sem <- struct{}{}
			go func(taskID string) {
				defer wg.Done()
				defer func() { <-sem }()
				_, err := uc.executeTask.Execute(ctx, ExecuteTaskInput{TaskID: taskID, RequestID: in.RequestID})
				mu.Lock()
				if err != nil {
					result.Failed[taskID] = err
				} else {
					result.Completed = append(result.Completed, taskID)
				}
				mu.Unlock()
			}(taskID)
		}
		wg.Wait()
		if in.StopOnFailure && len(result.Failed) > 0 {
			return result, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_BATCH_STOPPED", "batch execution stopped after a wave failure", nil)
		}
	}
	return result, nil
}
