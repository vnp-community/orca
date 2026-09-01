// ── onboarding.* ─────────────────────────────────────────────────────────
//
// The old TS backend's onboarding.get (backend/src/main/runtime/rpc/methods/
// onboarding.ts, backed by persistence.ts's Store.getOnboarding()) is purely
// local, per-installation wizard-progress UI state (frontend/src/shared/
// types.ts's OnboardingState/OnboardingChecklistState) — which step of the
// first-run wizard the user finished, which checklist items they've hit.
// It is NOT derived from any tenant/user/project existence check (that's a
// different concept: auth-service's bootstrap.go, which seeds the first
// admin and is deliberately not exposed over any RPC).
//
// Live bug (user-reported): onboarding.get/update never persisted anything
// — every page reload re-showed the onboarding wizard forever, since the
// only state that ever existed was echoed back from the caller's own
// partial update, in memory, for the life of one request. Fixed by
// persisting through tenant-service's GetOnboardingState/SetOnboardingState
// (backed by a new tenant.user_profiles.onboarding_state_json column) — see
// that RPC pair's doc comment in tenant.proto for why this is a dedicated
// per-user store rather than folded into profile settings_json.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// onboardingFlowVersion mirrors frontend/src/shared/constants.ts's
// ONBOARDING_FLOW_VERSION — bump together if the frontend's wizard step
// numbering ever changes.
const onboardingFlowVersion = 4

type onboardingChecklistView struct {
	AddedRepo                bool `json:"addedRepo"`
	ChoseAgent               bool `json:"choseAgent"`
	RanFirstAgent            bool `json:"ranFirstAgent"`
	RanSecondAgentOnSameTask bool `json:"ranSecondAgentOnSameTask"`
	TriedCmdJ                bool `json:"triedCmdJ"`
	ShapedSidebar            bool `json:"shapedSidebar"`
	ReviewedDiff             bool `json:"reviewedDiff"`
	OpenedPr                 bool `json:"openedPr"`
	AddedFolder              bool `json:"addedFolder"`
	OpenedFile               bool `json:"openedFile"`
	RanAgentOnFile           bool `json:"ranAgentOnFile"`
	Dismissed                bool `json:"dismissed"`
}

// set assigns value to the named checklist field — a plain switch rather
// than reflection, matching this codebase's preference for explicit code
// (see e.g. adminUserRoleWire) over magic for a small, fixed field set.
// Unknown item names are ignored (forward-compatible with a frontend built
// against a newer checklist than this binary knows about, rather than
// erroring the whole markChecklistItem call).
func (c *onboardingChecklistView) set(item string, value bool) {
	switch item {
	case "addedRepo":
		c.AddedRepo = value
	case "choseAgent":
		c.ChoseAgent = value
	case "ranFirstAgent":
		c.RanFirstAgent = value
	case "ranSecondAgentOnSameTask":
		c.RanSecondAgentOnSameTask = value
	case "triedCmdJ":
		c.TriedCmdJ = value
	case "shapedSidebar":
		c.ShapedSidebar = value
	case "reviewedDiff":
		c.ReviewedDiff = value
	case "openedPr":
		c.OpenedPr = value
	case "addedFolder":
		c.AddedFolder = value
	case "openedFile":
		c.OpenedFile = value
	case "ranAgentOnFile":
		c.RanAgentOnFile = value
	case "dismissed":
		c.Dismissed = value
	}
}

type onboardingStateView struct {
	FlowVersion       int                     `json:"flowVersion"`
	ClosedAt          *int64                  `json:"closedAt"`
	Outcome           *string                 `json:"outcome"`
	LastCompletedStep int                     `json:"lastCompletedStep"`
	Checklist         onboardingChecklistView `json:"checklist"`
}

func defaultOnboardingStateView() onboardingStateView {
	return onboardingStateView{
		FlowVersion:       onboardingFlowVersion,
		ClosedAt:          nil,
		Outcome:           nil,
		LastCompletedStep: -1, // sentinel: wizard not started, matches getDefaultOnboardingState()
		Checklist:         onboardingChecklistView{},
	}
}

