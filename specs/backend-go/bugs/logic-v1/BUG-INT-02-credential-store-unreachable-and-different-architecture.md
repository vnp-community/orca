# BUG-INT-02: WebCredentialStore's RPC surface has no backend-go equivalent reachable from the frontend, despite a real (differently-architected) credential service existing

**Business Logic:** [BL-INT-02](../../../../docs/logic/remote-integration/BL-INT-02-credential-store.md) — WebCredentialStore (API Token Management)
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A user in Web Server mode opening Integration Settings for Bitbucket/Azure DevOps/Gitea/Linear/Jira to save an API token gets no working `credentials.set/get/delete/list` calls — every one of the 4 RPC methods the spec (and the frontend's `CredentialInputForm`) calls is unregistered in backend-go's WS-compat layer and times out.

---

## Spec summary

BL-INT-02 describes `WebCredentialStore`: per-user AES-256-GCM encrypted `.enc` files on disk (`~/.orca/users/<userId>/credentials.enc`), keyed via PBKDF2(`ORCA_CREDENTIAL_KEY`+userId), exposed through 4 RPC methods (`credentials.set/get/delete/list`) scoped to 5 HTTP-based integrations (bitbucket, azure-devops, gitea, linear, jira).

## What backend-go has

A real, but architecturally different, credential-storage service exists — Vault-backed rather than file-based — and its RPC surface has actually grown since the prior audit (BUG-007) to cover more of what this namespace needs, but it remains unreachable from any client-facing channel:

- `backend-go/services/credential-broker-service/` — real gRPC service, Postgres metadata repository (id/tenant/owner/category/status/vault_path — "no secret columns, ever"), Vault-backed `SecretStore`.
- `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto:13-64` — `CredentialBrokerService` now defines **10** RPCs, including two added since BUG-007 was filed that directly close two of its previously-identified gaps: `ListCredentialsByCategory` (`:64-66`, doc comment: "backs `credentials.list`") and `GetCredentialMetadataByOwner` (`:59-63`) — both implemented server-side (`backend-go/services/credential-broker-service/internal/adapter/grpc/server.go:165` `GetCredentialMetadataByOwner`, `:180` `ListCredentialsByCategory`).
- `WriteCredential`/`ResolveCredentialByOwner`/`RevokeCredentialByOwner` cover set/get/delete conceptually (`credentialbroker.proto:14-49`), keyed by `(tenant_id, category, owner_id)` rather than a bare `service` string.

## What's missing

- **Still zero `credentials.*` registrations in `wscompat`** — `grep -n '"credentials\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` returns no matches, so none of `credentials.set/get/delete/list` are callable by the frontend at all, regardless of what credential-broker-service can now do.
- **`credential-broker-service` is still not directly gateway-reachable.** `backend-go/services/api-gateway/internal/domain/registry.go:79-81`'s own doc comment is unchanged: "credential-broker-service has no direct rule: ... it's 'reached only indirectly via infra-fleet-service's credential path' — no client calls it through this gateway directly."
- **No `service` (bitbucket/azure-devops/gitea/linear/jira) → `(tenant_id, category, owner_id)` mapping exists anywhere** — the RPCs are shaped for a category+owner key, and nothing in backend-go translates the flat integration-service name the frontend/spec use into that key.
- **No `mode` field** (`'electron'` vs. real backing) anywhere in `credentialbroker.proto` — the frontend contract's `mode` distinction has no backend-go representation.
- **Different security/storage model entirely** — Vault Transit/KV + Postgres metadata vs. the spec's per-user AES-256-GCM `.enc` file + `ORCA_CREDENTIAL_KEY` PBKDF2 derivation. Functionally comparable (both are per-tenant/user encrypted secret storage) but not the same mechanism; nothing in backend-go reads/writes a `credentials.enc`-shaped file, and there is no `ORCA_CREDENTIAL_KEY` env var anywhere in `backend-go/` (grep confirms no matches).

## See also

- [BUG-007](../missing-v1/BUG-007-credentials-channels-not-implemented.md) — the prior, more detailed missing-v1 report on this exact gap. Its "no List RPC at all" and "no metadata-by-owner RPC" findings are now **partially stale**: `ListCredentialsByCategory` and `GetCredentialMetadataByOwner` have since been added to the proto and implemented server-side. Its core finding — `credentials.*` unregistered in `wscompat`, and `credential-broker-service` not gateway-reachable — is unchanged and still the blocking gap.

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — confirmed no `credentials.*` registrations
- `backend-go/services/api-gateway/internal/domain/registry.go:79-81` — gateway-unreachability doc comment, unchanged since BUG-007
- `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto:13-66` — `CredentialBrokerService`'s 10 RPCs, including the two new by-owner/list additions
- `backend-go/services/credential-broker-service/internal/adapter/grpc/server.go:165,180` — `GetCredentialMetadataByOwner`, `ListCredentialsByCategory` server implementations
- `docs/logic/remote-integration/BL-INT-02-credential-store.md` — spec (AES-256-GCM file store, RPC API, 5 services)
