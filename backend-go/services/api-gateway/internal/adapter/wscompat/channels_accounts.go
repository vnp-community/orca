// Package wscompat — accounts.* channels.
//
// accounts.selectClaude/selectCodex/removeClaude/removeCodex relay through
// infra-fleet-service's Relay-family RPCs — see SOL-004
// (specs/backend-go/bugs/missing-v1/solutions/SOL-004-accounts-channels.md)
// for why this is not a new service or new backend-side storage: reading/
// writing the Claude/Codex CLI's login config is filesystem-shaped work on
// the target dev server host, the same class of thing devServer.*/fleet.*
// already relay for.
//
// AGENT-SIDE WORK LANDED (TASK-023, specs/backend-go/bugs/missing-v1/tasks/
// TASK-023-document-accounts-agent-gap.md): agent/src/relay/accounts-handler.ts
// implements all 4 relayed methods below for real, plus the read-only
// accounts.getSnapshot registerAccountsSubscribeChannel polls.
//
// Live bug (user-reported): every one of these channels used to route
// through ConnectionResolver.ResolveConnection — "is there an
// infra.connections DB row for this dev server", a DIFFERENT concept from
// "is the agent's session live" (see usecase.RelayByDevServer's doc
// comment on infra-fleet-service, and this same fix applied to
// devServer.browseDir/onboarding.detectAgents earlier). A dev server has
// no connections row until a repo/worktree is bound to it — the AI
// Provider Accounts picker has nothing to do with any bound project, so a
// perfectly live, connected dev server always showed "This dev server is
// not currently connected." Fixed by relaying through RelayByDevServer
// (devServerId-keyed, bypasses infra.connections) instead of Relay
// (connectionId-keyed) throughout this file. The wire field is still named
// `connectionId` on purpose — see accountsRelayArgs's doc comment — no
// frontend change was needed.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

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
	registerAccountsSubscribeChannel(r, client)
	registerAccountsResolveDevServerConnectionChannel(r, client)
}

// accountsResolveDevServerConnectionArgs — TASK-023's frontend dev-server
// picker sends the devServerId the user explicitly chose; this channel
// turns that choice into the identifier the 4 relay channels/
// accounts.subscribe below require. Named specifically for accounts.*'s
// picker rather than a bare generic "resolve any devServerId" channel, per
// this task's naming guidance.
type accountsResolveDevServerConnectionArgs struct {
	DevServerID string `json:"devServerId"`
}

// accountsResolveDevServerConnectionResult's ConnectionID is, despite the
// name, the devServerId echoed back verbatim — see this file's package doc
// comment for why. Kept named "connectionId" on the wire so the existing
// frontend contract (accounts-dev-server-connection.ts, which treats this
// as an opaque token it passes straight into the next accounts.* call)
// needed zero changes.
type accountsResolveDevServerConnectionResult struct {
	Connected    bool   `json:"connected"`
	ConnectionID string `json:"connectionId"`
}

// registerAccountsResolveDevServerConnectionChannel reports whether the
// chosen dev server's agent has a live session right now, via
// IsDevServerConnected (a pure peek at the agent's actual session state —
// see devserveragent.Client.IsConnected's doc comment) — NOT
// ResolveConnection, which answers a different question (does an
// infra.connections DB row exist) that's unrelated to account management.
// Deliberately does NOT fail when Connected is false: "this dev server has
// no live connection right now" is a legitimate, displayable picker state,
// not an error — callers (runtime-provider-accounts-client.ts) check
// `connected` themselves before attempting a relay call.
func registerAccountsResolveDevServerConnectionChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("accounts.resolveDevServerConnection", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[accountsResolveDevServerConnectionArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.DevServerID == "" {
			return nil, fmt.Errorf("ACCOUNTS_NO_DEV_SERVER: devServerId is required")
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.IsDevServerConnected(rpcCtx, &infrafleetv1.IsDevServerConnectedRequest{DevServerId: in.DevServerID})
		if err != nil {
			return nil, err
		}
		return accountsResolveDevServerConnectionResult{
			Connected:    resp.GetConnected(),
			ConnectionID: in.DevServerID,
		}, nil
	})
}

