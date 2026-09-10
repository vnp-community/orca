// Package wscompat — devServer.agentTokens.* channels.
//
// create/list/revoke wire BL-AWS-03's persistent agent token admin surface
// onto infra-fleet-service's CreateAgentToken/ListAgentTokens/
// RevokeAgentToken RPCs (SOL-AWS-03). Authenticated as a normal per-tenant
// admin action (session/JWT identity), NOT the ORCA_AGENT_API_SECRET gate
// TokenIssuer's bootstrap endpoint uses — see SOL-AWS-03's "reconciling
// with the existing ephemeral Registry/TokenIssuer" section for why
// conflating the two auth models would be a regression.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerDevServerAgentTokenChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("devServer.agentTokens.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			DevServerID string `json:"devServerId"`
			Name        string `json:"name"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.DevServerID == "" {
			return nil, fmt.Errorf("AGENT_TOKENS_NO_DEV_SERVER: devServerId is required")
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreateAgentToken(rpcCtx, &infrafleetv1.CreateAgentTokenRequest{DevServerId: in.DevServerID, Name: in.Name})
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"id": resp.GetId(), "token": resp.GetToken(), "name": resp.GetName(),
			"createdAtUnixMs": resp.GetCreatedAtUnixMs(),
		}, nil
	})

	r.Register("devServer.agentTokens.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			DevServerID string `json:"devServerId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListAgentTokens(rpcCtx, &infrafleetv1.ListAgentTokensRequest{DevServerId: in.DevServerID})
		if err != nil {
			return nil, err
		}
		out := make([]map[string]any, 0, len(resp.GetTokens()))
		for _, t := range resp.GetTokens() {
			entry := map[string]any{"id": t.GetId(), "name": t.GetName(), "createdAtUnixMs": t.GetCreatedAtUnixMs()}
			if t.LastUsedAtUnixMs != nil {
				entry["lastUsedAtUnixMs"] = t.GetLastUsedAtUnixMs()
			}
			out = append(out, entry)
		}
		return map[string]any{"tokens": out}, nil
	})

	r.Register("devServer.agentTokens.revoke", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type revokeArgs struct {
			DevServerID string `json:"devServerId"`
			ID          string `json:"id"`
		}
		in, err := decodeArg[revokeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		_, err = client.RevokeAgentToken(rpcCtx, &infrafleetv1.RevokeAgentTokenRequest{DevServerId: in.DevServerID, Id: in.ID})
		return map[string]bool{"ok": err == nil}, err
	})
}
