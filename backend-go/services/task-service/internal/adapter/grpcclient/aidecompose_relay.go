package grpcclient

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/agentrelay"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// AICompleter implements usecase.AICompleter by relaying to the Dev Server
// Agent's ai.complete method through infra-fleet-service's Relay RPC, via the
// shared agentrelay package. Only the prompt is sent: model and account
// resolution are out of this usecase's scope, so the agent falls back to its own
// configured default (the same posture as git-gateway-service's Complete).
type AICompleter struct {
	relay *agentrelay.Relayer
}

func NewAICompleter(client infrafleetv1.InfraFleetServiceClient) *AICompleter {
	return &AICompleter{relay: agentrelay.New(client, agentrelay.WithMetadata(withTenantMetadata))}
}

func (a *AICompleter) Complete(ctx context.Context, connectionID, prompt string) (string, error) {
	res, err := a.relay.Complete(ctx, agentrelay.Target{ConnectionID: connectionID}, agentrelay.CompleteParams{Prompt: prompt})
	if err != nil {
		return "", fmt.Errorf("grpcclient: ai.complete: %w", err)
	}
	return res.Content, nil
}
