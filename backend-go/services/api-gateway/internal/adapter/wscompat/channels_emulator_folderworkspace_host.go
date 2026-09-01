// Channels for three independently-shippable namespaces bundled into one
// file and one entry point (registerEmulatorFolderWorkspaceHostChannels) —
// see specs/backend-go/bugs/missing-v1/tasks/TASK-046, TASK-048, TASK-061..067,
// TASK-068, TASK-070 for each namespace's own design doc.
package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// protoTimeToRFC3339 formats a possibly-nil *timestamppb.Timestamp for the
// wire — every camelCase view struct in this file/channels_tenant_project.go
// uses it instead of returning the raw Timestamp (which plain encoding/json
// would serialize as its internal {seconds, nanos} fields, not a date
// string). Nil returns "" (falsy on the frontend), not the 1970 epoch
// AsTime() would otherwise produce — checked on the concrete *Timestamp,
// not an interface, so a nil pointer is actually caught (a nil pointer
// boxed into an interface value is itself a non-nil interface).
func protoTimeToRFC3339(t *timestamppb.Timestamp) string {
	if t == nil {
		return ""
	}
	return t.AsTime().Format(time.RFC3339)
}

// protoTimeMillis is protoTimeToRFC3339's epoch-milliseconds counterpart,
// for view structs (projectView) whose frontend type expects a number
// (OrcaProject.createdAt/updatedAt), not a date string.
func protoTimeMillis(t *timestamppb.Timestamp) int64 {
	if t == nil {
		return 0
	}
	return t.AsTime().UnixMilli()
}

// registerEmulatorFolderWorkspaceHostChannels wires every channel this
// file gives real backend-go implementations to: emulator.* (relay when a
// connectionId is present, TASK-048; honest permanent-unsupported stub
// otherwise, TASK-046), host.* (relay when a connectionId is present,
// TASK-070; honest local-answer stub otherwise, TASK-068), and
// folderWorkspace.* (real project-service CRUD, TASK-066).
func registerEmulatorFolderWorkspaceHostChannels(r *Registry, projectClient projectv1.ProjectServiceClient, infraFleetClient infrafleetv1.InfraFleetServiceClient) {
	registerEmulatorChannels(r, infraFleetClient)
	registerHostChannels(r, infraFleetClient)
	registerFolderWorkspaceChannels(r, projectClient)
}

// ── emulator.* ──────────────────────────────────────────────────────────
//
// Mobile emulator/simulator control (ADB/xcrun simctl device driving) has
// no backend-go-local implementation and, per
// 02-microservices-decomposition.md's "What's deliberately not a separate
// service" section, is explicitly excluded from the Go server deployment
// by design. The architecturally sound alternative — relay to the Dev
// Server Agent via infra-fleet-service's real ListEmulatorDevices/
// GetEmulatorAvailability/AttachEmulatorSession/SendEmulatorTap/
// SendEmulatorGesture/SendEmulatorButton/RotateEmulator/ShutdownEmulator
// RPCs (TASK-048) — is now wired for real below, but is honestly inert
// until agent/ gains a device.* JSON-RPC surface: every relay call reaches
// a real agent and gets back a real, permanent
// FailedPrecondition/INFRA_EMULATOR_UNSUPPORTED (see
// usecase.EmulatorRelay in infra-fleet-service), which this file surfaces
// as-is rather than translating further.
//
// Per TASK-048's own design, there is NO local/backend-host fallback: a
// call with no connectionId (or one that can't be resolved) gets the same
// permanent errEmulatorNotSupported answer TASK-046 shipped, not a
// disguised relay attempt — driving emulators on the shared backend-go
// host is out of scope by design, unlike host.* below, which DOES have an
// honest local answer to fall back to.
var errEmulatorNotSupported = errors.New(
	"mobile emulator control is not supported by the Go backend — " +
		"see specs/backend-go/bugs/missing-v1/solutions/SOL-008-emulator-channels.md")

type emulatorConnectionArgs struct {
	ConnectionID string `json:"connectionId"`
}

func registerEmulatorChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("emulator.listDevices", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in := decodeOptionalArg[emulatorConnectionArgs](args, 0)
		if in.ConnectionID == "" {
			return nil, errEmulatorNotSupported
		}
		rpcCtx, cancel := context.WithTimeout(gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}), rpcTimeout)
		defer cancel()
		resp, err := client.ListEmulatorDevices(rpcCtx, &infrafleetv1.ListEmulatorDevicesRequest{ConnectionId: in.ConnectionID})
		if err != nil {
			return nil, err
		}
		return resp.GetDevices(), nil
	})

	r.Register("emulator.availability", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in := decodeOptionalArg[emulatorConnectionArgs](args, 0)
		// Unlike every other emulator.* channel, GetEmulatorAvailability has
		// no connectionId requirement on infra-fleet-service's side either
		// (see its usecase's doc comment) — always relay so a genuinely
		// empty connectionId still gets infra-fleet-service's honest
		// false/reason answer instead of this file's harder permanent error.
		rpcCtx, cancel := context.WithTimeout(gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}), rpcTimeout)
		defer cancel()
		resp, err := client.GetEmulatorAvailability(rpcCtx, &infrafleetv1.GetEmulatorAvailabilityRequest{ConnectionId: in.ConnectionID})
		if err != nil {
			return nil, err
		}
		return map[string]any{"available": resp.GetAvailable(), "reason": resp.GetReason()}, nil
	})

	r.Register("emulator.attach", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type attachArgs struct {
			ConnectionID string `json:"connectionId"`
			DeviceID     string `json:"deviceId"`
		}
		in := decodeOptionalArg[attachArgs](args, 0)
		if in.ConnectionID == "" {
			return nil, errEmulatorNotSupported
		}
		rpcCtx, cancel := context.WithTimeout(gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}), rpcTimeout)
		defer cancel()
		return client.AttachEmulatorSession(rpcCtx, &infrafleetv1.AttachEmulatorSessionRequest{ConnectionId: in.ConnectionID, DeviceId: in.DeviceID})
	})

	r.Register("emulator.tap", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type tapArgs struct {
			ConnectionID string `json:"connectionId"`
			SessionID    string `json:"sessionId"`
			X            int32  `json:"x"`
			Y            int32  `json:"y"`
		}
		in := decodeOptionalArg[tapArgs](args, 0)
		if in.ConnectionID == "" {
			return nil, errEmulatorNotSupported
		}
		rpcCtx, cancel := context.WithTimeout(gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}), rpcTimeout)
		defer cancel()
		_, err := client.SendEmulatorTap(rpcCtx, &infrafleetv1.SendEmulatorTapRequest{
			ConnectionId: in.ConnectionID, SessionId: in.SessionID, X: in.X, Y: in.Y,
		})
		return map[string]bool{"ok": err == nil}, err
	})

	r.Register("emulator.gesture", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type gestureArgs struct {
			ConnectionID string `json:"connectionId"`
			SessionID    string `json:"sessionId"`
			StartX       int32  `json:"startX"`
			StartY       int32  `json:"startY"`
			EndX         int32  `json:"endX"`
			EndY         int32  `json:"endY"`
			DurationMs   int32  `json:"durationMs"`
		}
		in := decodeOptionalArg[gestureArgs](args, 0)
		if in.ConnectionID == "" {
			return nil, errEmulatorNotSupported
		}
		rpcCtx, cancel := context.WithTimeout(gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}), rpcTimeout)
		defer cancel()
		_, err := client.SendEmulatorGesture(rpcCtx, &infrafleetv1.SendEmulatorGestureRequest{
			ConnectionId: in.ConnectionID, SessionId: in.SessionID,
			StartX: in.StartX, StartY: in.StartY, EndX: in.EndX, EndY: in.EndY, DurationMs: in.DurationMs,
		})
		return map[string]bool{"ok": err == nil}, err
	})

	r.Register("emulator.button", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type buttonArgs struct {
			ConnectionID string `json:"connectionId"`
			SessionID    string `json:"sessionId"`
			Button       string `json:"button"`
		}
		in := decodeOptionalArg[buttonArgs](args, 0)
		if in.ConnectionID == "" {
			return nil, errEmulatorNotSupported
		}
		rpcCtx, cancel := context.WithTimeout(gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}), rpcTimeout)
		defer cancel()
		_, err := client.SendEmulatorButton(rpcCtx, &infrafleetv1.SendEmulatorButtonRequest{
			ConnectionId: in.ConnectionID, SessionId: in.SessionID, Button: in.Button,
		})
		return map[string]bool{"ok": err == nil}, err
	})

	r.Register("emulator.rotate", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type rotateArgs struct {
			ConnectionID string `json:"connectionId"`
			SessionID    string `json:"sessionId"`
			Orientation  string `json:"orientation"`
		}
		in := decodeOptionalArg[rotateArgs](args, 0)
		if in.ConnectionID == "" {
			return nil, errEmulatorNotSupported
		}
		rpcCtx, cancel := context.WithTimeout(gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}), rpcTimeout)
		defer cancel()
		_, err := client.RotateEmulator(rpcCtx, &infrafleetv1.RotateEmulatorRequest{
			ConnectionId: in.ConnectionID, SessionId: in.SessionID, Orientation: in.Orientation,
		})
		return map[string]bool{"ok": err == nil}, err
	})

	r.Register("emulator.shutdown", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type shutdownArgs struct {
			ConnectionID string `json:"connectionId"`
			SessionID    string `json:"sessionId"`
		}
		in := decodeOptionalArg[shutdownArgs](args, 0)
		if in.ConnectionID == "" {
			return nil, errEmulatorNotSupported
		}
		rpcCtx, cancel := context.WithTimeout(gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}), rpcTimeout)
		defer cancel()
		_, err := client.ShutdownEmulator(rpcCtx, &infrafleetv1.ShutdownEmulatorRequest{
			ConnectionId: in.ConnectionID, SessionId: in.SessionID,
		})
		return map[string]bool{"ok": err == nil}, err
	})
}

