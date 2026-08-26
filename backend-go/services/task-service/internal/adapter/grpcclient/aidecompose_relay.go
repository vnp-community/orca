package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// AICompleter implements usecase.AICompleter by relaying to the Dev Server
// Agent's ai.complete method via infra-fleet-service's Relay RPC — same
// method name and shape as git-gateway-service's RelayExecutor.Complete
// (relay_executor.go), duplicated per-service rather than shared since
// each service's Relay client is a distinct generated gRPC stub over its
// own dialed connection to infra-fleet-service.
//
// This method's "ai.complete" is confirmed against
// specs/agent/api/agent-rpc-catalog-runtime.md's real ai.complete handler
// contract (ai-complete-handler.ts:47, cited by git-gateway-service's own
// RelayExecutor.Complete doc comment): params `prompt(required), format?,
// taskId?, model?, accountId?, resolvedApiKey?`, result `{content,
// model?}`. Only prompt is sent here, matching git-gateway-service's own
// posture — model/account resolution is out of this usecase's scope.
type AICompleter struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewAICompleter(client infrafleetv1.InfraFleetServiceClient) *AICompleter {
	return &AICompleter{client: client}
}

func (a *AICompleter) Complete(ctx context.Context, connectionID, prompt string) (string, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return "", err
	}
	paramsJSON, err := json.Marshal(map[string]any{"prompt": prompt})
	if err != nil {
		return "", fmt.Errorf("grpcclient: marshal ai.complete params: %w", err)
	}
	resp, err := a.client.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: connectionID, Method: "ai.complete", ParamsJson: string(paramsJSON),
	})
	if err != nil {
		return "", fmt.Errorf("grpcclient: relay ai.complete: %w", err)
	}
	var result struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(resp.GetResultJson()), &result); err != nil {
		return "", fmt.Errorf("grpcclient: unmarshal ai.complete result: %w", err)
	}
	return result.Content, nil
}
