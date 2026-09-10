// Package infrafleetclient implements orchestration-service's outbound
// WorkerDispatcher port against infra-fleet-service — the only other
// service this one is allowed to dial directly for execution dispatch
// (orchestration-service.md §7).
package infrafleetclient

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// WorkerDispatcher implements usecase.WorkerDispatcher for real — relays a
// dispatch to a terminal-hosted AI-agent worker via infra-fleet-service's
// Relay RPC, following task-service's SimpleExecutor.Execute
// (simple_executor.go:132-193) pattern: this is a call-site reuse of an
// already-proven relay shape, not a new resolution mechanism.
type WorkerDispatcher struct {
	relay infrafleetv1.InfraFleetServiceClient
}

func NewWorkerDispatcher(relay infrafleetv1.InfraFleetServiceClient) *WorkerDispatcher {
	return &WorkerDispatcher{relay: relay}
}

// taskConnectionSpec is the subset of OrchestrationTask.Spec this adapter
// reads — connection targeting is task-service's ProjectExecutionResolver's
// job upstream, already baked into the spec at buildOrchestrationSpec time
// (SOL-TG-04) per orchestration-service.md §7's "does not decide
// decomposition strategy" — this adapter never re-resolves a connection on
// its own.
type taskConnectionSpec struct {
	ConnectionID string `json:"connectionId"`
	WorktreePath string `json:"worktreePath"`
}

// agentExecPromptParams/Result mirror SimpleExecutor's own types
// (simple_executor.go) — same wire shape, same convention, deliberately
// not shared as an exported type across services (each service's client
// package owns its own copy per this codebase's existing precedent of not
// sharing wire-shape structs cross-service outside the generated proto).
type agentExecPromptParams struct {
	Prompt       string `json:"prompt"`
	WorktreePath string `json:"worktreePath"`
	StepID       string `json:"stepId,omitempty"`
}

type agentExecPromptResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode *int   `json:"exitCode"`
	TimedOut bool   `json:"timedOut"`
}

func (d *WorkerDispatcher) Dispatch(ctx context.Context, tenantID string, task domain.OrchestrationTask, handle string) error {
	var conn taskConnectionSpec
	if err := json.Unmarshal(task.Spec, &conn); err != nil {
		return fmt.Errorf("worker_dispatcher: unmarshal task spec: %w", err)
	}
	if conn.ConnectionID == "" {
		return fmt.Errorf("worker_dispatcher: task %q spec has no connectionId", task.ID)
	}

	paramsJSON, err := json.Marshal(agentExecPromptParams{
		Prompt:       task.TaskTitle,
		WorktreePath: conn.WorktreePath,
		StepID:       handle,
	})
	if err != nil {
		return fmt.Errorf("worker_dispatcher: marshal params: %w", err)
	}
	resp, err := d.relay.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: conn.ConnectionID, Method: "agent.execPrompt", ParamsJson: string(paramsJSON),
	})
	if err != nil {
		return fmt.Errorf("worker_dispatcher: relay agent.execPrompt: %w", err)
	}
	var result agentExecPromptResult
	if err := json.Unmarshal([]byte(resp.GetResultJson()), &result); err != nil {
		return fmt.Errorf("worker_dispatcher: unmarshal agent.execPrompt result: %w", err)
	}
	if result.TimedOut {
		return fmt.Errorf("worker_dispatcher: agent.execPrompt timed out for task %q", task.ID)
	}
	if result.ExitCode == nil || *result.ExitCode != 0 {
		return fmt.Errorf("worker_dispatcher: agent.execPrompt exited non-zero for task %q: %s", task.ID, result.Stderr)
	}
	return nil
}
