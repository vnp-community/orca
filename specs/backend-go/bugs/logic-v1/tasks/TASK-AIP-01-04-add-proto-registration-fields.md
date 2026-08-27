# TASK-AIP-01-04: Add registration fields to `CreateAccountRequest`/`ProviderAccount` proto messages

**From Solution:** SOL-AIP-01
**Priority:** P1
**Service:** `ai-provider-service` proto
**File:** `backend-go/proto/orca/aiprovider/v1/aiprovider.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

The wire `CreateAccountRequest` only carries `tenant_id`/`type` today
(`aiprovider.proto:60-63`), and `ProviderAccount` only carries
`id/tenant_id/type/status/credential_ref/dev_server_id`
(`aiprovider.proto:47-56`). Neither has the fields `TASK-AIP-01-03` added
to the domain type. Additive-only change — `buf breaking` must stay clean.

## Changes to make

In `backend-go/proto/orca/aiprovider/v1/aiprovider.proto`, extend
`ProviderAccount`:

```protobuf
message ProviderAccount {
  string id = 1;
  string tenant_id = 2;
  ProviderType type = 3;
  string status = 4;
  string credential_ref = 5;
  string dev_server_id = 6;
  // NEW fields below — see TASK-AIP-01-03's domain additions.
  string label = 7;
  string model_hint = 8;
  string base_url = 9;
  int32 quota_limit_day = 10;
  repeated string models = 11;
  bool is_default = 12;
  string last_health_check_at = 13; // RFC3339, empty = never checked
  string created_by = 14;
}
```

Extend `CreateAccountRequest`:

```protobuf
message CreateAccountRequest {
  string tenant_id = 1;
  ProviderType type = 2;
  // NEW fields below.
  string dev_server_id = 3;
  string label = 4;
  string model_hint = 5;
  string base_url = 6;
  int32 quota_limit_day = 7;
  repeated string models = 8;
  bool is_default = 9;
}
```

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

Expected: clean build, `buf breaking` reports only additions (no removed
or renumbered fields).
