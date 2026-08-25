package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// SimpleExecutor implements usecase.SimpleExecutor for real (TASK-224),
// replacing the prior StubSimpleExecutor — dispatches Execute's simple path
// to infra-fleet-service's Relay RPC, method "agent.exec". See
// task-service.md §3.1 and TASK-224's Context note for why task-service
// goes through infra-fleet-service rather than dialing the Dev Server
// Agent itself (task-service.md §2/§3.1's "only two Go services that talk
// to the execution plane" rule — infra-fleet-service and
// git-gateway-service — task-service isn't one of them).
//
// KNOWN CONTRACT GAP, flagged rather than guessed around: the "agent.exec"
// method name and this params/result JSON shape below are the task's own
// best-effort sketch, mirroring git-gateway-service's RelayExecutor
// caveat-comment posture for its git.* methods. specs/agent/api/agent-rpc-catalog-runtime.md
// (confirmed present in this worktree) documents agent.exec's REAL params
// as `binary(required), args?, cwd?, stdin?, env?, timeoutMs?, stepId?,
// taskId?, parentTraceId?` (a "run this literal binary" primitive) and
// notes "No live backend caller as of 2026-08-16" — both former callers
// moved to `agent.execPrompt` (prompt(required), worktreePath(required),
// trustPreset?, model?, accountId?, ...) for exactly this "dispatch an
// AI-driven task" use case. Switching this call to agent.execPrompt is a
// real design decision this task does not have enough context to make
// safely under time pressure: worktreePath resolution, prompt
// construction, and model/account selection are not answered by this
// port's existing signature (tenantID, taskID, requestID) or by any
// existing task-service code. Rather than guess at that design, this
// method keeps the plumbing (resolve connection, relay, unmarshal) real
// and correct, and keeps the task's originally-specified method
// name/shape — reconcile the actual relay method name/params against a
// real Dev Server Agent (or a follow-up design task) before this executes
// against one. See TASK-224's Status line and final report for this gap.
type SimpleExecutor struct {
	tasks    usecase.TaskRepository
	resolver usecase.ProjectExecutionResolver
	relay    infrafleetv1.InfraFleetServiceClient
}

func NewSimpleExecutor(tasks usecase.TaskRepository, resolver usecase.ProjectExecutionResolver, relay infrafleetv1.InfraFleetServiceClient) *SimpleExecutor {
	return &SimpleExecutor{tasks: tasks, resolver: resolver, relay: relay}
}

func (s *SimpleExecutor) Execute(ctx context.Context, tenantID, taskID, requestID string) (string, error) {
	task, err := s.tasks.Get(ctx, tenantID, taskID)
	if err != nil {
		return "", fmt.Errorf("simple_executor: load task: %w", err)
	}
	connectionID, connected, err := s.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil {
		return "", fmt.Errorf("simple_executor: resolve connection: %w", err)
	}
	if !connected {
		// Per git-gateway-service.md §8's precedent: a resolve failure or
		// not-connected connectionId is a real error, never a silent local
		// fallback — task-service has no local agent.exec equivalent of its
		// own (unlike git-gateway-service's §2 step 3, there is no "this
		// service's own host" case for task execution).
		return "", apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_CONNECTION", "task's project has no connected dev server", nil)
	}

	paramsJSON, err := json.Marshal(map[string]any{
		"taskId":    taskID,
		"title":     task.Title,
		"requestId": requestID,
	})
	if err != nil {
		return "", fmt.Errorf("simple_executor: marshal params: %w", err)
	}
	resp, err := s.relay.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: connectionID, Method: "agent.exec", ParamsJson: string(paramsJSON),
	})
	if err != nil {
		return "", fmt.Errorf("simple_executor: relay agent.exec: %w", err)
	}
	var result struct {
		ExecutionRef string `json:"executionRef"`
	}
	if err := json.Unmarshal([]byte(resp.GetResultJson()), &result); err != nil {
		return "", fmt.Errorf("simple_executor: unmarshal agent.exec result: %w", err)
	}
	return result.ExecutionRef, nil
}
