package infrafleetclient

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// agentExecMethod is the Relay method name AgentExecutor uses.
//
// Best-effort, not verified against a live Dev Server Agent — same honest
// gap infra-fleet-service's decodeOpenPorts and git-gateway-service's
// RelayExecutor document. The Relay proto's own doc comment names
// "agent.exec" as the example method for agent steps, and this executor
// follows that/this build's explicit instruction. But TS's
// StepExecutors.executeAgent() (backend/src/main/workflow/StepExecutors.ts)
// found "agent.exec" IS a real agent RPC — just a different one: a generic
// {binary,args,cwd,stdin,env,timeoutMs} process-exec call with no
// prompt/model/trustPreset concept — and had to switch to "agent.execPrompt"
// to carry those fields (see specs/agent/api/gaps-and-findings.md, "TS Gap
// 4"). Reconcile the method name against the real agent handler contract
// before depending on this in production.
const agentExecMethod = "agent.exec"

// agentExecParams is the params_json payload AgentExecutor sends — see
// agentExecMethod's doc comment for the shape caveat.
type agentExecParams struct {
	Prompt       string `json:"prompt"`
	WorktreePath string `json:"worktreePath,omitempty"`
	TrustPreset  string `json:"trustPreset,omitempty"`
}

// AgentExecutor is the real Agent step executor — relays a prompt-driven
// agent invocation to infra-fleet-service's Relay RPC. See agentExecMethod's
// doc comment for the method-name/param-shape caveat.
type AgentExecutor struct {
	client   infrafleetv1.InfraFleetServiceClient
	resolver usecase.ServerResolver
}

// NewAgentExecutor wraps an already-constructed infrafleetv1 client and a
// ServerResolver (see domain.AgentStepConfig.Target's doc comment) — used
// by cmd/server/main.go (real dial + internal/adapter/serverresolver) and
// by tests (fakes).
func NewAgentExecutor(client infrafleetv1.InfraFleetServiceClient, resolver usecase.ServerResolver) *AgentExecutor {
	return &AgentExecutor{client: client, resolver: resolver}
}

var _ domain.StepExecutor = (*AgentExecutor)(nil)

func (e *AgentExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg domain.AgentStepConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: invalid step config JSON: %w", err)
	}

	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: %w", err)
	}
	connectionID, err := e.resolver.Resolve(ctx, tenantID, cfg.EffectiveTarget())
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: resolve target: %w", err)
	}

	var result execResult
	if err := relay(ctx, e.client, connectionID, agentExecMethod, agentExecParams{
		Prompt:       cfg.Prompt,
		WorktreePath: cfg.WorktreePath,
		TrustPreset:  cfg.TrustPreset,
	}, &result); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: %w", err)
	}

	return toStepResult(result)
}
