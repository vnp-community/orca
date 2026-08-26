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
// AGENT-SIDE WORK LANDED (TASK-023, specs/backend-go/bugs/missing-v1/tasks/
// TASK-023-document-accounts-agent-gap.md): agent/src/relay/accounts-handler.ts
// implements all 4 relayed methods below for real, plus the read-only
// accounts.getSnapshot registerAccountsSubscribeChannel polls. Reachability
// still requires a connectionId the frontend's documented call sites don't
// send today — see accountsRelayArgs's doc comment and TASK-023's "Open
// prerequisite" note, unchanged by this file.
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
			// Same fail-loud-not-guess posture as registerAccountsRelay above —
			// see TASK-023's "Open prerequisite" note.
			return nil, fmt.Errorf("ACCOUNTS_NO_CONNECTION: connectionId is required until the frontend contract adds it")
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
// loop (see that file's getAccountsSnapshot doc comment).
func fetchAccountsSnapshot(ctx context.Context, client infrafleetv1.InfraFleetServiceClient, connectionID string) (map[string]any, error) {
	resp, err := client.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: connectionID,
		Method:       "accounts.getSnapshot",
		ParamsJson:   "{}",
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
