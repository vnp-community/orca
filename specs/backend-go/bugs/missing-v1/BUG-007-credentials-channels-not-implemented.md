# BUG-007: `credentials.*` channels not implemented in backend-go

**Service:** `credential-broker-service` — exists in backend-go, but per `registry.go`'s own comment "has no direct rule: reached only indirectly via infra-fleet-service's credential path — no client calls it through this gateway directly," and its proto has no shape matching `credentials.list`/`status` as this namespace needs them
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (absent), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (falls through to `notImplementedHandler`)
**Severity:** Medium — gates Web Server mode (`ORCA_MULTI_USER=1`) integration-token setup (bitbucket/azure-devops/gitea/linear/jira); a real gap, but scoped to a non-default deployment mode and a secondary integrations settings screen, not core daily app usage.
**Symptom:** All 4 `credentials.*` calls from `CredentialInputForm.tsx`/`runtime-credentials-client.ts` time out with `channel "credentials.X" is not yet implemented in backend-go — see backend-go/docs/execution-plan.md's frontend-compatibility-layer coverage table`.
**Status:** ✅ Resolved — see TASK-037–043 (7 task(s), all DONE) for implementation evidence.

---

## Description

`credentials.*` is "Web Server mode (`ORCA_MULTI_USER=1`) credential
storage (`WebCredentialStore`) — set/revoke/status/list"
(`specs/frontend/api/rpc-catalog.md:39`). It covers exactly 5 external
integration services — `bitbucket`, `azure-devops`, `gitea`, `linear`,
`jira` — identified by a `service` name, not an opaque credential id
(`frontend/src/renderer/src/runtime/runtime-credentials-client.ts:14`,
`RuntimeCredentialService` union type). GitHub/GitLab are explicitly
excluded — they use the shared `gh`/`glab` OS keychain instead.

None of the 4 methods the frontend calls are registered in
`wscompat.Registry`. Confirmed via:

```
$ grep -n '"credentials\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
(no matches)
```

`credential-broker-service` **does exist** in backend-go
(`backend-go/services/credential-broker-service/`), with a real proto
(`backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto`), gRPC
server, Postgres metadata repository, and Vault-backed `SecretStore`. But
`registry.go`'s doc comment is explicit that it is **not reachable through
api-gateway at all today**:

> credential-broker-service has no direct rule: per §7, it's "reached only
> indirectly via infra-fleet-service's credential path" — no client calls
> it through this gateway directly.
> (`backend-go/services/api-gateway/internal/domain/registry.go:79-81`)

