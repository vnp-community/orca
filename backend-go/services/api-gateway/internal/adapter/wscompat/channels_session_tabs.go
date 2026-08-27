// channels_session_tabs.go registers session.tabs.listAll/subscribeAll —
// the worktree-tab view frontend/src/renderer/src/runtime/
// web-session-tabs-sync.ts polls/subscribes to. The old TS backend
// (backend/src/main/runtime/orca-runtime-mobile-session-tabs.ts) modeled a
// richer "mobile session tab" concept (per-tab pane metadata, hydrated from
// workspace session state). backend-go has no equivalent: infra-fleet-
// service's TerminalSession (ListTerminalSessions) is the only session-
// shaped data that exists, and it's connection-scoped, not worktree/tab-
// scoped — see proto/orca/infrafleet/v1/infrafleet.proto's TerminalSession
// message. This is a deliberate scope cut (see
// docs/execution-plan.md §0's frontend-compatibility-layer coverage table):
// one terminal session = one tab, joined to its worktree via
// ResolveConnection's connection_id -> worktree_id lookup. No new pane/tab
// concept is added to infra-fleet-service.
package wscompat

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"time"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerSessionTabsChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	registerSessionTabsListAllChannel(r, client)
	registerSessionTabsSubscribeAllChannel(r, client)
}

// sessionTabsSnapshot mirrors the TS backend's RuntimeMobileSessionTabsResult
// — one entry per worktree that has at least one live terminal session.
type sessionTabsSnapshot struct {
	WorktreeID string                `json:"worktreeId"`
	Tabs       []terminalSessionView `json:"tabs"`
}

func registerSessionTabsListAllChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("session.tabs.listAll", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		snapshots, err := fetchSessionTabsSnapshots(ctx, client)
		if err != nil {
			return nil, err
		}
		return map[string]any{"snapshots": snapshots}, nil
	})
}

// sessionTabsSubscribePollInterval is a var, not a const, so tests can
// shrink it — mirrors accountsSubscribePollInterval's precedent
// (channels_accounts.go).
var sessionTabsSubscribePollInterval = 30 * time.Second

// registerSessionTabsSubscribeAllChannel is session.tabs.subscribeAll — the
// streaming counterpart to listAll. infra-fleet-service has no native
// change-notification primitive for terminal-session state (ListTerminalSessions
// is a plain unary poll), so this follows accounts.subscribe's exact
// precedent (channels_accounts.go's registerAccountsSubscribeChannel):
// synchronous first fetch (fail loudly on error, no doomed subscription),
// then a background goroutine polling and diffing per-worktree.
func registerSessionTabsSubscribeAllChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.RegisterStream("session.tabs.subscribeAll", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
		relayCtx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		first, err := fetchSessionTabsSnapshots(relayCtx, client)
		if err != nil {
			return nil, err
		}

		out := make(chan PushEvent)
		go func() {
			defer close(out)
			// snapshots: the initial full list, matching the TS handler's
			// emit({type:'snapshots', snapshots}) shape (session-tabs.ts).
			if !sendSessionTabsSnapshotsEvent(ctx, out, first) {
				return
			}
			last := snapshotsByWorktree(first)
			ticker := time.NewTicker(sessionTabsSubscribePollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					next, err := fetchSessionTabsSnapshots(relayCtx, client)
					if err != nil {
						// Transient relay hiccups must not kill an otherwise
						// healthy subscription — skip this tick, try again.
						continue
					}
					nextByWorktree := snapshotsByWorktree(next)
					for worktreeID := range unionWorktreeIDs(last, nextByWorktree) {
						prev, hadPrev := last[worktreeID]
						cur, hasCur := nextByWorktree[worktreeID]
						if hadPrev && hasCur && reflect.DeepEqual(prev, cur) {
							continue // unchanged — don't spam the client every tick
						}
						if !hasCur {
							// Every tab in this worktree closed since the last
							// tick — still worth an "updated" push with an
							// empty tab list, matching onMobileSessionTabsChanged's
							// per-worktree firing in the TS handler.
							cur = sessionTabsSnapshot{WorktreeID: worktreeID, Tabs: []terminalSessionView{}}
						}
						if !sendSessionTabsUpdatedEvent(ctx, out, cur) {
							return
						}
					}
					last = nextByWorktree
				}
			}
		}()
		return out, nil
	})
}

