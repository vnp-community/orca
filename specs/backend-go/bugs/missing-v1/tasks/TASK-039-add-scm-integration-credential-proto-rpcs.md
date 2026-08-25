# TASK-039: Add `SetIntegrationCredential`/`GetIntegrationCredentialStatus`/`ListIntegrationCredentials` RPCs to `scmintegration.proto`

**From Solution:** SOL-007
**Priority:** P0
**Service:** `scm-integration-service`
**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`RevokeAuth` (`scmintegration.proto:26`) is **reused as-is** for
`credentials.revoke` — no new RPC needed for revoke. This task adds the 3
new RPCs `credentials.set`/`status`/`list` need. All additive.

---

## Changes to make

**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`

### Step 1 — add 3 RPCs to the `ScmIntegrationService` service block

Add after the existing `RevokeAuth` RPC:

```protobuf
  // SetIntegrationCredential/GetIntegrationCredentialStatus/
  // ListIntegrationCredentials back api-gateway's credentials.set/status/
  // list channels for bitbucket/azure-devops/gitea (SOL-007). Reuses the
  // same CREDENTIAL_CATEGORY_SCM_OAUTH / owner_id=provider-name convention
  // CompleteOAuthFlow already established — see complete_oauth_flow.go's
  // doc comment.
  rpc SetIntegrationCredential(SetIntegrationCredentialRequest) returns (SetIntegrationCredentialResponse);
  rpc GetIntegrationCredentialStatus(GetIntegrationCredentialStatusRequest) returns (GetIntegrationCredentialStatusResponse);
  rpc ListIntegrationCredentials(ListIntegrationCredentialsRequest) returns (ListIntegrationCredentialsResponse);
```

### Step 2 — append new messages to the bottom of the file

```protobuf
message SetIntegrationCredentialRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string token = 3;       // raw PAT/token, transport-encrypted at the edge same as every other credential write
  string config_json = 4; // optional, non-secret
}
message SetIntegrationCredentialResponse {}

message GetIntegrationCredentialStatusRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
}
message GetIntegrationCredentialStatusResponse {
  bool configured = 1;
  string config_json = 2; // present only when configured
}

message ListIntegrationCredentialsRequest {
  string tenant_id = 1;
}
message ListIntegrationCredentialsResponse {
  repeated ScmProvider configured_providers = 1;
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

Expected: clean build, `buf breaking` reports no breaking changes.
