# SOL-017: `nativeChat.readSession` relays to the Dev Server Agent via `infra-fleet-service.Relay` — no new service, no new proto

**Resolves:** [BUG-017](../BUG-017-nativechat-channels-not-implemented.md)
**Service:** `api-gateway` (`wscompat` wiring only) — deliberately **not** a new owning gRPC service
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_nativechat.go` (new file)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_nativechat_test.go` (new file)
- `backend-go/services/api-gateway/cmd/server/main.go` (`registerNativeChatChannels` call — reuses the already-dialed `infraFleetClient`)
**Status:** 📋 Proposed — not yet implemented

---

## Resolving BUG-017's open question: relay, and no new service needed to do it

BUG-017 correctly flags that reading transcript files off "the backend
host" doesn't transfer to backend-go's multi-tenant model, and asks whether
this should relay to the Dev Server Agent instead — but stops short of a
concrete proposal. It does. `00-service-catalog.md` confirms no service
owns `nativeChat` today, but that doesn't mean a new microservice is
warranted for one method: `infra-fleet-service` already exposes exactly the
mechanism this needs, and two existing services already use it for the
identical "reach the user's actual dev server, not wherever this backend
process happens to run" problem.

`infrafleet.proto`'s `Relay` RPC (`infrafleet.proto:25-31,103-116`) is a
generic `connectionId` + `method` + `params_json` passthrough onto the Dev
Server Agent execution plane, built precisely so callers don't each need a
purpose-built RPC — its own doc comment names `git-gateway-service`'s
`git.*`, `workflow-service`'s `agent.exec`/`shell.exec`/`notification.send`,
and `wscompat`'s own `devServer.*`/`fleet.*` channels as existing users.
`git-gateway-service`'s `RelayExecutor`
(`backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go:63-98`)
is the concrete precedent: `GetStatus`/`GetDiff` don't read the local
filesystem at all when a connection is live — they marshal params, call
`infraFleetClient.Relay(ctx, &RelayRequest{ConnectionId, Method: "git.status",
ParamsJson})`, and unmarshal the agent's JSON result. `nativeChat.readSession`
is architecturally the same shape: "read something off a specific path on
the user's dev server, get JSON back" — `git.status` just happens to run
`git status` instead of reading a JSONL file. No new proto RPC, no new
usecase layer, no new microservice — `wscompat`'s handler calls
`infraFleetClient.Relay` directly, the same way `devServer.*`/`fleet.*`
channels already do (`channels.go:390-406`), just with a Dev-Server-Agent
method name instead of an `infra-fleet-service`-native one. This keeps the
fix proportionate to a 1-method bug, matching SOL-002's precedent of "no
proto/usecase change needed" for a small, well-bounded gap.

## The one real gap this proposal has to name: no `connectionId` in the current call args

The frontend's `nativeChat.readSession(agent, sessionId, limit,
transcriptPath)`
(`frontend/src/preload/api-types.ts:845-850`, called via
`callRuntimeRpc(target, 'nativeChat.readSession', {agent, sessionId, limit,
transcriptPath}, ...)` in
`native-chat-session-transport.ts:56-61`) carries no dev-server/connection
identifier at all — a direct consequence of the old design's assumption
that the read always happens locally (BUG-017's "Dispatch model" section).
Every other relayed `wscompat` channel resolves "which host" from a
domain id already present in its args (`git.status`'s `worktreeId`,
`channels.go:222-226`) or needs none (`devServer.list` is tenant-scoped,
not connection-scoped). `nativeChat.readSession` has neither — `agent`/
`sessionId`/`transcriptPath` identify *which transcript*, not *which
machine it lives on*.

Propose adding an optional `connectionId` field to the RPC's args
(`NativeChatReadSessionArgs`, frontend-side — flagged here as a companion
change this backend-only solution depends on, not silently assumed to
already exist, the same way SOL-001 flagged `PATCH /admin/api/users/:id`
needing a corresponding RPC beyond the table it started from). The
frontend already resolves an active `RuntimeClientTarget{kind:'environment',
environmentId}` before making this call
(`native-chat-session-transport.ts:52`) — `environmentId` is exactly the
identifier `infra-fleet-service.ResolveConnection`
(`infrafleet.proto:65-81`) already accepts as `connection_id`, so this is
threading a value the frontend already computed, not inventing a new
concept for it to track.

## Design — `wscompat` wiring (`channels_nativechat.go`)

```go
package wscompat

