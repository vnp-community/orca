# TASK-024: Add `ListAccounts`/`UpdateAccount`/`DeleteAccount`/`WriteCredential`/`TestConnection` RPCs to `aiprovider.proto`

**From Solution:** SOL-005
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `ai-provider-service`
**File:** `backend-go/proto/orca/aiprovider/v1/aiprovider.proto`
**Depends on:** none
**Status:** `[x]` DONE — all 6 RPCs (CreateAccount/ListAccounts/UpdateAccount/DeleteAccount/WriteCredential/TestConnection) confirmed present in `aiprovider.proto`; this task doc's Status line was never updated by the implementing pass.

---

## Context

`specs/backend-go/tdd/services/ai-provider-service.md` §3's gRPC sketch
already specifies a 10-RPC surface; the actual proto today only has
`CreateAccount`/`ResolveProvider`/`RotateKey`/`GetUsageToday`. This task
adds the 5 missing RPCs SOL-005 needs (`List`→`ListAccounts` naming aligned
with the TDD doc, `Update`, `Delete`, standalone `WriteCredential`,
`TestConnection`) — all additive, no existing RPC signature changes, so
`buf breaking` stays clean.

`ProviderAccount` also gains a `dev_server_id` field — `TestConnection`
needs it to know which dev server to relay to (TASK-028), and
`ai-provider-service.md` §5 already specifies a `dev_server_id` column the
domain model doesn't carry yet.

---

## Changes to make

**File:** `backend-go/proto/orca/aiprovider/v1/aiprovider.proto`

### Step 1 — add the 5 RPCs to the `AiProviderService` service block

Current block:

```protobuf
service AiProviderService {
  rpc CreateAccount(CreateAccountRequest) returns (CreateAccountResponse);
  rpc ResolveProvider(ResolveProviderRequest) returns (ResolveProviderResponse);
  rpc RotateKey(RotateKeyRequest) returns (RotateKeyResponse);
  rpc GetUsageToday(GetUsageTodayRequest) returns (GetUsageTodayResponse);
}
```

Replace with:

```protobuf
service AiProviderService {
  rpc CreateAccount(CreateAccountRequest) returns (CreateAccountResponse);
  rpc ResolveProvider(ResolveProviderRequest) returns (ResolveProviderResponse);
  rpc RotateKey(RotateKeyRequest) returns (RotateKeyResponse);
  rpc GetUsageToday(GetUsageTodayRequest) returns (GetUsageTodayResponse);

  // ListAccounts/UpdateAccount/DeleteAccount back aiProvider.list/update/
  // delete (SOL-005 Group B) — metadata-only CRUD, no credential-broker
  // involvement.
  rpc ListAccounts(ListAccountsRequest) returns (ListAccountsResponse);
  rpc UpdateAccount(UpdateAccountRequest) returns (UpdateAccountResponse);
  rpc DeleteAccount(DeleteAccountRequest) returns (google.protobuf.Empty);

  // WriteCredential backs aiProvider.writeCredential (SOL-005 Group C) —
  // writes a credential onto an EXISTING account, distinct from
  // CreateAccount's create-time credential write.
  rpc WriteCredential(WriteCredentialRequest) returns (WriteCredentialResponse);

  // TestConnection backs aiProvider.testConnection (SOL-005 Group D) —
  // relays to the account's dev server via infra-fleet-service; see
  // TASK-028. Never returns plaintext key material.
  rpc TestConnection(TestConnectionRequest) returns (TestConnectionResponse);
}
```

Add the `google.protobuf.Empty` import at the top of the file if not
already present:

```protobuf
import "google/protobuf/empty.proto";
```

### Step 2 — add `dev_server_id` to `ProviderAccount`

Current:

```protobuf
message ProviderAccount {
  string id = 1;
  string tenant_id = 2;
  ProviderType type = 3;
  string status = 4; // active | rotating | revoked
  string credential_ref = 5; // credential-broker-service metadata id, never a secret value
}
```

Replace with:

```protobuf
message ProviderAccount {
  string id = 1;
  string tenant_id = 2;
  ProviderType type = 3;
  string status = 4; // active | rotating | revoked
  string credential_ref = 5; // credential-broker-service metadata id, never a secret value
  // dev_server_id is which dev server holds this account's pushed
  // ciphertext (ai-provider-service.md §9's PushCiphertext flow) — needed
  // by TestConnection (TASK-028) to know where to relay. Empty for
  // accounts that have never had a credential pushed to a dev server yet.
  string dev_server_id = 6;
}
```

### Step 3 — append new messages to the bottom of the file

```protobuf
message ListAccountsRequest {
  string tenant_id = 1;    // ignored server-side, taken from ctx metadata — matches ResolveProviderRequest's convention
  string dev_server_id = 2; // optional filter — matches the frontend's { devServerId } param
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

// WriteCredentialRequest/Response mirror ai-provider-service.md §3's sketch
// — encrypted_blob/iv are client-side-encrypted, forwarded unopened (ADR-008).
message WriteCredentialRequest {
  string account_id = 1;
  bytes encrypted_blob = 2;
  bytes iv = 3;
}
message WriteCredentialResponse {
  ProviderAccount account = 1;
}

message TestConnectionRequest {
  string account_id = 1;
  string trace_id = 2; // threaded into the outbound Relay call's request-id metadata for tracing correlation, not persisted
}
message TestConnectionResponse {
  bool success = 1;
  string message = 2; // human-readable result/error detail from the agent's live check, never a secret
}
```

---

## Regenerate stubs

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./proto/...
```

Expected: clean build, `buf breaking` reports no breaking changes (only
additions).
