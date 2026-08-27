# BUG-AIP-01: Provider account registration is metadata-only CRUD — no name/models/default/quota fields, no test-before-save gate, no audit log

**Business Logic:** [BL-AIP-01](../../../../docs/logic/ai-providers/BL-AIP-01-register-provider-account.md) — Đăng ký AI Provider Account trên Dev Server
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** High
**Symptom:** An Admin/Lead registering a provider account cannot give it a display name, cannot restrict which models it may serve, cannot mark it as the tenant's default for a provider (so BL-AIP-02's "server-default" tier can never be populated), cannot set a daily token quota, and the save happens with no proof the credential actually works — `CreateAccount` inserts the row unconditionally, there is no server-side call that mirrors the spec's "Test Connection before save" gate.

---

## Spec summary

Admin/Lead adds an AI provider account on a Dev Server: encrypt credentials client-side, test the connection, save metadata (`name`, `scope`, `projectId`, `models`, `isDefault`, `quotaLimitPerDay`) to the Orca DB, then push the credential to the Dev Server over its already-open WebSocket via JSON-RPC `ai.credential.write` (scrypt-derived master key, AES-256-GCM, written to `~/.orca/ai-providers/<accountId>.enc`). Setting `isDefault=true` auto-demotes any prior default for that dev-server+provider pair, and the whole flow ends with `audit_log('ai_provider.registered', ...)`. Field-level validation includes name uniqueness per (devServer, provider), API-key format checks, and `quotaLimitPerDay >= 1000`.

## What backend-go has

- `CreateAccount` usecase creates a row with `TenantID`, `ProviderType`, `Scope`/`UserID`/`ProjectID`, forwarding a caller-supplied encrypted blob unopened to `credential-broker-service` and storing only the returned opaque `CredentialRef` — `backend-go/services/ai-provider-service/internal/usecase/create_account.go:47-98`. Account is created `pending`, never `active` at creation (`create_account.go:82-88`).
- `WriteCredential` usecase writes/replaces a credential on an *existing* account, same broker-forwarding pattern — `backend-go/services/ai-provider-service/internal/usecase/write_credential.go:33-79`.
- Both are reachable end-to-end: gRPC (`aiprovider.proto:11,29`), REST (`POST /v1/ai-providers/accounts` — `backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:43-64`), and WS channels `aiProvider.create` / `aiProvider.writeCredential` — `backend-go/services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go:24-30,38-58,119-148`.
- `TestConnection` usecase exists as a **standalone** RPC that relays to the dev server via `infra-fleet-service` — `backend-go/services/ai-provider-service/internal/usecase/test_connection.go:27-51` — but per its own doc comment the target agent method `ai.testProviderConnection` "doesn't exist yet" (`test_connection.go:40-42`), and nothing in `CreateAccount`/`WriteCredential` calls it before persisting — there is no server-side "reject save if test fails" gate.
- `RotateKey` demonstrates the one fully-wired lifecycle mutation (`rotate_key.go:29-`), a template the spec's `isDefault`-demotion logic could follow but doesn't exist for `is_default`.
- Domain/schema: `ProviderAccount` struct and `ai_provider.accounts` table carry only `id, tenant_id, provider_type, status, credential_ref, scope, user_id, project_id, dev_server_id, rotation_grace_until, created_at, updated_at` — `backend-go/services/ai-provider-service/internal/domain/provider_account.go:142-155`, `backend-go/services/ai-provider-service/migrations/0001_init.up.sql:12-32`. `label`/`model_hint`/`base_url` exist only as `Update`-only columns (`repository.go:132-149`), not settable at creation and not the spec's `name`/`models` shape.

## What's missing

