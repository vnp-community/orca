# SOL-025: Register `status.get` as a local, no-downstream-call `wscompat` handler — resolving BUG-025's dispatch ambiguity by tracing both callers to their actual transport

**Resolves:** [BUG-025](../BUG-025-status-channels-not-implemented.md)
**Service:** `api-gateway` only — no other backend-go service involved
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Resolving the ambiguity: neither caller needs a relay — one of them doesn't even reach `wscompat`

BUG-025 correctly declines to guess between "local, `preflight.check`-style
handler" and "relay via `infra-fleet-service`" without checking each of
`status.get`'s two callers separately. Tracing both through the frontend's
actual transport selection resolves it decisively — **both real answers
turn out to be "local," and one caller is not `wscompat`'s problem at
all**:

**`browser-pane-remote.tsx`'s `status.get` call never reaches `wscompat`.**
It calls `callRuntimeRpc(target, 'status.get', ...)` with `target` bound
to `{ kind: 'environment', environmentId }` (confirmed: the same call
site reads `target.environmentId` immediately afterward,
`browser-pane-remote.tsx:906-919`). `callRuntimeRpc` branches on
`target.kind`:

```ts
// frontend/src/renderer/src/runtime/runtime-rpc-client.ts:82-90
target.kind === 'local'
  ? await window.api.runtime.call({ method, params: nextParams })
  : await window.api.runtimeEnvironments.call({ selector: target.environmentId, method, params: nextParams, timeoutMs: options.timeoutMs })
```