So even for the RPCs that conceptually overlap, this is not a pure
"channel-wiring gap over an existing reachable method" the way BUG-005's
`aiProvider.create` is — it requires exposing (or proxying) a
currently-gateway-unreachable service, on top of adding the `wscompat`
handlers.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `credentials.set` | `frontend/src/renderer/src/components/settings/CredentialInputForm.tsx`, `frontend/src/renderer/src/runtime/runtime-credentials-client.ts:20-35` (`setRuntimeCredential`, RPC call at lines 31-35; params: `{ service, token, config }`) | `credential-broker-service`'s `WriteCredential` RPC exists (`credentialbroker.proto:14,89-98`) and matches the write-a-secret shape, but it's keyed by `(tenant_id, owner_id, category)` + an `encrypted_envelope` — not a plain `(service, token, config)` triple, and `owner_id`/`category` have no defined mapping from a bare integration-service name like `"jira"` today. Also blocked by the gateway-unreachability issue above. |
| `credentials.revoke` | `frontend/src/renderer/src/components/settings/CredentialInputForm.tsx`, `frontend/src/renderer/src/runtime/runtime-credentials-client.ts:38-51` (`revokeRuntimeCredential`, RPC call at lines 47-51; params: `{ service }`) | `RevokeCredentialByOwner` RPC exists (`credentialbroker.proto:49,145-151`) and is keyed by `(tenant_id, category, owner_id)`, closer to a bare-`service` lookup than `RevokeCredential`'s by-id form — but same category-mapping and gateway-unreachability gaps as `set`. |
| `credentials.status` | `frontend/src/renderer/src/runtime/runtime-credentials-client.ts:54-67` (`getRuntimeCredentialStatus`, RPC call at lines 63-67; params: `{ service }`, returns `{ configured, mode, config? }`) | `GetCredentialMetadata` RPC exists (`credentialbroker.proto:28,127-133`) but is keyed by `credential_id`, not `service`; there is no by-owner metadata-read RPC (only `ResolveCredentialByOwner`, which returns the plaintext `value`, not status metadata — a security mismatch for a status check). No RPC returns a `mode` field at all (`WebCredentialStore`'s "electron-mode stub" vs. real-mode distinction, per the frontend comment below, has no backend-go equivalent). |
| `credentials.list` | `frontend/src/renderer/src/runtime/runtime-credentials-client.ts:70-83` (`listRuntimeCredentials`, RPC call at lines 78-82; returns `{ services: string[], mode: string }`) | **No matching RPC at all.** `CredentialBrokerService` has no `List`/`ListByOwner`-shaped method anywhere in the proto (`credentialbroker.proto:13-64`) — there is no way to enumerate which of the 5 integration services currently have a stored credential for a tenant/user. Deepest gap in this namespace: needs a new RPC, not just a new `wscompat` wrapper. |

A comment in the frontend client itself
(`frontend/src/renderer/src/runtime/runtime-credentials-client.ts:4-10`)
notes: "credential management (Web Server mode / `ORCA_MULTI_USER=1`) is
backed by the same `credentials.*` RPC methods on every host — Electron mode
has no separate native handler, it just answers with `mode:'electron'`
stubs" — confirming `mode` is a first-class part of this contract that
`credential-broker-service`'s proto has no field for today.

---

## Dispatch model

Per `specs/frontend/api/backend-agent-execution-boundary.md:151`:

> `credentials.*` (`set`/`revoke`/`status`/`list`) | 🟢 backend-local, **not
> Postgres** | AES-256-GCM `.enc` files on the backend host's own
> filesystem, per-user, gated behind `ORCA_MULTI_USER=1`. Covers
> external-integration tokens only (`bitbucket`/`azure-devops`/`gitea`/
> `linear`/`jira`) — GitHub/GitLab are explicitly excluded (they use the
> shared `gh`/`glab` OS keychain instead).

The old TS backend's `WebCredentialStore` was **not** Vault-backed and not
Postgres-backed at all — it was per-user AES-256-GCM `.enc` files directly
on the backend host's filesystem, gated behind `ORCA_MULTI_USER=1`.
backend-go's `credential-broker-service` is architecturally different: it is
Vault-backed (`TransitEncrypt`/`TransitDecrypt`/`KVWrite`/`KVRead` via
`common/secrets.Client`,
`backend-go/services/credential-broker-service/internal/usecase/ports.go:59-76`),
with Postgres used for metadata only (id/tenant/owner/category/status/
vault_path — "no secret columns, ever",
`backend-go/services/credential-broker-service/internal/usecase/ports.go:14-18`),
and it enforces a caller allow-list via Vault policy for `ResolveCredential`
(`credentialbroker.proto:100-103`: "called only by services explicitly
authorized to mediate a secret on behalf of a request
(ai-provider-service, scm-integration-service, issue-tracking-service,
infra-fleet-service)").

Whoever implements this in backend-go needs to make an explicit
architecture decision, not just "return 200":

1. **Reuse `credential-broker-service`** — map the 5 integration services to
   a `CredentialCategory` (likely `CREDENTIAL_CATEGORY_SCM_OAUTH` for
   `bitbucket`/`azure-devops`/`gitea` and `CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH`
   for `linear`/`jira`, per the enum at `credentialbroker.proto:67-74`) and
   the frontend's `service` string to `owner_id`. This requires: (a) adding
   `credentials.*` to api-gateway's authorized-caller allow-list for
   `ResolveCredential`/mediating RPCs, contradicting today's "reached only
   indirectly" design note at `registry.go:79-81` unless that note is
   revised; (b) adding a new `List`-by-tenant RPC (no equivalent exists);
   (c) adding a metadata-by-owner read RPC distinct from
   `ResolveCredentialByOwner` (which returns plaintext) for the `status`
   check; (d) deciding how `mode` (`'electron'` vs. real) is represented.
2. **Keep it backend-local, matching the old TS shape** — reimplement a
   simple per-tenant/per-service encrypted-file (or Postgres-blob) store
   scoped to api-gateway or a new lightweight service, bypassing
   credential-broker-service/Vault entirely for this narrower, lower-trust
   integration-token use case.

Either way this is a real design decision, not a thin wrapper — flag this
explicitly to whoever picks up the implementation.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — confirmed no `credentials.*` registrations
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/domain/registry.go:79-81` — comment confirming credential-broker-service is not directly gateway-reachable
- `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto:13-64,67-74,89-98,127-151` — `CredentialBrokerService`'s 8 RPCs, `CredentialCategory` enum, message shapes (no `List`)
- `backend-go/services/credential-broker-service/internal/usecase/ports.go:14-18,52-76` — `CredentialMetadataRepository` (no-secret-columns rule), `SecretStore` Vault port
- `frontend/src/renderer/src/runtime/runtime-credentials-client.ts:4-10,12,20-83` — namespace doc comment, `RuntimeCredentialService` type, all 4 call sites
- `frontend/src/renderer/src/components/settings/CredentialInputForm.tsx` — `set`/`revoke` call sites
- `specs/frontend/api/backend-agent-execution-boundary.md:151` — `credentials.*` dispatch classification (backend-local `.enc` files, not Postgres, not Vault)
- `specs/frontend/api/rpc-catalog.md:39,129-136` — `credentials.*` namespace summary and catalog entries
- `specs/backend-go/bugs/missing-v1/BUG-009-files-channels-not-implemented.md` — sibling bug report this follows the format of
