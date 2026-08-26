// Channels for three independently-shippable namespaces bundled into one
// file and one entry point (registerEmulatorFolderWorkspaceHostChannels) —
// see specs/backend-go/bugs/missing-v1/tasks/TASK-046, TASK-061..067,
// TASK-068 for each namespace's own design doc. Kept out of channels.go
// deliberately: this pass's integration instructions route every new
// channel registration through its own file so parallel passes touching
// channels.go don't collide — see this repo's active-worktree task brief.
//
// Wiring note for whoever merges this into RegisterRealChannels
// (channels.go): add a `projectClient projectv1.ProjectServiceClient`
// parameter to RegisterRealChannels (already dialed as `projectClient` in
// api-gateway/cmd/server/main.go for the /v1/projects REST routes — reuse
// it, don't dial a second client) and call
// `registerEmulatorFolderWorkspaceHostChannels(r, projectClient)` from
// inside it, alongside the other register*Channels calls.
package wscompat

import (
	"context"
	"encoding/json"
	"errors"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// registerEmulatorFolderWorkspaceHostChannels wires every channel this
// file gives real backend-go implementations to: emulator.* (honest
// permanent-unsupported stub, TASK-046), host.* (honest local-answer
// stub, TASK-068), and folderWorkspace.* (real project-service CRUD,
// TASK-066).
func registerEmulatorFolderWorkspaceHostChannels(r *Registry, projectClient projectv1.ProjectServiceClient) {
	registerEmulatorChannels(r)
	registerHostChannels(r)
	registerFolderWorkspaceChannels(r, projectClient)
}

// ── emulator.* ──────────────────────────────────────────────────────────
//
// Mobile emulator/simulator control (ADB/xcrun simctl device driving) has
// no backend-go implementation and, per
// 02-microservices-decomposition.md's "What's deliberately not a separate
// service" section, is explicitly excluded from the Go server deployment
// by design — not a gap awaiting a future pass. The architecturally sound
// alternative (relay to the Dev Server Agent) requires a new agent/
// capability that does not exist today; agent/ changes are out of scope
// for this rewrite. See specs/backend-go/bugs/missing-v1/tasks/TASK-048
// for the blocked, documented-only relay design. Every emulator.* channel
// below returns this same typed, permanent answer instead of falling
// through to notImplementedHandler's generic "not yet" wording, which
// would incorrectly imply this is only temporarily missing.
var errEmulatorNotSupported = errors.New(
	"mobile emulator control is not supported by the Go backend — " +
		"see specs/backend-go/bugs/missing-v1/solutions/SOL-008-emulator-channels.md")

func registerEmulatorChannels(r *Registry) {
	for _, channel := range []string{
		"emulator.attach", "emulator.availability", "emulator.button",
		"emulator.gesture", "emulator.listDevices", "emulator.rotate",
		"emulator.shutdown", "emulator.tap",
	} {
		r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
			return nil, errEmulatorNotSupported
		})
	}
}

// ── host.* ──────────────────────────────────────────────────────────────
//
// WSL/PowerShell/git-bash availability on the *backend-go host itself* —
// per BUG-011, the old backend probed only its own process host, never a
// per-target dev server. backend-go's own host is a Linux container
// (10-deployment-infrastructure.md's deployment model) with none of these
// three tools meaningful on it, so "false"/"[]" is the honest answer here,
// not a placeholder — same posture as preflight.check's honest gh/glab
// false answers (channels.go's registerPreflightChannels). Per-target
// (does the CALLER'S ACTIVE DEV SERVER have these) is a distinct, more
// useful question — see specs/backend-go/bugs/missing-v1/tasks/TASK-070
// for that design, which is blocked on an agent/ capability that doesn't
// exist yet.
func registerHostChannels(r *Registry) {
	r.Register("host.wsl.isAvailable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"available": false}, nil
	})
	r.Register("host.wsl.listDistros", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return []string{}, nil
	})
	r.Register("host.pwsh.isAvailable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"available": false}, nil
	})
	r.Register("host.gitBash.isAvailable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"available": false}, nil
	})
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
			DevServerID string `json:"devServerId"`
			Path        string `json:"path"`
			Name        string `json:"name"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.CreateFolderWorkspace(ctx, &projectv1.CreateFolderWorkspaceRequest{
			DevServerId: in.DevServerID, Path: in.Path, Name: in.Name,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetFolderWorkspace(), nil
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
		return resp.GetFolderWorkspace(), nil
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
		return resp.GetFolderWorkspaces(), nil
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
		return resp, nil
	})
}