// loadOnboardingState fetches userID's persisted state, falling back to
// defaults on any failure (no state saved yet, transient RPC error, or a
// caller with no resolved user id) — persistence is a nice-to-have for this
// channel family; a hiccup here must never block the onboarding UI itself
// the way it did before this was wired up at all.
func loadOnboardingState(ctx context.Context, id Identity, client tenantv1.TenantServiceClient) onboardingStateView {
	fallback := defaultOnboardingStateView()
	if client == nil || id.UserID == "" {
		return fallback
	}
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	resp, err := client.GetOnboardingState(rpcCtx, &tenantv1.GetOnboardingStateRequest{UserId: id.UserID})
	if err != nil || !resp.GetFound() || resp.GetStateJson() == "" {
		return fallback
	}
	var state onboardingStateView
	if err := json.Unmarshal([]byte(resp.GetStateJson()), &state); err != nil {
		return fallback
	}
	return state
}

// saveOnboardingState persists state for userID — same fail-open contract
// as loadOnboardingState's doc comment: a save failure is swallowed (not
// surfaced as a channel error) so a transient tenant-service hiccup never
// blocks the caller from advancing/closing the wizard, matching this
// channel's pre-persistence behavior when nothing could be saved at all.
func saveOnboardingState(ctx context.Context, id Identity, client tenantv1.TenantServiceClient, state onboardingStateView) {
	if client == nil || id.UserID == "" {
		return
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return
	}
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	_, _ = client.SetOnboardingState(rpcCtx, &tenantv1.SetOnboardingStateRequest{
		UserId:    id.UserID,
		StateJson: string(stateJSON),
	})
}

func registerOnboardingChannels(
	r *Registry,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	tenantClient tenantv1.TenantServiceClient,
) {
	r.Register("onboarding.get", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return loadOnboardingState(ctx, id, tenantClient), nil
	})

	// onboarding.update — frontend/src/renderer/src/components/onboarding/
	// use-onboarding-flow-persistence.ts's closeWith() awaits this on every
	// Skip/Continue-to-completion action and only proceeds (closes the
	// modal) if it resolves. Merges the caller's partial update onto the
	// PERSISTED current state (not bare defaults — see loadOnboardingState)
	// and saves the result, so a reload sees the same progress instead of
	// re-showing the wizard from scratch.
	r.Register("onboarding.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[onboardingUpdateArgs](args, 0)
		if err != nil {
			return nil, err
		}

		out := loadOnboardingState(ctx, id, tenantClient)
		if in.FlowVersion != nil {
			out.FlowVersion = *in.FlowVersion
		}
		if in.ClosedAt != nil {
			out.ClosedAt = in.ClosedAt
		}
		if in.Outcome != nil {
			out.Outcome = in.Outcome
		}
		if in.LastCompletedStep != nil {
			out.LastCompletedStep = *in.LastCompletedStep
		}
		if in.Checklist != nil {
			out.Checklist = *in.Checklist
		}
		saveOnboardingState(ctx, id, tenantClient, out)
		return out, nil
	})

	// onboarding.markChecklistItem — same persist-on-top-of-current-state
	// pattern as onboarding.update, for the one-field-at-a-time checklist
	// mark path (frontend calls this instead of a full onboarding.update
	// for individual product-moment tracking, e.g. "ran first agent").
	r.Register("onboarding.markChecklistItem", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type markArgs struct {
			Item  string `json:"item"`
			Value *bool  `json:"value"`
		}
		in, err := decodeArg[markArgs](args, 0)
		if err != nil {
			return nil, err
		}
		value := true
		if in.Value != nil {
			value = *in.Value
		}
		state := loadOnboardingState(ctx, id, tenantClient)
		state.Checklist.set(in.Item, value)
		saveOnboardingState(ctx, id, tenantClient, state)
		return map[string]any{"marked": true}, nil
	})

	// onboarding.detectAgents — the web build has no paired Electron desktop
	// app, so preflight.detectAgents (a local-PATH-only concept, see
	// channels_dev_server_access_control.go's connection-oriented siblings)
	// can never answer "which agent CLIs are installed" for a browser
	// session. The real question for a dev-server-agent connection is "what's
	// on THAT host's PATH" — answered by relaying the agent's own confirmed
	// preflight.detectAgents RPC (specs/agent/api/agent-rpc-catalog-runtime.md)
	// through the same resolve-devServerId-then-Relay path
	// registerAccountsResolveDevServerConnectionChannel/registerAccountsRelay
	// (channels_accounts.go) already established — no new infra-fleet-service
	// RPC needed.
	r.Register("onboarding.detectAgents", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return onboardingDetectAgents(ctx, id, infraFleetClient, args)
	})
}

