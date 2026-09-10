package infrafleetclient

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// agentExecMethod is the Relay method name AgentExecutor uses.
//
// The Relay proto's own doc comment names "agent.exec" as the example
// method for agent steps, but that name turned out to be a real, different
// RPC: a generic {binary,args,cwd,stdin,env,timeoutMs} process-exec call
// with no prompt/model/trustPreset concept. TS's
// StepExecutors.executeAgent() (backend/src/main/workflow/StepExecutors.ts)
// hit this same mismatch and switched to "agent.execPrompt" to carry those
// fields (see specs/agent/api/gaps-and-findings.md, "TS Gap 4"). Fixed here
// to match — do not revert to "agent.exec".
const agentExecMethod = "agent.execPrompt"

// agentExecParams is the params_json payload AgentExecutor sends to
// infra-fleet-service's Relay RPC for the "agent.execPrompt" method.
//
// Model/AccountID/StepID/Env are carried in the struct so BE-SOL-002
// (server/provider resolution) doesn't need another struct-shape change;
// Execute leaves them empty for now — an absent model/accountId is a
// documented, safe default in agent.execPrompt's real handler (defaults to
// claude / relies on the CLI's own authenticated state).
type agentExecParams struct {
	Prompt       string            `json:"prompt"`
	WorktreePath string            `json:"worktreePath,omitempty"`
	TrustPreset  string            `json:"trustPreset,omitempty"`
	Model        string            `json:"model,omitempty"`
	AccountID    string            `json:"accountId,omitempty"`
	StepID       string            `json:"stepId,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
}

// AgentExecutor is the real Agent step executor — relays a prompt-driven
// agent invocation to infra-fleet-service's Relay RPC via "agent.execPrompt".
// TASK-WF-002-03: cfg.ConnectionID is now a domain.TargetSpec string
// ("project:<id>" / "server:<id>" / "fleet:tag:<tag>"), resolved to a real
// connection id via serverResolver before every relay call — no longer a
// pre-resolved literal ID. providerResolver implements the explicit-pin vs.
// ai-provider-service priority-chain resolution for Model/AccountID.
type AgentExecutor struct {
	client           infrafleetv1.InfraFleetServiceClient
	serverResolver   *usecase.ServerResolver
	providerResolver *usecase.ProviderResolver
}

// NewAgentExecutor wraps an already-constructed infrafleetv1 client — used
// by cmd/server/main.go (real dial) and by tests (fake client).
func NewAgentExecutor(client infrafleetv1.InfraFleetServiceClient, serverResolver *usecase.ServerResolver, providerResolver *usecase.ProviderResolver) *AgentExecutor {
	return &AgentExecutor{client: client, serverResolver: serverResolver, providerResolver: providerResolver}
}

var _ domain.StepExecutor = (*AgentExecutor)(nil)

func (e *AgentExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg domain.AgentStepConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: invalid step config JSON: %w", err)
	}

	spec, err := domain.ParseTargetSpec(cfg.ConnectionID) // field name unchanged; semantics widen from "literal ID" to "target spec" — see step.go's doc comment
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: parsing target spec: %w", err)
	}
	connectionID, err := e.serverResolver.Resolve(ctx, spec)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: resolving target: %w", err)
	}

	// execCtx: see usecase.ExecutionContext's doc comment for why this is
	// read from ctx rather than an added Execute parameter.
	execCtx := usecase.ExecutionContextFrom(ctx)
	provider, err := e.providerResolver.Resolve(ctx, cfg, execCtx.ProjectID, execCtx.TriggeredBy)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: resolving provider: %w", err)
	}

	var result execResult
	if err := relay(ctx, e.client, connectionID, agentExecMethod, agentExecParams{
		Prompt:       cfg.Prompt,
		WorktreePath: cfg.WorktreePath,
		TrustPreset:  cfg.TrustPreset,
		Model:        provider.Model,
		AccountID:    provider.AccountID,
	}, &result); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: %w", err)
	}

	return toStepResult(result)
}
