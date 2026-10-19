package grpcclient

import (
	"context"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/common/agentrelay"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// maxStdoutBytes caps what is read into memory; the agent is asked for slightly more (agentrelay.AnalysisMaxOutputBytes)
// so a JSON block at the end survives.
const maxStdoutBytes = 256 * 1024

// AgentPromptRelay calls agent.execPrompt through infra-fleet's Relay (or RelayByDevServer when the project has no
// infra connection), so local, SSH and WSL dev servers behave the same.
type AgentPromptRelay struct {
	relay *agentrelay.Relayer
}

var _ usecase.AgentPromptRunner = (*AgentPromptRelay)(nil)

func NewAgentPromptRelay(infra infrafleetv1.InfraFleetServiceClient) *AgentPromptRelay {
	return &AgentPromptRelay{relay: newTenantRelayer(infra)}
}

// execParamsFor fixes trust and env: the env holds two ids and never a token or credential, and no branch yields the
// full trust preset (task-service's executor sends "full"; analysis must not).
func execParamsFor(in usecase.AgentPromptInput) (agentrelay.ExecParams, error) {
	run := agentrelay.Run{
		StepID: in.StepID, Prompt: in.Prompt, WorktreePath: in.RepoPath, TimeoutMS: in.TimeoutMS,
		Env:           map[string]string{"ORCA_REQUEST_ID": in.RequestID, "ORCA_PROJECT_ID": in.ProjectID},
		KnowledgeRoot: in.KnowledgeRoot, ProposeChangesets: in.ProposeChangesets, ResultNonce: in.ResultNonce,
	}
	switch {
	case in.ReadOnlyEnforced && in.WriteProfile != "":
		return agentrelay.ExecParams{}, agentrelay.ErrConflictingAccess
	case in.ReadOnlyEnforced:
		return agentrelay.ReadOnly(run), nil
	case in.WriteProfile != "":
		return agentrelay.WriteProfile(run, in.WriteProfile)
	}
	// Not Contract: that adds reportChanges and a 25 minute timeout request-service never sent.
	run.TrustPreset = agentrelay.TrustDefault
	return agentrelay.Plain(run), nil
}

func (r *AgentPromptRelay) ExecPrompt(ctx context.Context, conn usecase.AnalysisConnection, in usecase.AgentPromptInput) (usecase.AgentPromptResult, error) {
	params, err := execParamsFor(in)
	if err != nil {
		return usecase.AgentPromptResult{}, err
	}
	res, err := r.relay.ExecPrompt(ctx, agentrelay.Target{DevServerID: conn.DevServerID, ConnectionID: conn.ConnectionID}, params)
	if err != nil {
		return usecase.AgentPromptResult{}, mapAgentRelayError(err)
	}
	return agentPromptResultFrom(res), nil
}

// mapAgentRelayError keeps the usecase's own sentinels for the refusals it reacts to; everything else passes through.
func mapAgentRelayError(err error) error {
	switch {
	case errors.Is(err, agentrelay.ErrTimeout):
		return context.DeadlineExceeded
	case errors.Is(err, agentrelay.ErrReadonlyUnsupported):
		return usecase.ErrAgentReadonlyUnsupported
	case errors.Is(err, agentrelay.ErrKnowledgeHubUnavailable):
		return fmt.Errorf("%w: %w", usecase.ErrAgentKnowledgeHubUnavailable, err)
	}
	return err
}

// newTenantRelayer is the one place this service builds its relayer: every agent RPC carries the validated tenant.
func newTenantRelayer(infra infrafleetv1.InfraFleetServiceClient) *agentrelay.Relayer {
	return agentrelay.New(infra, agentrelay.WithMetadata(withTenantMetadata))
}
