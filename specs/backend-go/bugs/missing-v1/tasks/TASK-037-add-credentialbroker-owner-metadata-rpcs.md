# TASK-037: Add `GetCredentialMetadataByOwner`/`ListCredentialsByCategory` RPCs + `config_json` field to `credentialbroker.proto`

**From Solution:** SOL-007
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `credential-broker-service`
**File:** `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto`
**Depends on:** none
**Status:** `[x]` DONE — `get_credential_metadata_by_owner.go`/`GetCredentialMetadataByOwner` RPC confirmed present in proto+usecase+postgres+grpc server; migration `0002_config_json` applied.

---

## Context

BUG-007 flags two gaps in `credential-broker-service`'s RPC surface:
`credentials.status` needs a by-owner **metadata-only** read (today only
`ResolveCredentialByOwner` exists, which returns the plaintext value — a
security mismatch for a status check), and `credentials.list` needs a
"which owner_ids have a credential in this category" method (nothing
today answers that). Both new RPCs are metadata-only — no secret value in
either response, preserving the "database dump yields no usable
credential" invariant. This task also adds one additive `config_json`
field for non-secret sidecar config (e.g. a self-hosted Gitea/Jira base
URL) that `credentials.set`'s `{ service, token, config }` shape has
nowhere to live today.

---

## Changes to make

**File:** `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto`

### Step 1 — add 2 RPCs to the `CredentialBrokerService` service block

Add after the existing `SignVapidPayload` RPC:

```protobuf
  // GetCredentialMetadataByOwner is GetCredentialMetadata's lookup-by-owner
  // counterpart — for callers that know (tenant_id, category, owner_id) but
  // were never handed an opaque credential_id, mirroring
  // ResolveCredentialByOwner's existing by-owner convention but for a
  // metadata-only read instead of a plaintext resolve.
  rpc GetCredentialMetadataByOwner(GetCredentialMetadataByOwnerRequest) returns (GetCredentialMetadataByOwnerResponse);

  // ListCredentialsByCategory answers "which owner_ids have a credential in
  // this category for this tenant" — backs credentials.list.
  rpc ListCredentialsByCategory(ListCredentialsByCategoryRequest) returns (ListCredentialsByCategoryResponse);
```

### Step 2 — additive `config_json` field on `WriteCredentialRequest` and `CredentialMetadata`

Current `WriteCredentialRequest`:

```protobuf
message WriteCredentialRequest {
  string tenant_id = 1;
  string owner_id = 2;
  CredentialCategory category = 3;
  bytes encrypted_envelope = 4;
}
```

Replace with:

```protobuf
message WriteCredentialRequest {
  string tenant_id = 1;
  string owner_id = 2;
  CredentialCategory category = 3;
  bytes encrypted_envelope = 4;
  // config_json is non-secret sidecar configuration only (e.g. a
  // self-hosted Gitea/Jira instance base URL). NEVER put anything
  // sensitive here: unlike encrypted_envelope, this field is returned
  // verbatim by metadata-only reads (GetCredentialMetadataByOwner,
  // GetCredentialMetadata).
  string config_json = 5;
}
```

Current `CredentialMetadata`:

```protobuf
message CredentialMetadata {
  string id = 1;
  string tenant_id = 2;
  string owner_id = 3; // user id or service name
  CredentialCategory category = 4;
  string status = 5; // active|rotating|revoked
  string vault_path = 6; // reference only, never the secret itself
}
```

Replace with:

```protobuf
message CredentialMetadata {
  string id = 1;
  string tenant_id = 2;
  string owner_id = 3; // user id or service name
  CredentialCategory category = 4;
  string status = 5; // active|rotating|revoked
  string vault_path = 6; // reference only, never the secret itself
  string config_json = 7; // mirrors WriteCredentialRequest.config_json — never secret
}
```

### Step 3 — append new messages to the bottom of the file

```protobuf
message GetCredentialMetadataByOwnerRequest {
  string tenant_id = 1;
  CredentialCategory category = 2;
  string owner_id = 3;
}
message GetCredentialMetadataByOwnerResponse {
  // metadata is unset (not an error) when no credential exists yet for
  // this (tenant, category, owner) — the normal "not configured" case
  // credentials.status must distinguish from a real error.
  optional CredentialMetadata metadata = 1;
}

message ListCredentialsByCategoryRequest {
  string tenant_id = 1;
  CredentialCategory category = 2;
}
message ListCredentialsByCategoryResponse {
  repeated CredentialMetadata credentials = 1;
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
additions — `config_json` is a new field number on both messages, not a
renumbering).
