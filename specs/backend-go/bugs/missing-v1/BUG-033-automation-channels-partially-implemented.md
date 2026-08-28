# BUG-033: `automation.*` — 5 of 6 methods unimplemented in backend-go

**Service:** `automation-service` (via `api-gateway`'s `wscompat` WS layer)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** Medium — 5/6 methods missing, but automation is a secondary feature vs. e.g. git/task/terminal.
**Symptom:** Every `automation.*` call the frontend makes other than `automation.runNow` resolves through `notImplementedHandler` and errors out (see BUG-002 for the general "unregistered channel" failure mode).
**Status:** ✅ Resolved — see TASK-217–221 (5 task(s), all DONE) for implementation evidence.

---

## Description

`wscompat.registerAutomationChannels` (`channels.go:257-275`) wires exactly
**one** of the 6 `automation.*` methods the frontend calls:
`automation.runNow`. `automation.create`, `automation.delete`,
`automation.list`, `automation.runs`, and `automation.update` all fall
through to `notImplementedHandler`.

Do not re-report `automation.runNow` as missing — it is wired for real
against `automation-service`'s `RunNow` gRPC method (`channels.go:258`).

The 5 missing methods split unevenly by whether `automation-service` has a
backing RPC:

- **2 methods have a complete, real backing RPC already implemented and
  exposed over REST — they just aren't wired into `wscompat`:**
  `automation.create` → `CreateAutomation`, `automation.runs` →
  `ListRuns`. Both are defined in `automation.proto`
  (`backend-go/proto/orca/automation/v1/automation.proto:14,16`), have
  working usecases (`internal/usecase/create_automation.go`,
  `list_runs.go`), gRPC server methods
  (`internal/adapter/grpc/server.go:39` CreateAutomation, `:66` ListRuns),
  and REST equivalents at
  `internal/adapter/httpgateway/automation_routes.go:23`
  (`POST /v1/automations/`) and `:25` (`GET /v1/automations/{id}/runs`).
  Adding these to `wscompat` is a thin wrapper, not new backend work.
- **3 methods have no backing RPC anywhere in `automation-service` —
  not in the proto, and not even at the repository layer:**
  `automation.delete`, `automation.list` (list all automations for a
  tenant — distinct from `automation.runs`, which lists runs of one
  automation), and `automation.update`. `automation.proto` defines only 4
  RPCs total: `CreateAutomation`, `RunNow`, `ListRuns`,
  `HandleExternalTrigger` (`automation.proto:13-24`) — none is a
  list-all/update/delete. Even `internal/usecase/ports.go`'s
  `AutomationRepository` interface (`ports.go:19-21`) only exposes
  `Create`/`Get` — there is no `List`, `Delete`, or `Update` method on the
  persistence port itself, so this gap goes all the way down to the
  database layer, not just the gRPC surface. Genuinely unbuilt.

---

## Already wired (do not re-report)

| Method | What it does | File:line |
|---|---|---|
| `automation.runNow` | Calls `AutomationServiceClient.RunNow` | `channels.go:258-274` |

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `automation.create` | `automation-host-client.ts` | Backing RPC exists: `CreateAutomation` (`automation.proto:14`, `server.go:39`, REST at `automation_routes.go:23`). Wrapper-only gap. |
| `automation.runs` | `automation-host-client.ts` | Backing RPC exists: `ListRuns` (`automation.proto:16`, `server.go:66`, REST at `automation_routes.go:25`). Wrapper-only gap. |
| `automation.delete` | `automation-host-client.ts` | No backing RPC. Not in `automation.proto`; no `Delete` method on `AutomationRepository` (`ports.go:19-21`) either. Genuinely unbuilt at every layer. |
| `automation.list` | `automation-host-client.ts` | No backing RPC. Not in `automation.proto`; no `List` method on `AutomationRepository` (`ports.go:19-21`). Distinct from `ListRuns` (runs of one automation, not all automations for a tenant). Genuinely unbuilt at every layer. |
| `automation.update` | `automation-host-client.ts` | No backing RPC. Not in `automation.proto`; no `Update` method on `AutomationRepository` (`ports.go:19-21`). Genuinely unbuilt at every layer. |

---

## Dispatch model

🟢 **Postgres** for CRUD/list (old TS backend's `PgAutomationStore`).

⚠️ **Context-only note for implementers** (not itself a backend-go finding,
but worth flagging since it affects how much trust to place in
`automation.runNow` even though it is "wired"): the **old** TS backend's
`automation.runNow`/scheduled dispatch had **no working execution path**
server-side at all — the dispatcher was intentionally left `undefined`, so
runs always resolved `skipped_unavailable`.

**backend-go appears to have actually fixed this.** `automation.proto`'s
service doc comment states this explicitly: *"Execution always delegates to
workflow-service.ExecuteAdHocStep (closes TS Gap 3 — automation.runNow had
no working execution path in TS)"* (`automation.proto:9-11`), and
`internal/usecase/run_now.go`'s `RunNow.Execute` does call a real
`WorkflowStepExecutor.ExecuteAdHocStep` port (`run_now.go:111`,
implemented over gRPC to `workflow-service` per `ports.go`'s
`WorkflowStepExecutor` doc comment) rather than leaving it stubbed. This
looks like a real fix, not a re-introduction of the TS gap — but it is
based on reading the usecase/port wiring, not an end-to-end runtime test.
Recommend whoever picks up this bug still do an explicit runtime check
(trigger a real `RunNow` and confirm `workflow-service` executes something)
before relying on it, rather than assuming wired == verified-working
end-to-end.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:257-275` — `registerAutomationChannels`
- `backend-go/proto/orca/automation/v1/automation.proto:9-24` — `AutomationService` (full 4-RPC surface + Gap-3 doc comment)
- `backend-go/services/automation-service/internal/adapter/grpc/server.go:39,54,66,87` — `CreateAutomation`/`RunNow`/`ListRuns`/`HandleExternalTrigger`
- `backend-go/services/automation-service/internal/usecase/create_automation.go`, `run_now.go`, `list_runs.go` — usecases
- `backend-go/services/automation-service/internal/usecase/ports.go:19-21` — `AutomationRepository` (Create/Get only, no List/Delete/Update)
- `backend-go/services/automation-service/internal/adapter/postgres/repository.go:35,47` — repository confirms only `Create`/`Get` implemented
- `backend-go/services/api-gateway/internal/adapter/httpgateway/automation_routes.go:21-27` — REST equivalents already calling the 4 RPCs
- `backend-go/services/api-gateway/internal/domain/registry.go:97` — `/v1/automations` → `automation-service`, `RouteWired`
- `specs/frontend/api/rpc-catalog.md:98-107` — full `automation.*` frontend call-site table (6 methods)
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling bug-report format this follows
