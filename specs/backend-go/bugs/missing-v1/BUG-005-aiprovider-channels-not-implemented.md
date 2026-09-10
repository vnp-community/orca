# BUG-005: `aiProvider.*` channels not implemented in backend-go

**Service:** `ai-provider-service` (proto `orca.aiprovider.v1`) — already `RouteWired` for REST at `/v1/ai-providers`, but its gRPC surface only covers 4 of the 6 methods this namespace needs, and none of the 6 has a `wscompat` WS channel
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (absent), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (falls through to `notImplementedHandler`)
**Severity:** High — this is the entire AI-provider account management surface (create/list/update/delete/test/write-credential); without it, users on a backend-go-connected environment cannot configure or rotate any Anthropic/OpenAI/etc. provider account, which is a core onboarding flow.
**Symptom:** All 6 `aiProvider.*` calls from `ProviderForm.tsx`/`useAIProviders.ts` time out with `channel "aiProvider.X" is not yet implemented in backend-go — see backend-go/docs/execution-plan.md's frontend-compatibility-layer coverage table`.
**Status:** ✅ Resolved — see TASK-024–030 (7 task(s), all DONE) for implementation evidence.

---

## Description

`aiProvider.*` is "AI provider (Anthropic/OpenAI/etc.) config CRUD,
credentials" (`specs/frontend/api/rpc-catalog.md:35`). None of the 6 methods
the frontend calls are registered in `wscompat.Registry`. Confirmed via:

```
$ grep -n '"aiProvider\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
(no matches)
```

Unlike BUG-004/BUG-006, this namespace's owning service **does exist** and
already has a wired REST path: `registry.go`'s
`NewDefaultServiceRegistry()` maps `/v1/ai-providers` to `ai-provider-service`
/ `orca.aiprovider.v1` with `Status: RouteWired`
(`backend-go/services/api-gateway/internal/domain/registry.go:93`), and
`mountAIProviderRoutes` (`backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:29-34`)
proxies 4 REST endpoints to it (`POST /accounts`, `GET /resolve`,
`POST /accounts/{id}/rotate-key`, `GET /usage-today`). So this is a mix of
two gap types, per method — see the table below.

`ai-provider-service`'s gRPC contract
(`backend-go/proto/orca/aiprovider/v1/aiprovider.proto:10-15`) only exposes:

```
rpc CreateAccount(CreateAccountRequest) returns (CreateAccountResponse);
rpc ResolveProvider(ResolveProviderRequest) returns (ResolveProviderResponse);
rpc RotateKey(RotateKeyRequest) returns (RotateKeyResponse);
rpc GetUsageToday(GetUsageTodayRequest) returns (GetUsageTodayResponse);
```

There is **no `List`, `Update`, `Delete`, `WriteCredential`, or
`TestConnection` RPC** on `AiProviderService` at all. `CreateAccountRequest`
also only carries `tenant_id`/`type`
(`backend-go/proto/orca/aiprovider/v1/aiprovider.proto:36-39`) — no
encrypted-credential field yet, even though the usecase layer already
threads one through (see below).

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `aiProvider.create` | `frontend/src/renderer/src/components/ai-provider/ProviderForm.tsx:44`, `frontend/src/renderer/src/hooks/useAIProviders.ts:95` | RPC exists: `CreateAccount` (`aiprovider.proto:11`), REST-wired at `POST /v1/ai-providers/accounts` (`ai_provider_routes.go:30`). **Channel-wiring gap only** — needs a `wscompat` handler calling the existing gRPC client. |
| `aiProvider.list` | `frontend/src/renderer/src/hooks/useAIProviders.ts:50` (params: `{ devServerId }`) | **No matching RPC.** `AiProviderService` has no `List` method; usecase-layer `ProviderAccountRepository.List` exists (`backend-go/services/ai-provider-service/internal/usecase/ports.go:52`) but is not exposed over gRPC or REST. Needs a new RPC + proto message + `wscompat` handler. |
| `aiProvider.testConnection` | `frontend/src/renderer/src/hooks/useAIProviders.ts:70` (params: `{ accountId, traceId }`) | **No matching RPC**, and per the old-backend dispatch model this bypassed Postgres entirely and relayed straight to the Dev Server Agent — no backend-go equivalent (relay-aware or otherwise) exists. Deepest gap in this namespace. |
| `aiProvider.update` | `frontend/src/renderer/src/components/ai-provider/ProviderForm.tsx:47`, `frontend/src/renderer/src/hooks/useAIProviders.ts:104` | **No matching RPC.** `AiProviderService` has no `Update`; usecase-layer `ProviderAccountRepository.UpdateStatus` exists (`ports.go:53`) but only mutates lifecycle status/credential-ref/rotation-grace fields, not general account fields — needs a new RPC. |
| `aiProvider.writeCredential` | `frontend/src/renderer/src/components/ai-provider/ProviderForm.tsx:58-61` (params: `{ accountId, encryptedBlob, iv }`) | **No standalone RPC on `AiProviderService`.** `credential-broker-service`'s `WriteCredential` RPC exists (`backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto:14`) and `ai-provider-service`'s `CreateAccount` usecase already forwards an `EncryptedBlob` to it internally (`backend-go/services/ai-provider-service/internal/usecase/create_account.go:77`), but that path is only reachable through account *creation* — there is no way to write/replace a credential on an *existing* account, and `CreateAccountRequest` itself has no blob field yet (`aiprovider.proto:36-39`; usecase comment at `create_account.go:16-19` confirms "EncryptedBlob may be nil...WriteCredential isn't wired into the proto surface yet either"). Needs a new RPC on `AiProviderService` (or a proto field addition) before a `wscompat` handler is possible. |
| `aiProvider.delete` | `frontend/src/renderer/src/hooks/useAIProviders.ts:85` (params: `{ accountId }`) | **No matching RPC.** `AiProviderService` has no `Delete`; `ProviderAccountRepository` interface (`ports.go:49-54`) has no `Delete` method either — this is missing at both the usecase and gRPC layers. |

