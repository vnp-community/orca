# BUG-025: `status.*` channels not implemented in backend-go

**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels.go`
**Severity:** Low-Medium — a single method (`status.get`), called from a browser-pane remote-probe path and a Windows-terminal capability read, neither on the critical bootstrap path.
**Symptom:** `status.get` `callRuntimeRpc` calls fall through to `registry.go`'s `notImplementedHandler` and return `channel "status.get" is not yet implemented in backend-go`.
**Status:** ❌ Open

---

## Description

`grep -n '"status\.' internal/adapter/wscompat/channels.go` returns **zero matches** —
the single `status.*` method (`status.get`) is not registered.

Per `specs/frontend/api/rpc-catalog.md:61,429-433`, `status` is a "Runtime
status/capability handshake" with one method, `status.get`, called from two sites:
`renderer/src/components/browser-pane/browser-pane-remote.tsx` (probing a remote
browser pane's status) and `renderer/src/lib/windows-terminal-capability-read.ts`
(reading Windows-terminal capabilities).

The closest existing wired analog is `preflight.check`
(`registerPreflightChannels`, `channels.go:520-528`), registered as api-gateway's
own fast, local, no-downstream-call handler per `channels.go:508-513`'s doc comment.
`preflight.check`'s payload today is a fixed `{git, gh, glab}` installed/authenticated
shape (`channels.go:522-526`) — this is a **different concept** from what `status.get`'s
two call sites need (remote browser-pane liveness, Windows-terminal capability flags),
so `preflight.check` cannot simply be reused verbatim for `status.get`; at most it's a
structural template (local, api-gateway-owned handler) to follow, not a drop-in
substitute. No code in backend-go currently computes either payload shape
`status.get`'s callers expect.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `status.get` | `renderer/src/components/browser-pane/browser-pane-remote.tsx`, `renderer/src/lib/windows-terminal-capability-read.ts` | No handler registered. `preflight.check` (`channels.go:520-528`) is the closest existing pattern (local, no-downstream-call), but its `{git,gh,glab}` payload is a different concept — not a substitute for either call site's actual need. |

---

## Dispatch model

`specs/frontend/api/backend-agent-execution-boundary.md` does not break `status.get`
out as its own row — confirmed by grep, no `status\.` match in that file. The closest
documented analog is `preflight.check`'s row
(`backend-agent-execution-boundary.md:107`):

> `preflight.check` | 🔀 | Relays via `ctx.devServerManager.getRelay(devServerId)` —
> the WS relay, same transport as git/files.

Given one of `status.get`'s two callers (`browser-pane-remote.tsx`) is explicitly
probing a *remote* browser pane's status, it plausibly follows the same 🔀
dynamic-dispatch pattern as `preflight.check` — relay to the Dev Server Agent when a
`connectionId`/dev-server target is involved. This is stated as inference from the
`preflight.check` row, not a direct quote — `status.get` itself is not independently
documented in the execution-boundary doc, and its second caller
(`windows-terminal-capability-read.ts`) looks like a local capability read with no
obvious relay need. Implementers should not assume both call sites share one dispatch
model without checking each separately.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:520-528` — `registerPreflightChannels` (`preflight.check`), closest existing pattern
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `specs/frontend/api/rpc-catalog.md:61,429-433` — `status.*` catalog entry and method table
- `specs/frontend/api/backend-agent-execution-boundary.md:107` — `preflight.check` dispatch row (closest documented analog; `status.get` not separately broken out)
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling report this follows the shape of
