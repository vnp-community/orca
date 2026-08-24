// Channel handlers with REAL backend-go logic behind them — every other
// channel the frontend can call falls through to registry.go's
// notImplementedHandler. See docs/execution-plan.md's frontend-compatibility
// coverage table for the full list of what's wired here vs. not.
//
// Arg-shape caveat: the invoke envelope's `args` is a generic JSON array
// (rpc-client.ts's `invoke(channel, ...args)` spreads whatever the call
// site passed). specs/frontend/api/rpc-catalog.md documents method names
// and call sites but not param shapes. Every handler below decodes args[0]
// as a single JSON object with the field names its proto request expects —
// a reasonable, common convention, but NOT verified against every real
// frontend call site's actual argument marshaling. Treat as best-effort;
// verify against the actual call site before depending on a specific
// channel in production.
//
// Channel count: 13 wired for real as of the devServer.*/fleet.* pass —
// annotation.{create,list,update,delete} (4), task.{create,get} (2),
// git.{status,diff} (2), automation.runNow (1), preflight.check (1),
// devServer.{list,add} (2), fleet.health.checkAll (1).
package wscompat

import (
	"context"
	"encoding/json"
	"time"

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	automationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/automation/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// rateLimitReader is a minimal read interface over usecase.RateLimiter so
// this file stays testable without importing the full concrete struct.
// Satisfied by *usecase.RateLimiter after TASK-003 adds RPS()/Burst().
type rateLimitReader interface {
	RPS() float64
	Burst() int
}

// rateLimitInfo is the wire shape rateLimits.get returns — mirrors the
// frontend's RateLimitInfo type.
type rateLimitInfo struct {
	RequestsPerSecond float64 `json:"requestsPerSecond"`
	Burst             int     `json:"burst"`
}

// rpcTimeout is the per-RPC deadline applied to each outbound gRPC call inside
// a channel handler. Shorter than handler.go's invokeTimeout (25s) so a slow
// or unreachable downstream service fails fast with a meaningful error message
// rather than occupying the full dispatch window.
//
// Invariant: rpcTimeout (8s) < invokeTimeout (25s) — verified by
// TestRPCTimeoutConstant_ShorterThanInvokeTimeout in channels_test.go.
const rpcTimeout = 8 * time.Second

// RegisterRealChannels wires every channel this pass gives real backend-go
// implementations to. Called once from main.go's composition root with the
// gRPC clients already dialed there.
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	rateLimits rateLimitReader,
) {
	registerAnnotationChannels(r, annotationClient)
	registerTaskChannels(r, taskClient)
	registerGitChannels(r, gitClient)
	registerAutomationChannels(r, automationClient)
	registerPreflightChannels(r)
	registerDevServerChannels(r, infraFleetClient)
	registerFleetChannels(r, infraFleetClient)
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
}

// ── annotation.* ────────────────────────────────────────────────────────

type annotationAnchorArg struct {
	RepoID   string `json:"repoId"`
	FilePath string `json:"filePath"`
	Line     int32  `json:"line"`
	Ref      string `json:"ref"`
}

