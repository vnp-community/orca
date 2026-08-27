# SOL-005: Close the gap between `ai-provider-service.md`'s already-specified RPC surface and the actual proto

**Resolves:** [BUG-005](../BUG-005-aiprovider-channels-not-implemented.md)
**Service:** `ai-provider-service` (proto + usecase additions) + `infra-fleet-service` (one additive proto field, reused unmodified `Relay` RPC) + `api-gateway` (new `wscompat` channels)
**Affected files (proposed):**
- `backend-go/proto/orca/aiprovider/v1/aiprovider.proto`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (one additive field)
- `backend-go/services/ai-provider-service/internal/domain/provider_account.go` (add `DevServerID`)
- `backend-go/services/ai-provider-service/internal/usecase/{list_accounts,update_account,delete_account,write_credential,test_connection}.go` (new)
- `backend-go/services/ai-provider-service/internal/usecase/ports.go` (repository + new `InfraFleetClient` port)
- `backend-go/services/ai-provider-service/internal/adapter/postgres/*.go` (repository additions)
- `backend-go/services/ai-provider-service/internal/adapter/grpcclient/infrafleet_client.go` (new)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go` (new)
**Status:** 📋 Proposed — not yet implemented

---

## The design already exists — this is a gap-closing task, not a new design

`specs/backend-go/tdd/services/ai-provider-service.md` §3's gRPC sketch
already specifies exactly the surface this bug needs:

```protobuf
service AIProviderService {
  rpc CreateAccount(CreateAccountRequest) returns (ProviderAccount);
  rpc GetAccount(GetAccountRequest) returns (ProviderAccount);
  rpc ListAccounts(ListAccountsRequest) returns (ListAccountsResponse);
  rpc UpdateAccount(UpdateAccountRequest) returns (ProviderAccount);
  rpc DeleteAccount(DeleteAccountRequest) returns (google.protobuf.Empty);
  rpc WriteCredential(WriteCredentialRequest) returns (WriteCredentialResponse);
  rpc RotateKey(RotateKeyRequest) returns (RotateKeyResponse);
  rpc TestConnection(TestConnectionRequest) returns (TestConnectionResponse);
  rpc GetUsageToday(GetUsageTodayRequest) returns (QuotaState);
  rpc Resolve(ResolveRequest) returns (ProviderAccount);
}
```

The actual proto (`backend-go/proto/orca/aiprovider/v1/aiprovider.proto:10-15`)
only has `CreateAccount`/`ResolveProvider`/`RotateKey`/`GetUsageToday` — a
subset that matches an early implementation pass, not a deliberate scope
cut. This solution adds the missing 5 RPCs (`List`→`ListAccounts` naming
aligned with the TDD, `Update`, `Delete`, standalone `WriteCredential`,
`TestConnection`), all additive per `08-inter-service-communication.md`'s
`buf breaking` rule — no existing RPC signature changes.

The `ai_provider.accounts` schema in `ai-provider-service.md` §5 also
already specifies a `dev_server_id` column the actual `ProviderAccount`
domain type and proto message don't carry yet
(`backend-go/services/ai-provider-service/internal/domain/*.go`'s
`ProviderAccount` struct has `ID`/`TenantID`/`Type`/`Status`/`CredentialRef`/
`Scope`/`UserID`/`ProjectID` — no `DevServerID`). `TestConnection` needs this
field to know which dev server to relay to (see below), so this solution
also closes that specific implementation-vs-schema drift, not just the
RPC-count gap.

---

## Missing-channel breakdown, grouped by shared pattern

| Group | Methods | Gap type |
|---|---|---|
| **A — channel-wiring only** | `aiProvider.create` | RPC exists (`CreateAccount`), REST-wired; needs only a `wscompat` handler |
| **B — metadata CRUD, no broker involvement** | `aiProvider.list`, `aiProvider.update`, `aiProvider.delete` | New RPC + usecase + repo method each, but all Postgres-only, same shape as `CreateAccount`'s non-credential half |
| **C — broker-mediated mutation** | `aiProvider.writeCredential` | New RPC; usecase extends `CreateAccount`'s existing `broker.WriteCredential` call to an *existing* account (repo already supports this via `UpdateStatus`'s `CredentialRef` field) |
| **D — new design: agent-relay via infra-fleet-service** | `aiProvider.testConnection` | No existing concept anywhere in backend-go; genuinely new design, see below |

---

## Design — Group B: metadata CRUD (`list`/`update`/`delete`)

One representative sketch (`ListAccounts`) plus a signature table for the
other two — all three follow the identical usecase→repo→proto shape.

```protobuf
// aiprovider.proto additions
rpc ListAccounts(ListAccountsRequest) returns (ListAccountsResponse);
rpc UpdateAccount(UpdateAccountRequest) returns (UpdateAccountResponse);
rpc DeleteAccount(DeleteAccountRequest) returns (google.protobuf.Empty);

message ListAccountsRequest {
  string tenant_id = 1;    // ignored server-side, taken from ctx metadata — matches ResolveProviderRequest's convention
  string dev_server_id = 2; // optional filter — matches the frontend's { devServerId } param (BUG-005)
}
message ListAccountsResponse {
  repeated ProviderAccount accounts = 1;
}

message UpdateAccountRequest {
  string account_id = 1;
  string label = 2;
  string model_hint = 3;
  string base_url = 4;
}
message UpdateAccountResponse {
  ProviderAccount account = 1;
}

message DeleteAccountRequest {
  string account_id = 1;
}
```

```go
// usecase/list_accounts.go
type ListAccounts struct {
	repo ProviderAccountRepository // .List already exists — ports.go:52
}

func (uc *ListAccounts) Execute(ctx context.Context, in ListAccountsInput) ([]domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	// ListAccountsFilter already exists (ports.go:24-28) — this usecase is a
	// thin translation layer, the repo method it needs was already there.
	return uc.repo.List(ctx, ListAccountsFilter{TenantID: tenantID})
}
```

| RPC | Repo method needed | Notes |
|---|---|---|
| `UpdateAccount` | `Update(ctx, tenantID, accountID, fields UpdateFields) (domain.ProviderAccount, error)` — **new**, distinct from `UpdateStatus` (`ports.go:53`), which only mutates lifecycle fields per its own doc comment | `label`/`model_hint`/`base_url` are the only fields `ai-provider-service.md`'s domain model documents as user-editable outside the status machine |
| `DeleteAccount` | `Delete(ctx, tenantID, accountID) error` — **new**, no equivalent exists on `ProviderAccountRepository` today (`ports.go:49-54`) | Soft-delete via `status='revoked'` + a `deleted_at` column, not a hard `DELETE`, to preserve `usage_daily`'s FK and the account's row in `ai-provider-service.md`'s health-check/audit trail — mirrors `credential-broker-service`'s revoke-not-erase convention (§9 of that doc) |

---

## Design — Group C: `writeCredential` on an existing account

`CreateAccount`'s usecase already demonstrates the exact mechanism this
needs — it just needs to run against an account that already exists instead
of one being created:

```protobuf
rpc WriteCredential(WriteCredentialRequest) returns (WriteCredentialResponse);

message WriteCredentialRequest {
  string account_id = 1;
  bytes encrypted_blob = 2; // client-side-encrypted, forwarded unopened — ADR-008
  bytes iv = 3;
}
message WriteCredentialResponse {
  ProviderAccount account = 1;
}
```

```go
// usecase/write_credential.go
func (uc *WriteCredential) Execute(ctx context.Context, in WriteCredentialInput) (domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	account, err := uc.repo.Get(ctx, tenantID, in.AccountID)
	if err != nil {
		return domain.ProviderAccount{}, err
	}
	// Same broker call CreateAccount.Execute already makes
	// (create_account.go:77) — the owner_id derivation is identical, just
	// against an existing account's Scope/UserID/ProjectID instead of the
	// create-time input.
	ownerID := account.UserID
	if ownerID == "" {
		ownerID = account.ProjectID
	}
	if ownerID == "" {
		ownerID = "ai-provider-service"
	}
	ref, err := uc.broker.WriteCredential(ctx, WriteCredentialInput{TenantID: tenantID, OwnerID: ownerID, EncryptedBlob: in.EncryptedBlob})
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_CREDENTIAL_WRITE_FAILED", "failed to write credential via credential-broker-service", err)
	}
	// UpdateStatusInput.CredentialRef already exists for exactly this —
	// ports.go:36-42's doc comment: "a rotation never leaves the row in a
	// state where Status says 'rotating' but CredentialRef still points at
	// the old secret" — the same invariant applies here, on first write.
	return uc.repo.UpdateStatus(ctx, UpdateStatusInput{
		TenantID: tenantID, AccountID: in.AccountID,
		Status: domain.AccountStatusPending, CredentialRef: ref.ID,
	})
}
```

No new repository method needed — `UpdateStatus` (`ports.go:53`) already
accepts a `CredentialRef`. This is the smallest-diff RPC in this whole
solution because the plumbing was already built for `CreateAccount` and
just needed a second entry point.

---

## Design — Group D: `testConnection` (genuinely new design)

BUG-005 leaves this open explicitly: "does it call
`credential-broker-service.ResolveCredential` and then live-test the key
itself, or does it still need a relay to a remote host?" The answer is
determined by `credential-broker-service.md` §3's own per-category table:
for `AI_PROVIDER_KEY`, `ResolveCredential` "Returns Metadata only
(`vault_path`/`credential_ref`) — **never plaintext**... Execution plane
(Dev Server Agent) decrypts locally via its own Vault Transit access, per
`ai-provider-service.md` §9." So `ai-provider-service` **cannot** get a
usable key back from `ResolveCredential` for this category — the broker
route is architecturally closed off by design, not just unimplemented.
`TestConnection` must relay to whichever dev server already holds this
account's pushed ciphertext (`ai-provider-service.md` §9's `PushCiphertext`
flow), the same execution-plane-decrypts-locally pattern spawn-time
`Resolve` already relies on.

`ai-provider-service` itself must not gain its own Dev Server Agent
adapter — per §6 of its own doc, "This service has no `adapter/vault/`
package at all... only `credential-broker-service` talks to Vault directly,"
and by the same reasoning it should not become a third service (after
`infra-fleet-service`/`git-gateway-service`) with its own
`adapter/devserveragent/` package, duplicating wire-protocol client code
`03-clean-architecture-guidelines.md`'s "cross-service shared code policy"
already argues against sharing casually. Instead, reuse
`infra-fleet-service`'s **existing generic `Relay` RPC**
(`backend-go/proto/orca/infrafleet/v1/infrafleet.proto:103-116`,
`backend-go/services/infra-fleet-service/internal/usecase/relay.go`) exactly
the way SOL-004 does for `accounts.*` — `ai-provider-service` becomes a
plain gRPC client of `infra-fleet-service`, never touching the agent
protocol itself. This keeps the "only `infra-fleet-service`/
`git-gateway-service` talk to the Dev Server Agent" boundary intact:
`infra-fleet-service` relays on `ai-provider-service`'s behalf, it doesn't
grant `ai-provider-service` agent access.

`Relay` takes a `connection_id`, but `ai-provider-service`'s accounts are
keyed by `dev_server_id` (once the domain-model gap above is closed), not a
`connectionId` — those are different identifiers in `infra-fleet-service`'s
own domain model (`infra-fleet-service.md` §4: a `connectionId` is a live
*session* against a `DevServer`, resolved fresh per connect). `ResolveConnection`
only resolves the other direction (`connectionId` → `DevServer`). Propose
one additive field to close this, reusable by both this flow and
`ai-provider-service.md` §9's `PushCiphertext` step (which needs the exact
same `dev_server_id` → active `connection_id` lookup, per that doc's
sequence diagram: `Fleet-->>AIProv: dev_server_id, connection info`):

```protobuf
// infrafleet.proto — additive field on the existing request message
message ResolveConnectionRequest {
  string connection_id = 1;
  string dev_server_id = 2; // NEW — alternate lookup key: resolve the
                             // current active connectionId for a dev
                             // server, instead of a connectionId for its
                             // dev server. Exactly one of the two fields
                             // set. Shared prerequisite for
                             // TestConnection (this doc) and
                             // ai-provider-service.md §9's PushCiphertext.
}
```

```go
// ai-provider-service/internal/usecase/test_connection.go
type TestConnection struct {
	repo  ProviderAccountRepository
	infra InfraFleetClient // new port — grpc client to infra-fleet-service.Relay
}

func (uc *TestConnection) Execute(ctx context.Context, in TestConnectionInput) (ConnectionTestResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ConnectionTestResult{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	account, err := uc.repo.Get(ctx, tenantID, in.AccountID)
	if err != nil {
		return ConnectionTestResult{}, err
	}
	// Relays a new agent-side JSON-RPC method — see SOL-004's "Agent-side
	// companion work" for the same out-of-scope-for-backend-go flag; this
	// method is new agent surface (decrypt ciphertext already pushed via
	// §9, then make one live, lightweight provider API call), not existing
	// agent behavior.
	result, err := uc.infra.Relay(ctx, account.DevServerID, "ai.testProviderConnection", map[string]any{
		"credentialRef": account.CredentialRef,
		"providerType":  account.Type.String(),
	})
	if err != nil {
		return ConnectionTestResult{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_TEST_CONNECTION_FAILED", "failed to relay connection test to dev server agent", err)
	}
	return parseConnectionTestResult(result), nil
}
```

`InfraFleetClient.Relay` (a new, narrow port in `ai-provider-service`'s own
`usecase/ports.go`, implemented in a new `adapter/grpcclient/` package —
not to be confused with `infra-fleet-service`'s own `Relay` RPC on the other
side of this call) takes a `dev_server_id`, resolves it to a `connection_id`
via the additive `ResolveConnectionRequest.dev_server_id` field above, then
calls `InfraFleetServiceClient.Relay` exactly as SOL-004's `wscompat`
handlers do. The plaintext key never crosses into `ai-provider-service`'s
memory at any point — the agent decrypts locally and returns only a
success/failure result, mirroring `ai-provider-service.md` §9's spawn-time
sequence diagram's "no secret crosses backend" property.

---

## Design — `wscompat` channel wiring (all 6 methods)

```go
// channels_ai_provider.go
func registerAIProviderChannels(r *Registry, client aiproviderv1.AiProviderServiceClient) {
	r.Register("aiProvider.create", handleAIProviderCreate(client))
	r.Register("aiProvider.list", handleAIProviderList(client))
	r.Register("aiProvider.update", handleAIProviderUpdate(client))
	r.Register("aiProvider.delete", handleAIProviderDelete(client))
	r.Register("aiProvider.writeCredential", handleAIProviderWriteCredential(client))
	r.Register("aiProvider.testConnection", handleAIProviderTestConnection(client))
}

// handleAIProviderList is representative of all 6 — decode args, attach
// identity (ai-provider-service's ResolveProvider/CreateAccount usecases
// already require tenant via ctx, per tenant.RequireTenantID — same
// AttachIdentity requirement as devServer.*/fleet.* in SOL-004), call,
// return the proto response verbatim.
func handleAIProviderList(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			DevServerID string `json:"devServerId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListAccounts(rpcCtx, &aiproviderv1.ListAccountsRequest{DevServerId: in.DevServerID})
		if err != nil {
			return nil, err
		}
		return resp.GetAccounts(), nil
	}
}
```

| Channel | RPC called | Notes |
|---|---|---|
| `aiProvider.create` | `CreateAccount` (existing) | Pure channel-wiring gap — mirrors `handleCreateAccount` in `ai_provider_routes.go:43-64` verbatim, no new proto |
| `aiProvider.list` | `ListAccounts` (new, above) | Shown in full above |
| `aiProvider.update` | `UpdateAccount` (new) | `{ accountId, label, modelHint, baseUrl }` → proto fields 1:1 |
| `aiProvider.delete` | `DeleteAccount` (new) | `{ accountId }` → `{ ok: true }` on success, matching `annotation.delete`'s response shape (`channels.go:162-174`) |
| `aiProvider.writeCredential` | `WriteCredential` (new) | `{ accountId, encryptedBlob, iv }` — `encryptedBlob`/`iv` are base64 in the JSON envelope, decode before setting `bytes` proto fields |
| `aiProvider.testConnection` | `TestConnection` (new) | `{ accountId, traceId }` — `traceId` threads into the outbound `Relay` call's request-id metadata for tracing correlation, not a proto field |

---

## Test plan

- `ai-provider-service/internal/usecase/list_accounts_test.go` — against an
  in-memory fake repo, filter by `dev_server_id`, empty tenant → error.
- `update_account_test.go` — updates `label`/`model_hint`/`base_url` only;
  asserts `status`/`credential_ref` are untouched (guards against
  `UpdateAccount` accidentally becoming a second path to mutate lifecycle
  state that should only go through `UpdateStatus`).
- `delete_account_test.go` — soft-delete sets `status='revoked'`, row still
  readable by `Get` (for audit), excluded from `List`'s default filter.
- `write_credential_test.go` — fake `CredentialBrokerClient`, assert
  `owner_id` derivation matches `CreateAccount`'s (same 3-branch fallback),
  assert resulting `Status` is `pending`, not `active` (mirrors §9's "not
  active until push-confirmed" rule).
- `test_connection_test.go` — fake `InfraFleetClient.Relay`; assert the
  plaintext credential is never constructed anywhere in this usecase's code
  path (a `go vet`-style grep-based guard test, following the same
  "no field capable of holding a secret" discipline
  `credential-broker-service.md` §9 documents for its own domain types).
- `infra-fleet-service`: a `ResolveConnectionRequest.dev_server_id` test —
  resolving by dev-server-id returns the same `ConnectionId` a
  by-connection-id resolve of the same live connection would.
- `services/api-gateway/internal/adapter/wscompat/channels_ai_provider_test.go` —
  one test per channel, fake gRPC client, following `channels.go`'s existing
  test conventions (see `channels_test.go`'s `TestRPCTimeoutConstant_ShorterThanInvokeTimeout`
  style).

## References

- `specs/backend-go/tdd/services/ai-provider-service.md` §3,§5,§6,§9 — the target design this solution implements; §9's sequence diagram is the direct source for `TestConnection`'s relay design
- `specs/backend-go/tdd/services/credential-broker-service.md` §3's per-category `ResolveCredential` table — the reason `TestConnection` cannot use `ResolveCredential` directly
- `specs/backend-go/tdd/architecture/06-secrets-vault-architecture.md` — "backend never decrypts" for `AI_PROVIDER_KEY`, ADR-008
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — cross-service shared code policy (why `ai-provider-service` shouldn't get its own `adapter/devserveragent/`)
- `backend-go/proto/orca/aiprovider/v1/aiprovider.proto:10-15,36-39` — current 4-RPC surface
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:65-116` — `ResolveConnection`/`Relay`, reused/extended by this solution
- `backend-go/services/ai-provider-service/internal/usecase/ports.go:24-28,36-54,109-124` — `ListAccountsFilter`, `UpdateStatusInput`, `ProviderAccountRepository`, `CredentialBrokerClient`
- `backend-go/services/ai-provider-service/internal/usecase/create_account.go:64-98` — the owner_id derivation and broker-call pattern `WriteCredential`/`TestConnection` reuse
- `backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:43-64` — `handleCreateAccount`, the pattern `aiProvider.create`'s channel mirrors
- `specs/backend-go/bugs/missing-v1/BUG-005-aiprovider-channels-not-implemented.md` — full method-by-method gap breakdown this solution builds on
- `specs/backend-go/bugs/missing-v1/solutions/SOL-004-accounts-channels.md` — the `Relay`-reuse pattern this solution's `TestConnection` design follows