// onboardingDetectAgentsArgs' commands mirrors desktop's
// AgentDetectionCommand[] (desktop/src/shared/agent-detection-commands.ts) /
// frontend's TuiAgentDetectionCommand[] — built client-side from
// TUI_AGENT_CONFIG (the single source of truth for the agent catalog) and
// passed through verbatim, so this service does not need its own copy of
// the catalog.
type onboardingDetectAgentsArgs struct {
	DevServerID string           `json:"devServerId"`
	Commands    []map[string]any `json:"commands"`
}

type onboardingDetectAgentsResult struct {
	Agents      []string `json:"agents"`
	Platform    *string  `json:"platform"`
	DevServerID string   `json:"devServerId"`
}

func onboardingDetectAgents(
	ctx context.Context,
	id Identity,
	client infrafleetv1.InfraFleetServiceClient,
	args []json.RawMessage,
) (any, error) {
	in, err := decodeArg[onboardingDetectAgentsArgs](args, 0)
	if err != nil {
		return nil, err
	}
	if in.DevServerID == "" {
		return nil, fmt.Errorf("ONBOARDING_NO_DEV_SERVER: devServerId is required")
	}

	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	paramsJSON, err := json.Marshal(map[string]any{"commands": in.Commands})
	if err != nil {
		return nil, err
	}
	// Why RelayByDevServer, not ResolveConnection+Relay: ResolveConnection
	// answers "is there an infra.connections row for this dev server", a
	// different concept from "is the agent's session live" (see
	// usecase.RelayByDevServer's doc comment) — a dev server has no
	// connections row until a repo/worktree is bound to it, which hasn't
	// happened yet during onboarding's agent-detection step.
	resp, err := client.RelayByDevServer(rpcCtx, &infrafleetv1.RelayByDevServerRequest{
		DevServerId: in.DevServerID,
		Method:      "preflight.detectAgents",
		ParamsJson:  string(paramsJSON),
	})
	if err != nil {
		// Why: "no live connection right now" is a legitimate onboarding
		// state (agent not connected yet), not an error — mirrors
		// registerAccountsResolveDevServerConnectionChannel's same
		// tolerance. Any OTHER error (e.g. an unknown devServerId) still
		// propagates.
		if status.Code(err) == codes.FailedPrecondition {
			return onboardingDetectAgentsResult{Agents: []string{}, DevServerID: in.DevServerID}, nil
		}
		return nil, err
	}

	var relayResult struct {
		Agents   []string `json:"agents"`
		Platform *string  `json:"platform"`
	}
	if raw := resp.GetResultJson(); raw != "" {
		if err := json.Unmarshal([]byte(raw), &relayResult); err != nil {
			return nil, fmt.Errorf("onboarding.detectAgents: decoding relay result: %w", err)
		}
	}
	if relayResult.Agents == nil {
		relayResult.Agents = []string{}
	}
	return onboardingDetectAgentsResult{
		Agents:      relayResult.Agents,
		Platform:    relayResult.Platform,
		DevServerID: in.DevServerID,
	}, nil
}

// onboardingUpdateArgs mirrors the frontend's Partial<OnboardingState> —
// every field optional, pointer so "field absent" is distinguishable from
// "field present with its zero value" (e.g. lastCompletedStep: 0 is a real
// value, distinct from "not sent").
type onboardingUpdateArgs struct {
	FlowVersion       *int                     `json:"flowVersion"`
	ClosedAt          *int64                   `json:"closedAt"`
	Outcome           *string                  `json:"outcome"`
	LastCompletedStep *int                     `json:"lastCompletedStep"`
	Checklist         *onboardingChecklistView `json:"checklist"`
}
