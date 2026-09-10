# BUG-017: `nativeChat.*` channels not implemented in backend-go

**Service:** `api-gateway` (WS compat layer) — no owning gRPC service exists at all
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** Low
**Symptom:** `nativeChat.readSession` falls through to `registry.go`'s `notImplementedHandler` and returns an error immediately.
**Status:** ✅ Resolved — see TASK-108–109 (2 task(s), all DONE) for implementation evidence.

---

## Description

`wscompat/channels.go` registers 13 real channels total (`annotation.*`, `task.{create,get}`,
`git.{status,diff}`, `automation.runNow`, `preflight.check`, `devServer.{list,add}`,
`fleet.health.checkAll`, `crashReports.getLatestPending`, `rateLimits.get` — file header comment
at `channels.go:16-19`). None of them is a `nativeChat.*` channel:

```
grep -n '"nativeChat\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
```

returns zero matches. Unlike `jira.*`/`linear.*` (BUG-015/BUG-016), there is no partial or
generic owning service to fall back on either — a repo-wide search for `nativeChat`/`native_chat`
across `backend-go/**/*.go`, `backend-go/**/*.proto`, and `specs/backend-go/**/*.md` returns zero
hits. No proto, no usecase, no route, no registry entry, no design doc mentions this namespace at
all in backend-go.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `nativeChat.readSession` | `components/native-chat/native-chat-session-transport.ts:56-61` | No matching RPC — no owning service exists in backend-go at all. |

Note: `frontend/src/renderer/src/components/native-chat/native-chat-session-transport.ts` also
opens a `nativeChat.subscribe` streaming RPC (`native-chat-session-transport.ts:91-97`) for live
tail updates, but that method isn't in `specs/frontend/api/rpc-catalog.md`'s `callRuntimeRpc`
catalog under this task's assigned method list (only `nativeChat.readSession` is), so it's noted
here for context but not tracked as a separately assigned missing channel.

---

## Dispatch model

Old TS backend (`specs/frontend/api/backend-agent-execution-boundary.md:164`): 🏠 **backend-local**
— reads/tails local JSONL transcript files **on the backend host's own filesystem**. No Postgres,
no relay.

**Open question for whoever implements this** (flagging rather than silently porting as-is): the
old design reads transcript files directly off the *backend process's* host filesystem. That
assumption doesn't transfer cleanly to backend-go's multi-tenant deployment model, where
`api-gateway`/whatever owns this namespace runs centrally and the actual agent session transcripts
live on the user's dev server, not the backend host. Every other host-filesystem-shaped concern in
this codebase (files, terminal, git) already routes through the Dev Server Agent /
`infra-fleet-service`'s relay to reach the user's actual machine — `nativeChat.readSession` should
probably follow that same pattern (relay to the Dev Server Agent to read the transcript where it
actually lives) rather than reading from wherever the backend-go process happens to run. This
needs a design decision, not just a straight port of the old TS handler's file-read logic.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:1-19` — registered-channel inventory (no `nativeChat.*`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `frontend/src/renderer/src/components/native-chat/native-chat-session-transport.ts:40-167` — the only frontend call site, both `readSession` and the `nativeChat.subscribe` stream
- `specs/frontend/api/backend-agent-execution-boundary.md:164` — old backend's `nativeChat.*` dispatch model ("reads/tails local JSONL transcript files on the backend host filesystem")
- `specs/frontend/api/rpc-catalog.md:343` — authoritative RPC catalog (`nativeChat.readSession`)
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — original example of this bug shape