// accountsRelayArgs is shared by all 4 channels — accountId plus the
// devServerId (still named connectionId on the wire, see this file's
// package doc comment) accounts.resolveDevServerConnection resolved.
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
			return nil, fmt.Errorf("ACCOUNTS_NO_CONNECTION: connectionId (devServerId) is required — pick a dev server first")
		}
		paramsJSON, err := json.Marshal(map[string]any{"accountId": in.AccountID})
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.RelayByDevServer(ctx, &infrafleetv1.RelayByDevServerRequest{
			DevServerId: in.ConnectionID,
			Method:      agentMethod,
			ParamsJson:  string(paramsJSON),
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

// accountsSubscribeArgs mirrors accountsRelayArgs minus AccountID —
// accounts.subscribe's real frontend call site
// (frontend/src/renderer/src/runtime/runtime-provider-accounts-client.ts's
// watchProviderAccounts) sends NO params object at all today (`{selector,
// method: 'accounts.subscribe', timeoutMs}`, no third params argument), so
// this channel decodes with decodeOptionalArg (registry.go) rather than
// decodeArg — a genuinely missing args[0] must degrade to the zero value
// (ConnectionID: "") so the explicit ACCOUNTS_NO_CONNECTION check below can
// fire with its specific message, not a generic "missing arg[0]" decode
// error that would obscure the real, already-documented TASK-023 gap.
// ConnectionID is, like accountsRelayArgs.ConnectionID, actually a
// devServerId on the wire — see this file's package doc comment.
type accountsSubscribeArgs struct {
	ConnectionID string `json:"connectionId"`
}

// accountsSubscribePollInterval is a var, not a const, so tests can shrink
// it — see channels_accounts_test.go.
var accountsSubscribePollInterval = 30 * time.Second

// registerAccountsSubscribeChannel wires accounts.subscribe — the streaming
// counterpart to the 4 mutation channels above. Real desktop backend/'s
// accounts.subscribe (backend/src/main/runtime/rpc/methods/accounts.ts) is
// event-driven off RateLimitService.onStateChange; a bare remote Dev Server
// Agent host has no equivalent change-notification infrastructure (no
// fs-watcher on ~/.claude/.claude.json or ~/.codex/auth.json exists
// anywhere in this codebase today), so this is deliberately the
// "poll-based wscompat bridge" SOL-004's own "Agent-side companion work"
// section flagged as the alternative to a server-streaming agent RPC —
// polling accounts.getSnapshot (agent/src/relay/accounts-handler.ts) is
// cheap, local filesystem reads on the target host, not a network call per
// tick. Detects an external `claude login`/`codex login` run directly on
// the host outside any select/remove call this connection ever made.
func registerAccountsSubscribeChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.RegisterStream("accounts.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
		in := decodeOptionalArg[accountsSubscribeArgs](args, 0)
		if in.ConnectionID == "" {
			return nil, fmt.Errorf("ACCOUNTS_NO_CONNECTION: connectionId (devServerId) is required — pick a dev server first")
		}
		relayCtx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		// Fail the subscribe call itself on the FIRST fetch — a connectionId
		// that resolves to nothing (or an agent build predating
		// accounts.getSnapshot) should reject synchronously, not open a
		// subscription doomed to poll-and-swallow-errors forever.
		first, err := fetchAccountsSnapshot(relayCtx, client, in.ConnectionID)
		if err != nil {
			return nil, err
		}

		out := make(chan PushEvent)
		go func() {
			defer close(out)
			// ready: the initial snapshot, matching the real accounts.ts
			// handler's emit({type:'ready', snapshot}) shape (subscriptionId
			// omitted — nothing in this codebase's wscompat layer has a
			// per-subscription unsubscribe channel to correlate it against;
			// ending this subscription is always whole-connection teardown,
			// same limitation notifications.subscribe already has).
			if !sendAccountsEvent(ctx, out, "ready", first) {
				return
			}
			last := first
			ticker := time.NewTicker(accountsSubscribePollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					next, err := fetchAccountsSnapshot(relayCtx, client, in.ConnectionID)
					if err != nil {
						// Transient relay/agent hiccups must not kill an
						// otherwise-healthy subscription — just skip this tick
						// and try again next interval.
						continue
					}
					if reflect.DeepEqual(next, last) {
						continue // unchanged — don't spam the client every tick
					}
					last = next
					if !sendAccountsEvent(ctx, out, "snapshot", next) {
						return
					}
				}
			}
		}()
		return out, nil
	})
}

// fetchAccountsSnapshot relays accounts.getSnapshot — the read-only method
// agent/src/relay/accounts-handler.ts added specifically to back this poll
// loop (see that file's getAccountsSnapshot doc comment). devServerID is,
// like the rest of this file, the value callers still name "connectionId"
// on the wire — see the package doc comment.
func fetchAccountsSnapshot(ctx context.Context, client infrafleetv1.InfraFleetServiceClient, devServerID string) (map[string]any, error) {
	resp, err := client.RelayByDevServer(ctx, &infrafleetv1.RelayByDevServerRequest{
		DevServerId: devServerID,
		Method:      "accounts.getSnapshot",
		ParamsJson:  "{}",
	})
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resp.GetResultJson()), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// sendAccountsEvent writes one accounts.subscribe push event, wrapping
// snapshot in the {type, snapshot} shape
// ProviderAccountsSubscriptionMessage (frontend/src/renderer/src/runtime/
// runtime-provider-accounts-client.ts) expects, and returns false if ctx
// was cancelled before the send could complete (the caller's cue to stop
// without a further write).
func sendAccountsEvent(ctx context.Context, out chan<- PushEvent, msgType string, snapshot map[string]any) bool {
	select {
	case out <- PushEvent{Channel: "accounts.event", Args: []any{map[string]any{"type": msgType, "snapshot": snapshot}}}:
		return true
	case <-ctx.Done():
		return false
	}
}