// ── host.* ──────────────────────────────────────────────────────────────
//
// WSL/PowerShell/git-bash availability, now resolved per-target via
// infra-fleet-service's real GetHostCapabilities RPC (TASK-070) when a
// connectionId is present in the request. Per BUG-011, the old backend
// probed only its own process host, never a per-target dev server — this
// closes that gap for real, but is honestly inert until agent/ gains a
// host.capabilities method: a resolved connectionId reaches a real agent
// and gets back a real, permanent
// FailedPrecondition/INFRA_HOST_CAPABILITIES_UNSUPPORTED (see
// usecase.GetHostCapabilities in infra-fleet-service).
//
// Unlike emulator.* above, host.* DOES keep its local-honest-answer
// fallback: no connectionId in the request (today's frontend contract,
// same as before this pass) skips the relay entirely and answers
// false/[] directly, matching TASK-068's original stub and
// infra-fleet-service's own GetHostCapabilities "conn == nil" branch — the
// two are kept in sync deliberately so a bug in one doesn't silently
// diverge from the other.
type hostConnectionArgs struct {
	ConnectionID string `json:"connectionId"`
}

func registerHostChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("host.wsl.isAvailable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		caps, err := resolveHostCapabilities(ctx, id, client, args)
		if err != nil {
			return nil, err
		}
		return map[string]bool{"available": caps.GetWslAvailable()}, nil
	})
	r.Register("host.wsl.listDistros", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		caps, err := resolveHostCapabilities(ctx, id, client, args)
		if err != nil {
			return nil, err
		}
		distros := caps.GetWslDistros()
		if distros == nil {
			distros = []string{}
		}
		return distros, nil
	})
	r.Register("host.pwsh.isAvailable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		caps, err := resolveHostCapabilities(ctx, id, client, args)
		if err != nil {
			return nil, err
		}
		return map[string]bool{"available": caps.GetPwshAvailable()}, nil
	})
	r.Register("host.gitBash.isAvailable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		caps, err := resolveHostCapabilities(ctx, id, client, args)
		if err != nil {
			return nil, err
		}
		return map[string]bool{"available": caps.GetGitBashAvailable()}, nil
	})
}

// resolveHostCapabilities is the single relay-or-local-answer decision
// shared by all 4 host.* channels — a short-TTL cache per this file's
// package doc comment note is a follow-up, not added here to keep this
// pass's diff to the plumbing itself.
func resolveHostCapabilities(ctx context.Context, id Identity, client infrafleetv1.InfraFleetServiceClient, args []json.RawMessage) (*infrafleetv1.GetHostCapabilitiesResponse, error) {
	in := decodeOptionalArg[hostConnectionArgs](args, 0)
	if in.ConnectionID == "" {
		// Local honest answer — no relay target, see this file's host.*
		// doc comment.
		return &infrafleetv1.GetHostCapabilitiesResponse{WslDistros: []string{}}, nil
	}
	rpcCtx, cancel := context.WithTimeout(gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}), rpcTimeout)
	defer cancel()
	return client.GetHostCapabilities(rpcCtx, &infrafleetv1.GetHostCapabilitiesRequest{ConnectionId: in.ConnectionID})
}

