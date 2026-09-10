package infrafleetclient

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// shellExecMethod is the Relay method name ShellExecutor uses — the Relay
// proto's own doc comment names "shell.exec" as the example method for
// shell steps. Best-effort, not verified against a live Dev Server Agent —
// see AgentExecutor's agentExecMethod doc comment for the sibling caveat on
// this same relay contract.
const shellExecMethod = "shell.exec"

// shellExecParams is the params_json payload ShellExecutor sends — mirrors
// TS's StepExecutors.ts executeShell() shape (script + env).
type shellExecParams struct {
	Script string            `json:"script"`
	Env    map[string]string `json:"env,omitempty"`
}

// ShellExecutor is the real Shell step executor — relays a script to
// infra-fleet-service's Relay RPC for execution on the target connection.
// TASK-WF-002-03: cfg.ConnectionID is now a domain.TargetSpec string,
// resolved to a real connection id via serverResolver before every relay
// call — see AgentExecutor's doc comment for the same widening.
type ShellExecutor struct {
	client         infrafleetv1.InfraFleetServiceClient
	serverResolver *usecase.ServerResolver
}

// NewShellExecutor wraps an already-constructed infrafleetv1 client — used
// by cmd/server/main.go (real dial) and by tests (fake client).
func NewShellExecutor(client infrafleetv1.InfraFleetServiceClient, serverResolver *usecase.ServerResolver) *ShellExecutor {
	return &ShellExecutor{client: client, serverResolver: serverResolver}
}

var _ domain.StepExecutor = (*ShellExecutor)(nil)

func (e *ShellExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg domain.ShellStepConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: shell: invalid step config JSON: %w", err)
	}

	spec, err := domain.ParseTargetSpec(cfg.ConnectionID)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: shell: parsing target spec: %w", err)
	}
	connectionID, err := e.serverResolver.Resolve(ctx, spec)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: shell: resolving target: %w", err)
	}

	var result execResult
	if err := relay(ctx, e.client, connectionID, shellExecMethod, shellExecParams{
		Script: cfg.Script,
		Env:    cfg.Env,
	}, &result); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: shell: %w", err)
	}

	return toStepResult(result)
}