`window.api.runtime.call` is the one that reaches backend-go's `/ws`
`wscompat.Registry` (per `wscompat`'s own package doc, "the legacy
channel-based RPC transport the deployed frontend/ actually speaks over
/ws"). `window.api.runtimeEnvironments.call` is a **different IPC
surface** implemented only in `desktop/src/main/ipc/runtime-environments.ts`
— pairing-code-based connections to *other Orca instances* (see its
`addFromPairingCode`/`PublicKnownRuntimeEnvironment` shape in
`frontend/src/preload/api-types.ts:3079-3103`), an Electron-desktop-only
feature with no browser-mode/`api-gateway` equivalent. Per
`api-gateway.md` §10, "Electron desktop mode is out of scope" for this
migration — this caller's `status.get` traffic is out of scope for
backend-go entirely, not a channel to implement here.

**`windows-terminal-capability-read.ts`'s `status.get` call reaches
`wscompat` only via `target.kind === 'local'`, and only reads one field.**
Its `status.get` call sits inside a `Promise.all` alongside
`host.wsl.isAvailable`/`host.pwsh.isAvailable`/`host.gitBash.isAvailable`,
and the *only* thing it extracts from the response is `status.hostPlatform`
(`windows-terminal-capability-read.ts:58-60`):

```ts
callRuntimeRpc<RuntimeStatus>(target, 'status.get', undefined, { timeoutMs })
  .then((status) => status.hostPlatform ?? null)
  .catch(() => null)
```

No relay target, no `connectionId`, nothing SSH- or dev-server-shaped in
this call site — it wants to know what OS the backend process it's talking
to is running on, exactly the kind of fact `preflight.check` already
answers locally for `git`/`gh`/`glab` (`channels.go:520-528`'s doc
comment: "Registered as a fast, LOCAL (no downstream call) response").

**Concrete recommendation: implement `status.get` as a single local,
no-downstream-call handler, structurally identical to
`registerPreflightChannels`.** No relay branch, no `connectionId`
handling — the one call site that actually reaches `wscompat` never asks
for one, and the call site that might have needed one doesn't route
through `wscompat` at all.

---

## Design

`RuntimeStatus` (`frontend/src/shared/runtime-types.ts:53-68`) is a large
type shared with Electron's own multi-window runtime-graph concept
(`rendererGraphEpoch`, `authoritativeWindowId`, `liveTabCount`,
`liveLeafCount`) that has no server-mode equivalent — `api-gateway` is
stateless with no window/tab graph (`api-gateway.md` §2/§5). Report those
fields honestly as their "nothing to report" zero-values, same posture
`preflight.check` already takes for `gh`/`glab` (`channels.go:520-528`'s
doc comment: "Reporting installed:false/authenticated:false for both is
the honest answer") and `crashReports.getLatestPending` takes for `null`
(`channels.go:544-551`) — never fabricate a plausible-looking value for a
concept that doesn't exist server-side.

`RUNTIME_PROTOCOL_VERSION`/`MIN_COMPATIBLE_RUNTIME_SERVER_VERSION`
(`frontend/src/shared/protocol-version.ts:20,22`, currently `3`/`2`) only
matter for `assertRuntimeStatusCompatible`'s compat gate, which
`callRuntimeRpc` only runs for `target.kind === 'environment'`
(`runtime-rpc-client.ts:79-81`) — never for the `local` target this
handler serves. Reporting them honestly is still worthwhile for future
callers, but no test in this solution depends on a specific value; pick
whatever the frontend's own `RUNTIME_PROTOCOL_VERSION` constant is at
implementation time rather than hardcoding a number here that will drift.

```go
// ── status.get ────────────────────────────────────────────────────────
//
// Registered as a fast, LOCAL (no downstream call) response, same pattern
// as registerPreflightChannels (channels.go:520-528's doc comment) — see
// SOL-025 for why: status.get's only wscompat-reachable caller
// (windows-terminal-capability-read.ts, target.kind==='local') reads
// nothing but hostPlatform; its other nominal caller
// (browser-pane-remote.tsx) always uses target.kind==='environment' and
// never reaches this handler — that path goes through
// window.api.runtimeEnvironments.call, an Electron-desktop-only IPC
// surface out of scope for backend-go (api-gateway.md §10).
//
// runtimeId/graphStatus/authoritativeWindowId/liveTabCount/liveLeafCount
// mirror Electron's multi-window runtime-graph concept, which has no
// server-mode equivalent — reported as honest zero-values, not
// fabricated, matching preflight.check's gh/glab convention.
func registerStatusChannels(r *Registry) {
    r.Register("status.get", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
        return map[string]any{
            "runtimeId":                        "api-gateway",
            "rendererGraphEpoch":                0,
            "graphStatus":                       "n/a", // no window/tab graph server-side
            "authoritativeWindowId":             nil,
            "liveTabCount":                      0,
            "liveLeafCount":                     0,
            "runtimeProtocolVersion":            currentRuntimeProtocolVersion,
            "minCompatibleRuntimeClientVersion": minCompatibleRuntimeClientVersion,
            "capabilities":                      []string{},
            "hostPlatform":                      runtime.GOOS, // the one field windows-terminal-capability-read.ts actually reads
        }, nil
    })
}
```

`runtime.GOOS` (Go's own `runtime` package) reports the OS `api-gateway`'s
process runs on — the honest server-mode answer to "what platform is this
backend on," matching what the field is asked for. `hostPlatform` in
`RuntimeStatus` is typed as `NodeJS.Platform` on the frontend side
(`'win32' | 'darwin' | 'linux' | ...`); `runtime.GOOS` already returns
those same string values on the three targeted platforms
(`darwin`/`linux`/`windows` — note `windows`, not `win32`; the JSON value
needs the one-line translation `"windows"` → `"win32"` if strict
`NodeJS.Platform` matching is required by a caller — flag as an
implementation detail, not a design gap, since the actual call site
(`windows-terminal-capability-read.ts`) treats `hostPlatform` as a display
string, not a `switch` discriminant, per its usage at that call site.

Register alongside `registerPreflightChannels` in `RegisterRealChannels`
(`channels.go:71`):

```go
registerPreflightChannels(r)
registerStatusChannels(r) // NEW
```

No proto/usecase change, no new gRPC client — this is a pure `api-gateway`
transport-layer completeness fix, same category as SOL-002's
`/auth/sso/:provider` stub.

---

## Test plan

```go
func TestStatusGet_ReturnsHostPlatformAndHonestZeroValues(t *testing.T) {
    r := NewRegistry()
    registerStatusChannels(r)
    result, err := r.Dispatch(context.Background(), Identity{}, "status.get", nil)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    m := result.(map[string]any)
    if m["hostPlatform"] == "" {
        t.Error("want non-empty hostPlatform")
    }
    if m["liveTabCount"] != 0 || m["authoritativeWindowId"] != nil {
        t.Error("want honest zero-values for window-graph fields, not fabricated data")
    }
}
```

- `channels_test.go` — assert `status.get` is registered (no
  `notImplementedHandler` fallthrough) via the same registry-inspection
  helper other channel tests in this file already use.
- No integration/contract test needed — this handler makes no downstream
  call, so there's no cross-service contract to verify beyond the unit
  test above.

## References

- `frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx:894-919` — `status.get` call site, confirmed `target.kind === 'environment'` (reads `target.environmentId` right after)
- `frontend/src/renderer/src/runtime/runtime-rpc-client.ts:68-90` — `callRuntimeRpc`'s `target.kind` branch (`window.api.runtime.call` vs. `window.api.runtimeEnvironments.call`)
- `frontend/src/preload/api-types.ts:3043,3079-3103` — `window.api.runtime` (backend-go-reachable) vs. `window.api.runtimeEnvironments` (pairing-code peer-instance IPC) surfaces
- `desktop/src/main/ipc/runtime-environments.ts` — confirms `runtimeEnvironments.call`'s only implementation is Electron-desktop-only (no match under `frontend/src/main` or any browser-mode shim)
- `frontend/src/renderer/src/lib/windows-terminal-capability-read.ts:41-60` — `status.get`'s wscompat-reachable call site; only reads `status.hostPlatform`
- `frontend/src/shared/runtime-types.ts:53-68` — `RuntimeStatus` shape
- `frontend/src/shared/protocol-version.ts:20,22` — `RUNTIME_PROTOCOL_VERSION`/`MIN_COMPATIBLE_RUNTIME_SERVER_VERSION` current values
- `frontend/src/renderer/src/runtime/runtime-protocol-compat.ts:19-30` — `assertRuntimeStatusCompatible`, only invoked for `target.kind === 'environment'`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:508-528,538-552` — `registerPreflightChannels`/`registerCrashReportChannels`, the "local, honest placeholder" pattern this design follows
- `specs/backend-go/tdd/services/api-gateway.md` §2,§5,§10 — stateless/no-window-graph rationale for the honest zero-values; Electron-desktop-out-of-scope statement
- `specs/backend-go/bugs/missing-v1/BUG-025-status-channels-not-implemented.md` — the dispatch-ambiguity this solution resolves