// folderWorkspaceView/toFolderWorkspaceView: protoc-gen-go's own
// encoding/json struct tags are snake_case (e.g. `json:"dev_server_id"`,
// `json:"project_group_id"`), but this envelope's Result field
// (envelope.go) is serialized via plain encoding/json (wsjson.Write), not
// protojson — returning the raw *projectv1.FolderWorkspace silently ships
// dev_server_id/added_by/created_at/project_group_id (all undefined to a
// camelCase-only frontend). Same bug class already fixed for profile.* in
// channels_tenant_project.go (see that file's userProfileView) — found
// live here during Phase 4b's folder-workspace grouping pass. CreatedAt is
// formatted as RFC3339 (not epoch millis) to match
// createFolderWorkspace/mergeCreatedFolderWorkspaceResponse's
// `new Date(result.createdAt).getTime()` parsing on the frontend.
type folderWorkspaceView struct {
	ID             string `json:"id"`
	DevServerID    string `json:"devServerId"`
	Path           string `json:"path"`
	Name           string `json:"name"`
	AddedBy        string `json:"addedBy"`
	CreatedAt      string `json:"createdAt,omitempty"`
	ProjectGroupID string `json:"projectGroupId"`
}

func toFolderWorkspaceView(fw *projectv1.FolderWorkspace) folderWorkspaceView {
	return folderWorkspaceView{
		ID: fw.GetId(), DevServerID: fw.GetDevServerId(), Path: fw.GetPath(),
		Name: fw.GetName(), AddedBy: fw.GetAddedBy(),
		CreatedAt:      protoTimeToRFC3339(fw.GetCreatedAt()),
		ProjectGroupID: fw.GetProjectGroupId(),
	}
}

// ── folderWorkspace.* ─────────────────────────────────────────────────────
//
// Straightforward CRUD dispatch against project-service's FolderWorkspace
// RPCs (TASK-061..065) — mirrors registerAnnotationChannels's shape. Calls
// go through with the bare inbound ctx, no gatewaygrpc.AttachIdentity: like
// every project-service RPC this file's registerAnnotationChannels/etc.
// already call, tenant/user are pulled from the interceptor-populated
// context on project-service's own side (common/tenant), not from request
// fields or outbound metadata — see project-service's
// usecase.FolderWorkspaceUseCase.Create for the receiving end of that
// contract. This differs from registerDevServerChannels'
// infra-fleet-service calls, which DO need AttachIdentity — see that
// section's doc comment for why the two backends differ.
func registerFolderWorkspaceChannels(r *Registry, client projectv1.ProjectServiceClient) {
	r.Register("folderWorkspace.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			DevServerID    string `json:"devServerId"`
			Path           string `json:"path"`
			Name           string `json:"name"`
			ProjectGroupID string `json:"projectGroupId"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.CreateFolderWorkspace(ctx, &projectv1.CreateFolderWorkspaceRequest{
			DevServerId: in.DevServerID, Path: in.Path, Name: in.Name, ProjectGroupId: in.ProjectGroupID,
		})
		if err != nil {
			return nil, err
		}
		return toFolderWorkspaceView(resp.GetFolderWorkspace()), nil
	})

	r.Register("folderWorkspace.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.UpdateFolderWorkspace(ctx, &projectv1.UpdateFolderWorkspaceRequest{Id: in.ID, Name: in.Name})
		if err != nil {
			return nil, err
		}
		return toFolderWorkspaceView(resp.GetFolderWorkspace()), nil
	})

	r.Register("folderWorkspace.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type deleteArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[deleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if _, err := client.DeleteFolderWorkspace(ctx, &projectv1.DeleteFolderWorkspaceRequest{Id: in.ID}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("folderWorkspace.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		resp, err := client.ListFolderWorkspaces(ctx, &projectv1.ListFolderWorkspacesRequest{})
		if err != nil {
			return nil, err
		}
		// Why {folderWorkspaces: [...]}, not a bare array: repos.ts's
		// fetchFolderWorkspacesForTarget reads `.folderWorkspaces` off this
		// response — matches that established wrapper shape.
		fws := resp.GetFolderWorkspaces()
		views := make([]folderWorkspaceView, 0, len(fws))
		for _, fw := range fws {
			views = append(views, toFolderWorkspaceView(fw))
		}
		return map[string]any{"folderWorkspaces": views}, nil
	})

	r.Register("folderWorkspace.getPathStatus", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type statusArgs struct {
			DevServerID string `json:"devServerId"`
			Path        string `json:"path"`
		}
		in, err := decodeArg[statusArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GetFolderWorkspacePathStatus(ctx, &projectv1.GetFolderWorkspacePathStatusRequest{
			DevServerId: in.DevServerID, Path: in.Path,
		})
		if err != nil {
			return nil, err
		}
		// Why a view, not raw resp: existing_folder_workspace_id is the only
		// multi-word field here (status is already a plain string, not an
		// enum) — same camelCase bug as folderWorkspaceView above.
		return map[string]any{
			"status":                    resp.GetStatus(),
			"existingFolderWorkspaceId": resp.GetExistingFolderWorkspaceId(),
		}, nil
	})
}