Note: `aiProvider.rotateKey` is not in this namespace's assigned 6-method
list (it maps to the existing `RotateKey` RPC, `aiprovider.proto:13`, and is
REST-wired at `POST /v1/ai-providers/accounts/{id}/rotate-key`), so it is
excluded here per the task's method list, but is worth flagging as the one
`aiProvider.*`-family capability that already has a complete gRPC+usecase
implementation end to end — a useful template for `writeCredential`/`update`/
`delete`.

---

## Dispatch model

Per `specs/frontend/api/backend-agent-execution-boundary.md:148-150`:

> `aiProvider.list`/`get`/`update`/`delete`/`getUsageToday`/`resolve` | 🟢 |
> Metadata-only against `orca_ai_provider_accounts`/`orca_provider_usage`.
> `resolve` explicitly returns the best-matching account *without*
> credential material (per ADR-008).
>
> `aiProvider.create` | 🟢 | Inserts a `'pending'` account row; no credential
> written yet.
>
> **`aiProvider.writeCredential`, `rotateKey`, `testConnection`** | 🔌
> **relays to Dev Server Agent** | The 3 exceptions.
> `relay.call('ai.provider.writeCredential'|'testConnection', ...)` — only
> already-client-encrypted ciphertext (`encryptedBlob`/`iv`) crosses the
> wire; the backend never decrypts it (ADR-008). Postgres involvement here
> is limited to updating `status` (`'active'`/`'rotating'`) on the metadata
> row.

So `create`/`list`/`update`/`delete` should be Postgres-only metadata CRUD
against `ai-provider-service`'s own account table — `create` already matches
this shape in backend-go (`CreateAccount` inserts a pending row via
`ProviderAccountRepository.Create`,
`backend-go/services/ai-provider-service/internal/usecase/create_account.go:82-95`).
`list`/`update`/`delete` need new RPCs following the same pattern.

`writeCredential` and `testConnection` are architecturally different in
backend-go than the old TS backend: the old backend relayed both directly to
the Dev Server Agent with the backend never touching credential-broker at
all mid-flight; backend-go instead routes credential material through
`credential-broker-service` (Vault-backed KV+Transit — see
`backend-go/services/credential-broker-service/internal/usecase/ports.go:52-76`).
`ai-provider-service`'s `CreateAccount` already demonstrates this new shape
for account creation (`create_account.go:77`: `uc.broker.WriteCredential(...)`)
— whoever implements the standalone `writeCredential` RPC should extend that
same broker-mediated pattern (add a `WriteCredential` RPC to
`AiProviderService` that forwards to `CredentialBrokerClient.WriteCredential`
for an *existing* account) rather than reintroducing a raw Dev-Server-Agent
relay. `testConnection` has no equivalent backend-go concept yet at all —
this needs new design, not just new plumbing: does it call
`credential-broker-service.ResolveCredential` and then live-test the key
itself, or does it still need a relay to a remote host? That decision isn't
made anywhere in backend-go today.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — confirmed no `aiProvider.*` registrations
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/domain/registry.go:93` — `/v1/ai-providers` → `ai-provider-service`, `RouteWired`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:29-34` — `mountAIProviderRoutes`, the 4 REST endpoints actually wired
- `backend-go/proto/orca/aiprovider/v1/aiprovider.proto:10-15,36-39` — `AiProviderService`'s 4 RPCs; `CreateAccountRequest` has no credential field
- `backend-go/services/ai-provider-service/internal/usecase/create_account.go:16-19,77,82-95` — `CreateAccount` usecase, credential-forwarding comment, pending-row insert
- `backend-go/services/ai-provider-service/internal/usecase/ports.go:49-54,65-124` — `ProviderAccountRepository` (no Delete), `CredentialBrokerClient` port
- `backend-go/services/ai-provider-service/internal/usecase/rotate_key.go` — the one fully-implemented broker-mediated mutation, as a template
- `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto:14` — `credential-broker-service`'s `WriteCredential` RPC
- `frontend/src/renderer/src/components/ai-provider/ProviderForm.tsx:44,47,58-61` — `create`/`update`/`writeCredential` call sites
- `frontend/src/renderer/src/hooks/useAIProviders.ts:50,70,85,95,104` — `list`/`testConnection`/`delete`/`create`/`update` call sites
- `specs/frontend/api/backend-agent-execution-boundary.md:148-150` — `aiProvider.*` dispatch classification (Postgres CRUD vs. the 3 relay exceptions)
- `specs/frontend/api/rpc-catalog.md:35,80-89` — `aiProvider.*` namespace summary and catalog entries
- `specs/backend-go/bugs/missing-v1/BUG-009-files-channels-not-implemented.md` — sibling bug report this follows the format of