import (
    "context"
    "encoding/json"
    "fmt"

    infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

    gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
    "github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// registerNativeChatChannels relays nativeChat.readSession straight to the
// Dev Server Agent via infra-fleet-service's generic Relay RPC — mirrors
// git-gateway-service's RelayExecutor (relay_executor.go), the established
// pattern for "read something that lives on the user's dev server, not
// wherever this backend process runs." No owning gRPC service exists for
// this one-method namespace (00-service-catalog.md), and none is warranted
// — see SOL-017's "no new service" rationale.
func registerNativeChatChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
    r.Register("nativeChat.readSession", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type readSessionArgs struct {
            Agent          string `json:"agent"`
            SessionID      string `json:"sessionId"`
            Limit          int    `json:"limit,omitempty"`
            TranscriptPath string `json:"transcriptPath,omitempty"`
            ConnectionID   string `json:"connectionId,omitempty"` // see "companion frontend change" note
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
        // ({messages: [...]} | {error: string}), same convention as
        // RelayExecutor's decode-straight-into-domain-shape calls.
        var result json.RawMessage
        if resp.GetResultJson() != "" {
            result = json.RawMessage(resp.GetResultJson())
        }
        return result, nil
    })
}
```

`RegisterRealChannels` gains a `registerNativeChatChannels(r,
infraFleetClient)` call — `infraFleetClient` is already a
`RegisterRealChannels` parameter (`channels.go:70`, used by
`registerDevServerChannels`/`registerFleetChannels`), so no new client
threading through `main.go` is needed beyond the one new call.

## Agent-side dependency (flagged, out of `backend-go`'s scope)

The Dev Server Agent needs a `nativeChat.readSession` JSON-RPC handler to
answer this relay — today only the desktop Electron main process has this
logic (`desktop/src/main/native-chat/transcript-read-cache.ts`, reading
local JSONL transcript files with an LRU cache, windowed to `limit` most
recent messages). Porting that handler's logic onto the agent (whatever
runtime the Dev Server Agent itself runs) is a dependency this solution
surfaces but does not implement — `backend-go`'s repo has no agent code.
Flagged explicitly per `relay_executor.go`'s own precedent doc comment
("no live Dev Server Agent in this environment to confirm these JSON field
names against... reconcile this against the real agent handler"), not
silently assumed to already exist.

`nativeChat.subscribe` (the companion live-tail stream, noted in BUG-017 as
out of this task's assigned method list) would need the equivalent
treatment at some point — a streaming relay rather than a single
request/response one — but is intentionally not designed here, matching
BUG-017's own scoping.

## Test plan

- `services/api-gateway/internal/adapter/wscompat/channels_nativechat_test.go`:
  - `TestNativeChatReadSessionChannel_RelaysToInfraFleet` — fake `InfraFleetServiceClient.Relay` asserts `ConnectionId`/`Method: "nativeChat.readSession"`/`ParamsJson` match the decoded args, mirroring `channels_test.go`'s `TestDevServerListChannel_Success`.
  - `TestNativeChatReadSessionChannel_MissingConnectionID_FailsClosed` — no silent local-filesystem fallback; asserts the specific error message.
  - `TestNativeChatReadSessionChannel_PropagatesRelayError` — mirrors `TestDevServerListChannel_PropagatesError`.
  - `TestNativeChatReadSessionChannel_PassesThroughResultJSONVerbatim` — both the `{messages: [...]}` and `{error: "..."}` result shapes round-trip unmodified.

## References

- `specs/backend-go/bugs/missing-v1/BUG-017-nativechat-channels-not-implemented.md` — the open question this solution resolves
- `specs/backend-go/tdd/services/00-service-catalog.md` — confirms no service owns `nativeChat` (used to rule out "new RPC on an existing service" and choose "no new service" instead)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:25-31,65-81,103-116` — `Relay`'s generic passthrough design and `ResolveConnection`'s `connection_id` concept
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go:1-158` — the concrete precedent this solution's `wscompat` handler mirrors directly
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:16-19,70,390-406` — `RegisterRealChannels`'s existing `infraFleetClient` parameter and the `devServer.*`/`fleet.*` channels already using it
- `frontend/src/preload/api-types.ts:819-857` — `NativeChatReadSessionResult`/`NativeChatApi.readSession` wire shapes
- `frontend/src/renderer/src/components/native-chat/native-chat-session-transport.ts:40-61` — the frontend call site and its `RuntimeClientTarget{environmentId}` resolution
- `desktop/src/main/ipc/native-chat.ts:17-49`, `desktop/src/main/runtime/rpc/methods/native-chat.ts:1-97` — the existing TS read-and-window logic the Dev Server Agent's own handler would need to port
