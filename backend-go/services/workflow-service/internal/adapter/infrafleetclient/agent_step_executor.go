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

// agentExecPromptMethod replaces the prior, wrong "agent.exec" — see this
// file's Context. Regression-tested directly: agent_step_executor_test.go
// asserts the relay call's method string unconditionally.
const agentExecPromptMethod = "agent.execPrompt"

// agentExecPromptParams is the params_json payload AgentExecutor sends —
// see agentExecPromptMethod's doc comment for the shape caveat. AccountID
// (TASK-WF-02-05) is passthrough: this executor resolves WHICH account to
// use (via ProviderResolver) but never touches credential material itself —
// that stays entirely on infra-fleet-service's/the Dev Server Agent's side
// of the Relay hop.
type agentExecPromptParams struct {
	Prompt       string            `json:"prompt"`
	WorktreePath string            `json:"worktreePath"`
	TrustPreset  string            `json:"trustPreset,omitempty"`
	AccountID    string            `json:"accountId,omitempty"`
	Model        string            `json:"model,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	InitFile     string            `json:"initFile,omitempty"` // project-context preamble — field name UNVERIFIED against the real agent handler, see this task's Verify section
}

// AgentExecutor is the real Agent step executor — relays a prompt-driven
// agent invocation to infra-fleet-service's Relay RPC, method
// "agent.execPrompt". Profile-aware: when the step config carries a
// UserID, resolves the caller's ResolvedProfile and injects it into the
// spawned process's env — see domain/agent_environment.go's BuildAgentEnv.
type AgentExecutor struct {
	client   infrafleetv1.InfraFleetServiceClient
	resolver usecase.ServerResolver
	provider usecase.ProviderResolver
	profiles usecase.ProfileResolver
	projects usecase.ProjectContextResolver
}

// NewAgentExecutor wraps an already-constructed infrafleetv1 client, a
// ServerResolver (see domain.AgentStepConfig.Target's doc comment), a
// ProviderResolver (see domain.AgentStepConfig.Provider's doc comment), a
// ProfileResolver, and a ProjectContextResolver (both TASK-PRF-04-05/06's
// profile-aware env injection) — used by cmd/server/main.go (real dials +
// internal/adapter/serverresolver + internal/adapter/providerresolver +
// internal/adapter/infrafleetclient's own profile/project resolvers) and by
// tests (fakes).
func NewAgentExecutor(client infrafleetv1.InfraFleetServiceClient, resolver usecase.ServerResolver, provider usecase.ProviderResolver, profiles usecase.ProfileResolver, projects usecase.ProjectContextResolver) *AgentExecutor {
	return &AgentExecutor{client: client, resolver: resolver, provider: provider, profiles: profiles, projects: projects}
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
	// Target resolution (TASK-WF-02-04): turn the step's Target/ConnectionID
	// into a concrete connectionId before anything else — EffectiveTarget's
	// "connection:<id>"/legacy-ConnectionID/empty shapes all flow through
	// the same resolver, so this subsumes the old direct
	// relay(..., cfg.ConnectionID, ...) passthrough.
	connectionID, err := e.resolver.Resolve(ctx, tenantID, cfg.EffectiveTarget())
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: resolve target: %w", err)
	}

	// Provider resolution (TASK-WF-02-05): which ai-provider-service account
	// this step uses — userID/projectID come from ctx
	// (tenant.WithUserID/WithProjectID) — best-effort empty until
	// TASK-WF-02-06's waveDispatcher enriches the dispatch ctx with
	// ExecutionContext.UserID/ProjectID; ExecuteAdHocStep's synchronous path
	// (which forwards the caller's own ctx unchanged) already has UserID
	// today.
	ctxUserID, _ := tenant.UserID(ctx)
	ctxProjectID, _ := tenant.ProjectID(ctx)
	accountID, err := e.provider.Resolve(ctx, tenantID, ctxUserID, ctxProjectID, cfg.Provider)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: resolve provider: %w", err)
	}

	params := agentExecPromptParams{Prompt: cfg.Prompt, WorktreePath: cfg.WorktreePath, TrustPreset: cfg.TrustPreset, AccountID: accountID, Model: cfg.Model}

	// Profile-aware path only when UserID is present — a step authored
	// before this migration (no UserID in its config JSON) degrades to a
	// bare passthrough rather than failing outright. An expand/contract
	// compatibility shim, not a permanent branch.
	if cfg.UserID != "" {
		// The outbound resolvers (ProfileResolver/ProjectContextResolver)
		// forward tenant.UserID(ctx) as the acting-user identity — stamp it
		// to cfg.UserID for this step's calls specifically, since ctx's
		// ambient identity (if any) belongs to whatever dispatched the
		// workflow execution, not necessarily this step's target user.
		userCtx := tenant.WithUserID(ctx, cfg.UserID)

		resolved, err := e.profiles.GetResolvedProfile(userCtx, cfg.UserID)
		if err != nil {
			return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: resolve profile: %w", err)
		}
		// agent.execPrompt's model field wants the raw model NAME, not a
		// binary path — the agent-side handler resolves its own binary from
		// the name. domain.ResolveAgentBinary/AgentBinaryMap are NOT called
		// here; they exist only for a future richer spawn RPC (see
		// agent_environment.go's package doc comment). A resolved
		// preferredModel overrides cfg.Model, the same precedence the
		// pre-existing profile-aware path used.
		if agent, ok := resolved["agent"].(map[string]any); ok {
			if model, ok := agent["preferredModel"].(string); ok {
				params.Model = model
			}
		}

		existingPath := "" // this service has no visibility into the target host's existing PATH — env.PATH is additive-only from pathAdditions
		env := domain.BuildAgentEnv(resolved, cfg.UserID, cfg.ProjectID, "", existingPath)

		if cfg.ProjectID != "" {
			pctx, err := e.projects.GetProjectContext(userCtx, cfg.ProjectID)
			if err == nil { // best-effort — a preamble-build failure must never block the agent spawn itself
				env["ORCA_PROJECT_NAME"] = pctx.ProjectName
				params.InitFile = domain.BuildProjectContext(domain.PreambleInput{
					ProjectName: pctx.ProjectName, Description: pctx.Description, RepoURL: pctx.RepoURL,
					WorktreePath: cfg.WorktreePath, DevServerHostname: pctx.DevServerHostname,
				})
			}
		}
		params.Env = env
	}

	var result execResult
	if err := relay(ctx, e.client, connectionID, agentExecPromptMethod, params, &result); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: %w", err)
	}
	return toStepResult(result)
}