- **No `name` field at creation**: `CreateAccountRequest` proto carries only `tenant_id`+`type` (`backend-go/proto/orca/aiprovider/v1/aiprovider.proto:59-62`); `label` is `Update`-only. No uniqueness-per-(devServer, provider) check anywhere.
- **No `models` (allowed-models list) field at all** on the domain type, schema, or proto — the spec's "models must be a subset of provider's supported models" validation and BL-AIP-02's "validate model in account.models" step have no field to validate against.
- **No `isDefault` field or default-demotion logic** — `ai_provider.accounts` has no `is_default` column; there is no `UPDATE ... SET is_default=false WHERE dev_server_id=? AND provider=? AND id!=newId` equivalent anywhere in the codebase (confirmed via grep for `is_default` — zero hits under `ai-provider-service/`). This also means BL-AIP-02's "server-default" resolution tier can never actually be populated in backend-go.
- **No `quotaLimitPerDay` field or `>= 1000` validation** — no `quota_limit_per_day` column on `ai_provider.accounts` (only the separate daily-rollup `ai_provider.usage` table exists, which has no limit column either).
- **No server-side test-before-save gate** — `CreateAccount.Execute` never calls `TestConnection`; a credential that fails a live test is still persisted as `pending`.
- **No API-key-format validation** (`sk-ant-api03-...`, `sk-...`) or endpoint-reachability validation for Ollama/vLLM — `CreateAccountInput` never inspects `EncryptedBlob` contents (correctly, per ADR-008 — it's opaque ciphertext), and no earlier client-side-equivalent format check exists server-side either.
- **No `audit_log('ai_provider.registered', ...)` call** — no audit-log package import or call anywhere in `ai-provider-service`.
- **Architecturally different credential-push topology than the spec** — the spec's design is Dev-Server-initiated WS + `AgentConnectionManager.getConnection(devServerId)` + JSON-RPC `ai.credential.write` with local scrypt/AES-256-GCM storage in `~/.orca/ai-providers/<accountId>.enc` on the Dev Server; backend-go instead centralizes credential storage in `credential-broker-service` (Vault-backed) and reaches the dev server only via `infra-fleet-service`'s `Relay` for `TestConnection` (`ports.go:162-169`). This is a deliberate redesign (see `ai-provider-service.md` and Epic B notes in the service README), not simply "missing," but it means the spec's exact JSON-RPC contract (`ai.testConnection`, `ai.credential.write`) and local-file storage format never apply in backend-go — flagging so a reviewer doesn't look for that literal contract.

## See also

- [missing-v1/BUG-005](../missing-v1/BUG-005-aiprovider-channels-not-implemented.md) — documents an earlier state where none of the 6 `aiProvider.*` WS channels were wired and several RPCs (List/Update/Delete/WriteCredential/TestConnection) didn't exist on the proto at all. **That RPC/channel-wiring gap is now closed** — `channels_ai_provider.go:24-30` registers all 6, and `aiprovider.proto:16-31` now defines all 6 RPCs. BUG-005 remains useful context for *why* the architecture (credential-broker-mediated, not direct dev-server relay) ended up this way, but should not be re-reported as "channels not implemented."

## References

- `backend-go/services/ai-provider-service/internal/usecase/create_account.go:16-27,47-98`
- `backend-go/services/ai-provider-service/internal/usecase/write_credential.go:11-79`
- `backend-go/services/ai-provider-service/internal/usecase/test_connection.go:27-65`
- `backend-go/services/ai-provider-service/internal/domain/provider_account.go:136-213`
- `backend-go/services/ai-provider-service/migrations/0001_init.up.sql:12-32`
- `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go:41-56,132-149`
- `backend-go/proto/orca/aiprovider/v1/aiprovider.proto:16-31,59-62,116-127`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go:24-30,38-58`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:26-64`
- `backend-go/services/ai-provider-service/README.md` — "Known gaps" and "Deviations from the design doc" sections explicitly list missing `Label`/`ModelHint`/`BaseURL`/`QuotaLimitDay`/`is_default`/health-check fields and no transactional outbox/audit trail