func registerAnnotationChannels(r *Registry, client annotationv1.AnnotationServiceClient) {
	r.Register("annotation.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Anchor    annotationAnchorArg `json:"anchor"`
			Content   string              `json:"content"`
			RequestID string              `json:"requestId"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.CreateAnnotation(ctx, &annotationv1.CreateAnnotationRequest{
			Anchor: &annotationv1.Anchor{
				RepoId:   in.Anchor.RepoID,
				FilePath: in.Anchor.FilePath,
				Line:     in.Anchor.Line,
				Ref:      in.Anchor.Ref,
			},
			Content:   in.Content,
			RequestId: in.RequestID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetAnnotation(), nil
	})

	r.Register("annotation.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			RepoID    string `json:"repoId"`
			FilePath  string `json:"filePath"`
			PageToken string `json:"pageToken"`
			PageSize  int32  `json:"pageSize"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.ListAnnotations(ctx, &annotationv1.ListAnnotationsRequest{
			RepoId:    in.RepoID,
			FilePath:  in.FilePath,
			PageToken: in.PageToken,
			PageSize:  in.PageSize,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("annotation.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID       string `json:"id"`
			Content  string `json:"content"`
			Resolved bool   `json:"resolved"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.UpdateAnnotation(ctx, &annotationv1.UpdateAnnotationRequest{
			Id: in.ID, Content: in.Content, Resolved: in.Resolved,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetAnnotation(), nil
	})

	r.Register("annotation.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type deleteArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[deleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if _, err := client.DeleteAnnotation(ctx, &annotationv1.DeleteAnnotationRequest{Id: in.ID}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})
}

// ── task.* (subset: create/get — the DAG/grant channels backing
// task-service's real BFS/cycle-detection logic; execute/AI-decompose are
// not wired since they depend on infra-fleet-service/orchestration-service,
// still stubs — see task-service's own README) ─────────────────────────

func registerTaskChannels(r *Registry, client taskv1.TaskServiceClient) {
	r.Register("task.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Title    string `json:"title"`
			ParentID string `json:"parentId"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.CreateTask(ctx, &taskv1.CreateTaskRequest{
			TenantId: id.TenantID, Title: in.Title, ParentId: in.ParentID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetTask(), nil
	})

	r.Register("task.get", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[getArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GetTask(ctx, &taskv1.GetTaskRequest{Id: in.ID})
		if err != nil {
			return nil, err
		}
		return resp.GetTask(), nil
	})
}

// ── git.* (subset: status/diff — the two ops git-gateway-service
// implements for real against the local git binary; commit/push/pull relay
// to the Dev Server Agent, still a stub) ────────────────────────────────

func registerGitChannels(r *Registry, client gitgatewayv1.GitGatewayServiceClient) {
	r.Register("git.status", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type statusArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[statusArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GetStatus(ctx, &gitgatewayv1.GetStatusRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.diff", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type diffArgs struct {
			WorktreeID string `json:"worktreeId"`
			Staged     bool   `json:"staged"`
		}
		in, err := decodeArg[diffArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GetDiff(ctx, &gitgatewayv1.GetDiffRequest{WorktreeId: in.WorktreeID, Staged: in.Staged})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}

// ── automation.runNow (the one real cross-service call in this whole
// scaffold — see automation-service's README) ──────────────────────────

func registerAutomationChannels(r *Registry, client automationv1.AutomationServiceClient) {
	r.Register("automation.runNow", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type runNowArgs struct {
			AutomationID string `json:"automationId"`
			RequestID    string `json:"requestId"`
		}
		in, err := decodeArg[runNowArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.RunNow(ctx, &automationv1.RunNowRequest{
			AutomationId: in.AutomationID, RequestId: in.RequestID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetRun(), nil
	})
}

// ── devServer.* / fleet.health.* (infra-fleet-service) ──────────────────
//
// Unlike every register*Channels above, these handlers MUST call
// gatewaygrpc.AttachIdentity before invoking the client: infra-fleet-service
// requires tenant metadata on the OUTBOUND gRPC context (it has no
// tenant_id request field to fall back on for List/Register — and even
// GetFleetHealthRequest's tenant_id field is ignored server-side, see the
// proto comment). Every other channel in this file calls its client with
// the bare inbound ctx and silently does NOT forward tenant metadata — do
// not copy that pattern here, it would make every call fail with
// INFRA_NO_TENANT.
//
// Field-shape gap: infra-fleet-service's DevServer message is
// {id, tenant_id, host, mode} only — none of the frontend's
// name/status/platform/arch/nodeVersion/lastConnectedAt/lastError/
// workspaceDir/addedAt/capabilities fields exist server-side yet. toDevServerView
// below fills the required-but-absent fields with honest placeholders
// (host doubles as display name; status is always "disconnected" since
// backend-go doesn't track live relay connection state; everything else
// is null/zero) rather than fabricating data. Same story in reverse for
// devServer.add: DevServerInput's name/sshTargetId/wsUrl have no matching
// proto fields beyond a single generic `host` string — devServerHost below
// picks the best available one, in order wsUrl > sshTargetId > name.
//
// devServer.connect/disconnect/remove/testConnection are deliberately NOT
// registered — infra-fleet-service has no backing RPC for any of them
// (no persisted connection lifecycle or delete endpoint exists yet). They
// fall through to notImplementedHandler like every other unregistered
// channel; this is a known gap, not an oversight.

// devServerView is the wire shape devServer.list/add return — mirrors
// frontend/src/shared/dev-server-types.ts's DevServer type field-for-field,
// including its required (non-optional) null-able fields, so a decoder on
// the frontend written against that type sees keys it expects even when
// this backend has nothing real to put in them yet.
type devServerView struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	ConnectionType  string   `json:"connectionType"`
	SSHTargetID     *string  `json:"sshTargetId,omitempty"`
	WSUrl           *string  `json:"wsUrl,omitempty"`
	Status          string   `json:"status"`
	Platform        *string  `json:"platform"`
	Arch            *string  `json:"arch"`
	NodeVersion     *string  `json:"nodeVersion"`
	LastConnectedAt *int64   `json:"lastConnectedAt"`
	LastError       *string  `json:"lastError"`
	WorkspaceDir    *string  `json:"workspaceDir"`
	AddedAt         int64    `json:"addedAt"`
	Capabilities    []string `json:"capabilities"`
}

// toDevServerView maps a proto DevServer (id/tenant_id/host/mode only) onto
// the frontend's richer DevServer shape — see this section's doc comment
// for which fields are real vs. placeholder.
func toDevServerView(ds *infrafleetv1.DevServer) devServerView {
	view := devServerView{
		ID:             ds.GetId(),
		Name:           ds.GetHost(), // no `name` field server-side — host doubles as display name
		ConnectionType: fromConnectionMode(ds.GetMode()),
		Status:         "disconnected", // backend-go doesn't track live relay connection state yet
	}
	if host := ds.GetHost(); host != "" {
		view.WSUrl = &host
	}
	return view
}

// toConnectionMode maps frontend/src/shared/dev-server-types.ts's
// DevServerConnectionType string union onto the proto ConnectionMode enum.
func toConnectionMode(connectionType string) infrafleetv1.ConnectionMode {
	switch connectionType {
	case "relay-ssh":
		return infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH
	case "relay-websocket":
		return infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_WEBSOCKET
	case "direct-websocket":
		return infrafleetv1.ConnectionMode_CONNECTION_MODE_DIRECT_WEBSOCKET
	default:
		return infrafleetv1.ConnectionMode_CONNECTION_MODE_UNSPECIFIED
	}
}

// fromConnectionMode is toConnectionMode's inverse, for devServer.list's
// response mapping.
func fromConnectionMode(mode infrafleetv1.ConnectionMode) string {
	switch mode {
	case infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH:
		return "relay-ssh"
	case infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_WEBSOCKET:
		return "relay-websocket"
	case infrafleetv1.ConnectionMode_CONNECTION_MODE_DIRECT_WEBSOCKET:
		return "direct-websocket"
	default:
		return ""
	}
}

// devServerHost picks RegisterDevServerRequest's single generic `host`
// string out of DevServerInput's three more specific fields — wsUrl first
// (most literally a connection address), then sshTargetId, then name, since
// backend-go has no field to hold more than one of these at once.
func devServerHost(wsURL, sshTargetID, name string) string {
	switch {
	case wsURL != "":
		return wsURL
	case sshTargetID != "":
		return sshTargetID
	default:
		return name
	}
}

func registerDevServerChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("devServer.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		// Per-RPC deadline: fail fast so the write-back (SOL-001 / TASK-001)
		// still has time to deliver the error to the frontend before invokeTimeout.
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListDevServers(rpcCtx, &infrafleetv1.ListDevServersRequest{})
		if err != nil {
			return nil, err
		}
		views := make([]devServerView, 0, len(resp.GetDevServers()))
		for _, ds := range resp.GetDevServers() {
			views = append(views, toDevServerView(ds))
		}
		return views, nil
	})

	r.Register("devServer.add", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type addArgs struct {
			Name           string `json:"name"`
			ConnectionType string `json:"connectionType"`
			SSHTargetID    string `json:"sshTargetId"`
			WSUrl          string `json:"wsUrl"`
		}
		in, err := decodeArg[addArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		// Per-RPC deadline (same reasoning as devServer.list above).
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.RegisterDevServer(rpcCtx, &infrafleetv1.RegisterDevServerRequest{
			TenantId: id.TenantID,
			Host:     devServerHost(in.WSUrl, in.SSHTargetID, in.Name),
			Mode:     toConnectionMode(in.ConnectionType),
		})
		if err != nil {
			return nil, err
		}
		return toDevServerView(resp.GetDevServer()), nil
	})
}

// ── fleet.health.checkAll ─────────────────────────────────────────────────

// serverHealthView mirrors frontend/src/renderer/src/store/slices/ssh.ts's
// ServerHealthMetrics type. uptimeSeconds/relayVersion/nodeVersion have no
// equivalent on infra-fleet-service's DevServerHealth message — left as
// permanently-nil pointers (marshal to JSON null) rather than fabricated,
// per this file's package doc comment's best-effort/honesty convention.
type serverHealthView struct {
	ServerID         string  `json:"serverId"`
	LastCheckedAt    int64   `json:"lastCheckedAt"`
	IsReachable      bool    `json:"isReachable"`
	UptimeSeconds    *int64  `json:"uptimeSeconds"`
	RelayVersion     *string `json:"relayVersion"`
	NodeVersion      *string `json:"nodeVersion"`
	DiskUsagePercent float64 `json:"diskUsagePercent"`
	CPUUsagePercent  float64 `json:"cpuUsagePercent"`
	MemUsagePercent  float64 `json:"memUsagePercent"`
}

func registerFleetChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("fleet.health.checkAll", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type checkAllArgs struct {
			ServerIDs []string `json:"serverIds"`
		}
		in, err := decodeArg[checkAllArgs](args, 0)
		if err != nil {
			return nil, err
		}

		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		// Per-RPC deadline: GetFleetHealth involves active health checks to
		// dev servers which can be slow — 8s allows for reasonable network
		// latency while still failing before invokeTimeout.
		// TenantId on the request is ignored server-side (tenant always comes
		// from ctx metadata, per infrafleet.proto's comment) — set anyway for
		// readability/documentation at this call site.
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetFleetHealth(rpcCtx, &infrafleetv1.GetFleetHealthRequest{TenantId: id.TenantID})
		if err != nil {
			return nil, err
		}

		// GetFleetHealth returns health for ALL of the tenant's dev servers,
		// not filtered by the requested serverIds (no such filter param
		// exists on the RPC) — filter client-side here to match what the
		// frontend actually asked for.
		wanted := make(map[string]bool, len(in.ServerIDs))
		for _, sid := range in.ServerIDs {
			wanted[sid] = true
		}

		now := time.Now().UnixMilli()
		views := make([]serverHealthView, 0, len(resp.GetStatuses()))
		for _, s := range resp.GetStatuses() {
			if !wanted[s.GetDevServerId()] {
				continue
			}
			views = append(views, serverHealthView{
				ServerID:         s.GetDevServerId(),
				LastCheckedAt:    now,
				IsReachable:      s.GetReachable(),
				DiskUsagePercent: s.GetDiskPercent(),
				CPUUsagePercent:  s.GetCpuPercent(),
				MemUsagePercent:  s.GetRamPercent(),
			})
		}
		return views, nil
	})
}

// ── preflight.check ──────────────────────────────────────────────────────
//
// Registered as a fast, LOCAL (no downstream call) response — see
// docs/execution-plan.md §7. This handler is intentionally local-only: if it is
// observed to time out in production after SOL-001 (TASK-001) and SOL-003
// (TASK-008) are applied, the cause is writeMu contention (BUG-004 Cause B) —
// look for "wscompat: writeMu contention detected" log entries on the same
// connection around the same timestamp.
//
// frontend/src/preload/api-types.ts's PreflightStatus asks about `gh`/`glab`
// CLI installed+authenticated state — that concept doesn't map onto
// backend-go's design: scm-integration-service is a direct OAuth API client,
// deliberately NOT a `gh`/`glab` CLI wrapper. Reporting installed:false/
// authenticated:false for both is the honest answer.
func registerPreflightChannels(r *Registry) {
	r.Register("preflight.check", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]any{
			"git":  map[string]any{"installed": true}, // git-gateway-service's local executor requires the real git binary
			"gh":   map[string]any{"installed": false, "authenticated": false},
			"glab": map[string]any{"installed": false, "authenticated": false},
		}, nil
	})
}

// ── crashReports.* ──────────────────────────────────────────────────────────
//
// backend-go has no crash reporting service — this architecture uses structured
// gRPC error propagation (apperrors.ToGRPCStatus) and OpenTelemetry traces
// instead of a separate crash-report collection service. The frontend calls
// crashReports.getLatestPending on every bootstrap; returning null is the
// honest answer ("no pending crash report"), not a stub — there is genuinely
// nothing to report from backend-go's crash/panic path.
func registerCrashReportChannels(r *Registry) {
	r.Register("crashReports.getLatestPending", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		// null signals "no pending crash report" — matches the frontend's
		// crashReports.getLatestPending contract (nullable return).
		return nil, nil
	})
}

// ── rateLimits.* ────────────────────────────────────────────────────────────
//
// Exposes api-gateway's in-process per-tenant rate limiter configuration.
// The frontend calls rateLimits.get during bootstrap to understand the
// current throttle policy (e.g. for UI-level quota indicators). Returns
// the limiter's configured RPS/burst — not per-tenant counters (those are
// ephemeral per-replica state, not meaningful to expose externally).
func registerRateLimitChannels(r *Registry, rl rateLimitReader) {
	r.Register("rateLimits.get", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return rateLimitInfo{
			RequestsPerSecond: rl.RPS(),
			Burst:             rl.Burst(),
		}, nil
	})
}
