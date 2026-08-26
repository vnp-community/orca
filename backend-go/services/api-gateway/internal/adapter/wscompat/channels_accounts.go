// Package wscompat — accounts.* channels.
//
// accounts.selectClaude/selectCodex/removeClaude/removeCodex relay through
// infra-fleet-service's existing generic Relay RPC — see SOL-004
// (specs/backend-go/bugs/missing-v1/solutions/SOL-004-accounts-channels.md)
// for why this is not a new service or new backend-side storage: reading/
// writing the Claude/Codex CLI's login config is filesystem-shaped work on
// the target dev server host, the same class of thing devServer.*/fleet.*
// already relay for.
//
// INERT UNTIL AGENT-SIDE WORK LANDS: the Dev Server Agent method names
// below (accounts.selectClaude, etc.) do not exist on the agent's JSON-RPC
// dispatcher yet — see TASK-023 (specs/backend-go/bugs/missing-v1/tasks/
// TASK-023-document-accounts-agent-gap.md). This file's plumbing is
// correct and buildable on its own merits; every call will fail with a
// "method not found" error from the agent until that companion work ships.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// registerAccountsChannels wires accounts.* to infra-fleet-service's
// existing generic Relay RPC. See this file's package doc comment (SOL-004)
// for why no new proto/usecase code is needed on infra-fleet-service's side.
func registerAccountsChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	registerAccountsRelay(r, client, "accounts.selectClaude", "accounts.selectClaude")
	registerAccountsRelay(r, client, "accounts.selectCodex", "accounts.selectCodex")
	registerAccountsRelay(r, client, "accounts.removeClaude", "accounts.removeClaude")
	registerAccountsRelay(r, client, "accounts.removeCodex", "accounts.removeCodex")
}

// accountsRelayArgs is shared by all 4 channels — accountId plus the
// connectionId prerequisite (see this file's package doc comment and
// TASK-023's "Open prerequisite" note).
type accountsRelayArgs struct {
	AccountID    string `json:"accountId"`
	ConnectionID string `json:"connectionId"`
}

// registerAccountsRelay is the single representative implementation shared
// by all 4 channels (select/remove x Claude/Codex are identical in shape —
// only the channel name and the relayed agent method name differ).
func registerAccountsRelay(r *Registry, client infrafleetv1.InfraFleetServiceClient, channel, agentMethod string) {
	r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[accountsRelayArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.ConnectionID == "" {
			// See TASK-023 — accounts.* has no connectionId in today's
			// documented frontend params; fail loudly rather than guessing
			// (e.g. "the tenant's only connection" would silently break
			// multi-environment tenants).
			return nil, fmt.Errorf("ACCOUNTS_NO_CONNECTION: connectionId is required until the frontend contract adds it")
		}
		paramsJSON, err := json.Marshal(map[string]any{"accountId": in.AccountID})
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.Relay(ctx, &infrafleetv1.RelayRequest{
			ConnectionId: in.ConnectionID,
			Method:       agentMethod,
			ParamsJson:   string(paramsJSON),
		})
		if err != nil {
			return nil, err
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(resp.GetResultJson()), &result); err != nil {
			return nil, err
		}
		return result, nil
	})
}
