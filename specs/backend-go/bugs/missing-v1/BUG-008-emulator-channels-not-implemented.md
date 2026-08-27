# BUG-008: `emulator.*` channels not implemented in backend-go

**Service:** none — no owning service exists in backend-go for this namespace
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (absent), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (falls through to `notImplementedHandler`)
**Severity:** Medium — the mobile emulator/simulator pane is a real feature (device preview, tap/gesture/button input, attach/shutdown lifecycle), but it's a secondary/opt-in workflow, not core to daily terminal/git/task usage.
**Symptom:** Every `emulator.*` call from the emulator pane and mobile-emulator settings times out with `channel "emulator.X" is not yet implemented in backend-go — see backend-go/docs/execution-plan.md's frontend-compatibility-layer coverage table`.
**Status:** ❌ Open

---

## Description

None of the 8 `emulator.*` methods the frontend calls are registered in
`wscompat.Registry`. Confirmed via:

```
$ grep -n '"emulator\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
(no matches)
```

Unlike `files.*`/`folderWorkspace.*`/`host.*`, this isn't just a missing
wscompat handler wired to an existing gRPC method — **no backend-go service
owns this capability at all**. A repo-wide search for emulator/ADB/simctl
logic in `backend-go/` (proto packages and service source) turns up nothing;
`backend-go/proto/orca/` has no `emulator` package, and none of the 17
existing services (`ai-provider-service`, `annotation-service`, `api-gateway`,
`auth-service`, `automation-service`, `credential-broker-service`,
`git-gateway-service`, `infra-fleet-service`, `issue-tracking-service`,
`notification-service`, `orchestration-service`, `project-service`,
`scm-integration-service`, `task-service`, `tenant-service`, `usage-service`,
`workflow-service`) has emulator-related usecases. `registry.go`'s
`NewDefaultServiceRegistry()` (`backend-go/services/api-gateway/internal/domain/registry.go:82-101`)
has no `/v1/emulator` (or similar) routing rule either — confirming this is a
missing capability, not just an unwired REST route.

Every call falls through to `notImplementedHandler`
(`backend-go/services/api-gateway/internal/adapter/wscompat/registry.go`),
which returns an error immediately.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `emulator.attach` | `frontend/src/renderer/src/components/emulator-pane/use-emulator-pane-session.ts:199`, `frontend/src/renderer/src/lib/open-mobile-emulator-tab.ts:100` | Attaches the pane to a running/booting device |
| `emulator.availability` | `frontend/src/renderer/src/components/settings/MobileEmulatorSettingsPane.tsx:134` | Checks whether ADB/`xcrun simctl` tooling is present |
| `emulator.button` | `frontend/src/renderer/src/components/emulator-pane/use-emulator-pane-controls.ts:21` | Hardware button input (back/home/etc.) |
| `emulator.gesture` | `frontend/src/renderer/src/components/emulator-pane/use-emulator-pane-controls.ts:28` | Swipe/gesture input |
| `emulator.listDevices` | `frontend/src/renderer/src/components/emulator-pane/use-emulator-pane-session.ts:75` | Lists available emulators/simulators |
| `emulator.rotate` | `frontend/src/renderer/src/components/emulator-pane/use-emulator-pane-controls.ts:36` | Rotates device orientation |
| `emulator.shutdown` | `frontend/src/renderer/src/components/emulator-pane/use-emulator-pane-shutdown.ts:37`, `frontend/src/renderer/src/lib/simulator-pane-shutdown-scheduler.ts:28` | Shuts down a booted device |
| `emulator.tap` | `frontend/src/renderer/src/components/emulator-pane/use-emulator-pane-controls.ts:14` | Tap input at device coordinates |

None of these are registered anywhere in `channels.go`, confirmed by the grep
above — this is a full-namespace gap, not a partial one.

---

## Dispatch model

Per `specs/frontend/api/backend-agent-execution-boundary.md:163`, the old
TypeScript backend ran this 🏠 **backend-local**: it drives local ADB
(Android) and `xcrun simctl` (iOS) child processes **on the Orca backend
process's own host machine**. There is no Postgres involvement and no relay
to the Dev Server Agent — device control happens wherever the backend
process itself runs.

⚠️ **Architecture question for whoever picks this up**, not just "port it
as-is": in a multi-tenant `backend-go` deployment, driving mobile
emulators on the shared backend host doesn't translate cleanly — one
tenant's ADB/simctl processes would be running on infrastructure shared by
all tenants, with no per-tenant isolation and no guarantee the backend host
even has emulator tooling or GUI/simulator support installed (`xcrun simctl`
requires macOS). The Dev Server Agent already exists as a per-user/per-host
execution surface for `git.*`/`files.*`/`terminal.*` — it may be a better fit
to relay `emulator.*` there (so emulator processes run on the user's own dev
server) rather than reproducing the old backend-local design on a shared
backend-go host. This decision should be made before implementation starts,
since it changes whether this needs a new backend-go service, an extension
to `infra-fleet-service`'s Dev Server Agent relay, or something else
entirely.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — confirmed no `emulator.*` registrations
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/domain/registry.go:82-101` — `NewDefaultServiceRegistry()`, no emulator routing rule
- `specs/frontend/api/backend-agent-execution-boundary.md:163` — `emulator.*` 🏠 dispatch classification
- `specs/frontend/api/rpc-catalog.md:148-155` — `emulator.*` catalog entries
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling bug report this follows the format of