// fetchSessionTabsSnapshots lists every terminal session for the tenant
// (ListTerminalSessionsRequest with an empty connection_id — "all sessions
// for the caller's tenant" per its proto doc comment) and groups them by
// worktree, resolved via ResolveConnection's connection_id -> worktree_id
// lookup (infrafleet.proto's ResolveConnectionResponse.worktree_id doc
// comment). One ResolveConnection call per DISTINCT connection_id in this
// fetch, not per session or per worktree — cached in worktreeIDByConnection
// for the duration of a single fetch since TerminalSession itself carries
// no worktree_id. Sessions whose connection resolves to no worktree_id
// (local/non-worktree execution) are dropped: session.tabs is inherently
// worktree-shaped.
func fetchSessionTabsSnapshots(ctx context.Context, client infrafleetv1.InfraFleetServiceClient) ([]sessionTabsSnapshot, error) {
	resp, err := client.ListTerminalSessions(ctx, &infrafleetv1.ListTerminalSessionsRequest{})
	if err != nil {
		return nil, err
	}

	tabsByWorktree := make(map[string][]terminalSessionView)
	worktreeIDByConnection := make(map[string]string)
	for _, s := range resp.GetSessions() {
		connID := s.GetConnectionId()
		worktreeID, resolved := worktreeIDByConnection[connID]
		if !resolved {
			rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
			out, err := client.ResolveConnection(rpcCtx, &infrafleetv1.ResolveConnectionRequest{ConnectionId: connID})
			cancel()
			if err != nil {
				return nil, err
			}
			worktreeID = out.GetWorktreeId()
			worktreeIDByConnection[connID] = worktreeID
		}
		if worktreeID == "" {
			continue
		}
		tabsByWorktree[worktreeID] = append(tabsByWorktree[worktreeID], toTerminalSessionView(s))
	}

	snapshots := make([]sessionTabsSnapshot, 0, len(tabsByWorktree))
	for worktreeID, tabs := range tabsByWorktree {
		snapshots = append(snapshots, sessionTabsSnapshot{WorktreeID: worktreeID, Tabs: tabs})
	}
	// Stable order so reflect.DeepEqual-based diffing across polling ticks
	// isn't fooled by Go's randomized map iteration order.
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].WorktreeID < snapshots[j].WorktreeID })
	return snapshots, nil
}

func snapshotsByWorktree(snapshots []sessionTabsSnapshot) map[string]sessionTabsSnapshot {
	m := make(map[string]sessionTabsSnapshot, len(snapshots))
	for _, s := range snapshots {
		m[s.WorktreeID] = s
	}
	return m
}

func unionWorktreeIDs(a, b map[string]sessionTabsSnapshot) map[string]struct{} {
	keys := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	return keys
}

// sendSessionTabsSnapshotsEvent/sendSessionTabsUpdatedEvent write
// session.tabs.subscribeAll push events, matching SessionTabsStreamEvent's
// {type:'snapshots', snapshots} / {...RuntimeMobileSessionTabsResult,
// type:'updated'} shapes (frontend's web-session-tabs-sync.ts). Both return
// false if ctx was cancelled before the send could complete.
func sendSessionTabsSnapshotsEvent(ctx context.Context, out chan<- PushEvent, snapshots []sessionTabsSnapshot) bool {
	select {
	case out <- PushEvent{Channel: "session.tabs.event", Args: []any{map[string]any{"type": "snapshots", "snapshots": snapshots}}}:
		return true
	case <-ctx.Done():
		return false
	}
}

func sendSessionTabsUpdatedEvent(ctx context.Context, out chan<- PushEvent, snapshot sessionTabsSnapshot) bool {
	select {
	case out <- PushEvent{Channel: "session.tabs.event", Args: []any{map[string]any{
		"type": "updated", "worktreeId": snapshot.WorktreeID, "tabs": snapshot.Tabs,
	}}}:
		return true
	case <-ctx.Done():
		return false
	}
}
