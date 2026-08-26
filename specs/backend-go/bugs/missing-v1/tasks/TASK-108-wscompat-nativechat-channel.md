# TASK-108: Wire `nativeChat.readSession` via `infra-fleet-service.Relay`

**From Solution:** SOL-017
**Priority:** P2
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_nativechat.go` (new), `channels.go`, `cmd/server/main.go`
**Depends on:** none
**Status:** `[partial]` — implemented as a standalone file registering into `channels_issuetracking_orchestration.go` (per cross-group convention, `channels.go` untouched). Worktree `agent-a412325f0d1276bb5`, committed as `c29ca9e6a`. **Integration note:** needs `registerIssueTrackingOrchestrationChannels(r, issueTrackingClient, orchestrationClient, infraFleetClient)` added to `RegisterRealChannels`/`main.go` — all 3 clients already dialed there.

---

## Context

SOL-017 resolves BUG-017's open question with "relay, no new service
needed": `infra-fleet-service`'s existing generic `Relay` RPC is exactly
`git-gateway-service`'s `RelayExecutor` pattern
(`relay_executor.go`) applied to one more method name,
`nativeChat.readSession`, instead of `git.status`/`git.diff`. No proto
change, no new usecase layer — `wscompat`'s handler calls
`infraFleetClient.Relay` directly, mirroring `devServer.*`/`fleet.*`'s
existing use of the same already-threaded `infraFleetClient`.

**Real gap this task must not paper over:** the frontend's current
`nativeChat.readSession(agent, sessionId, limit, transcriptPath)` call
carries no `connectionId` — every other relayed channel resolves "which
host" from a domain id already in its args (`git.status`'s `worktreeId`) or
needs none (tenant-scoped, not connection-scoped). This handler fails
closed with a clear error when `connectionId` is absent rather than
silently reading `api-gateway`'s own filesystem — the exact bug BUG-017
reports. A frontend companion change (adding `connectionId` to
`NativeChatReadSessionArgs`) is required before this channel does anything
useful in practice; this task ships the backend side regardless, per
SOL-017's own scoping.

## Changes to make

### New file `services/api-gateway/internal/adapter/wscompat/channels_nativechat.go`

```go
// registerNativeChatChannels relays nativeChat.readSession straight to the
// Dev Server Agent via infra-fleet-service's generic Relay RPC — mirrors
// git-gateway-service's RelayExecutor (relay_executor.go), the established
// pattern for "read something that lives on the user's dev server, not
// wherever this backend process runs." No owning gRPC service exists for
// this one-method namespace (00-service-catalog.md), and none is
// warranted — see SOL-017's "no new service" rationale.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerNativeChatChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("nativeChat.readSession", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type readSessionArgs struct {
			Agent          string `json:"agent"`
			SessionID      string `json:"sessionId"`
			Limit          int    `json:"limit,omitempty"`
			TranscriptPath string `json:"transcriptPath,omitempty"`
			ConnectionID   string `json:"connectionId,omitempty"` // see companion-frontend-change note above
		}
		in, err := decodeArg[readSessionArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.ConnectionID == "" {
			// No relay target — fail closed with a clear message rather than
			// silently reading api-gateway's own filesystem (the exact bug
			// BUG-017 flags). A future connectionId-carrying frontend build
			// resolves this branch away entirely.
			return nil, fmt.Errorf("nativeChat.readSession: connectionId is required — this backend never reads transcript files from its own host")
		}

		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()

		paramsJSON, err := json.Marshal(map[string]any{
			"agent": in.Agent, "sessionId": in.SessionID,
			"limit": in.Limit, "transcriptPath": in.TranscriptPath,
		})
		if err != nil {
			return nil, err
		}
		resp, err := client.Relay(rpcCtx, &infrafleetv1.RelayRequest{
			ConnectionId: in.ConnectionID,
			Method:       "nativeChat.readSession",
			ParamsJson:   string(paramsJSON),
		})
		if err != nil {
			return nil, err
		}
		// result_json is passed through verbatim — the Dev Server Agent's
		// response already matches NativeChatReadSessionResult's wire shape
		// ({messages: [...]} | {error: string}), same convention
		// RelayExecutor's decode-straight-into-domain-shape calls use.
		var result json.RawMessage
		if resp.GetResultJson() != "" {
			result = json.RawMessage(resp.GetResultJson())
		}
		return result, nil
	})
}
```

Check `usecase.Identity` and `gatewaygrpc.AttachIdentity`'s exact
signatures in `services/api-gateway/internal/usecase` and
`services/api-gateway/internal/adapter/grpc` before writing this — other
`channels_*.go` files in this package call `AttachIdentity` directly with
`id.TenantID`/`id.UserID` from the `wscompat.Identity` struct; match
whatever the sibling `registerDevServerChannels`/`registerGitChannels`
call sites already do rather than assuming this exact shape if it differs.

### `channels.go` — wire `registerNativeChatChannels`

This task is independent of TASK-100/TASK-106 (SOL-015/SOL-016) — whichever
lands first in `RegisterRealChannels`, add this call alongside whatever is
already there. Against today's baseline (no `jira.*`/`linear.*` wiring
yet), the change is:

```go
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
	registerNativeChatChannels(r, infraFleetClient) // NEW — reuses the already-threaded infraFleetClient, no new param
}
```

If TASK-100/TASK-106 already landed first, `RegisterRealChannels` will
already have the extra `issueTrackingClient` parameter and
`registerJiraChannels`/`registerLinearChannels` calls — just add the
`registerNativeChatChannels(r, infraFleetClient)` line to that version
instead; no parameter list conflict either way, since this task adds no
new parameter of its own (`infraFleetClient` is already threaded through
for `registerDevServerChannels`/`registerFleetChannels`). No `main.go`
change is needed in either case.

## Agent-side dependency (out of `backend-go` scope, flagged not silently assumed)

The Dev Server Agent needs a `nativeChat.readSession` JSON-RPC handler to
answer this relay — today only the desktop Electron main process has this
logic (`desktop/src/main/native-chat/transcript-read-cache.ts`). Porting
that onto the agent is a dependency this task surfaces but does not
implement (no agent code lives in `backend-go`). Do not attempt to write
agent-side code as part of this task.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/... && go vet ./services/api-gateway/...
```
