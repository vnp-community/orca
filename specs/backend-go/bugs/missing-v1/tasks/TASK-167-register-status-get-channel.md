# TASK-167: Register `status.get` as a local, no-downstream-call `wscompat` handler

**From Solution:** SOL-025
**Priority:** P2
**Service:** `api-gateway` only — no other backend-go service involved
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`, `services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** none
**Status:** `[x]` DONE — implemented in worktree `agent-a5714e047dcaed0fc`, **committed** as `56c5fbeff`. Build/vet/test clean. Pending merge.

---

## Context

`status.get` has two nominal callers on the frontend, but only one
actually reaches `wscompat`:

- `browser-pane-remote.tsx`'s call uses `target.kind === 'environment'`,
  which routes through `window.api.runtimeEnvironments.call` — a
  pairing-code peer-instance IPC surface implemented only in Electron
  desktop mode, out of scope for `backend-go` entirely
  (`api-gateway.md` §10).
- `windows-terminal-capability-read.ts`'s call uses `target.kind ===
  'local'`, which does reach `wscompat` over `/ws`, and reads **only**
  `status.hostPlatform` from the response.

So `status.get` should be implemented as a single local, no-downstream-call
handler — structurally identical to `registerPreflightChannels` — not a
relay to any other service. `RuntimeStatus`'s window/tab-graph fields
(`rendererGraphEpoch`, `authoritativeWindowId`, `liveTabCount`,
`liveLeafCount`) have no server-mode equivalent (`api-gateway` is
stateless with no window/tab graph) — report them as honest zero-values,
the same posture `preflight.check` already takes for `gh`/`glab`.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

Add, near `registerPreflightChannels`/`registerCrashReportChannels`:

```go
// ── status.get ────────────────────────────────────────────────────────
//
// Registered as a fast, LOCAL (no downstream call) response, same pattern
// as registerPreflightChannels — see SOL-025 for why: status.get's only
// wscompat-reachable caller (windows-terminal-capability-read.ts,
// target.kind==='local') reads nothing but hostPlatform; its other
// nominal caller (browser-pane-remote.tsx) always uses
// target.kind==='environment' and never reaches this handler — that path
// goes through window.api.runtimeEnvironments.call, an Electron-desktop-only
// IPC surface out of scope for backend-go (api-gateway.md §10).
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
			"hostPlatform":                      hostPlatformString(), // the one field windows-terminal-capability-read.ts actually reads
		}, nil
	})
}

// currentRuntimeProtocolVersion/minCompatibleRuntimeClientVersion should
// match frontend/src/shared/protocol-version.ts's RUNTIME_PROTOCOL_VERSION/
// MIN_COMPATIBLE_RUNTIME_SERVER_VERSION constants at implementation time —
// check that file's current values rather than hardcoding a number here
// that will drift.
const (
	currentRuntimeProtocolVersion     = 3
	minCompatibleRuntimeClientVersion = 2
)

// hostPlatformString reports runtime.GOOS translated to match
// RuntimeStatus.hostPlatform's frontend type (NodeJS.Platform:
// 'win32' | 'darwin' | 'linux' | ...) — runtime.GOOS already matches on
// darwin/linux; only "windows" needs translating to "win32".
func hostPlatformString() string {
	if runtime.GOOS == "windows" {
		return "win32"
	}
	return runtime.GOOS
}
```

Add `"runtime"` to this file's import block.

Register alongside `registerPreflightChannels` in `RegisterRealChannels`:

```go
	registerPreflightChannels(r)
	registerStatusChannels(r) // NEW
```

No proto/usecase change, no new gRPC client — this is a pure `api-gateway`
transport-layer completeness fix.

## Verify

```go
// channels_test.go
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

Add this test to `channels_test.go`, plus a registry-inspection assertion
that `status.get` is registered (no `notImplementedHandler` fallthrough),
matching how other channel tests in this file already check registration.

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
go test ./internal/adapter/wscompat/... -run TestStatusGet -v
```

No integration/contract test needed — this handler makes no downstream
call.
