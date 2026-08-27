# TASK-AUTH-05-06: `QueryAuditLog` usecase and proto gain `action`/`actor_id`/`to` filters

**From Solution:** SOL-AUTH-05
**Priority:** P1
**Service:** `auth-service` (proto + usecase)
**File:** `backend-go/proto/orca/auth/v1/auth.proto`, `backend-go/services/auth-service/internal/usecase/query_audit_log.go`
**Depends on:** TASK-AUTH-05-03
**Status:** `[ ]` TODO

---

## Context

`AuditEntry`'s proto message and `QueryAuditLogRequest` need the new fields exposed over the wire so `api-gateway` (TASK-AUTH-05-07) can forward the extended filters and CSV export (TASK-AUTH-05-08) can read `target_type`/`ip_address`/`metadata` off each returned entry.

## Changes to make

In `backend-go/proto/orca/auth/v1/auth.proto`, change `AuditEntry` and `QueryAuditLogRequest`:

```protobuf
message AuditEntry {
  string id = 1;
  string tenant_id = 2;
  string actor_id = 3;
  string action = 4;
  string target_type = 5;
  string target_id = 6;
  string metadata_json = 7;  // JSON-serialized map[string]any
  string ip_address = 8;
  google.protobuf.Timestamp occurred_at = 9;
}

message QueryAuditLogRequest {
  string tenant_id = 1;
  google.protobuf.Timestamp since = 2;
  string page_token = 3;
  int32  page_size = 4;
  google.protobuf.Timestamp to = 5;
  string action = 6;
  string actor_id = 7;
}
```

Keep any existing field numbers already assigned to `id`/`tenant_id`/`actor_id`/`action`/`occurred_at`/`since`/`page_token`/`page_size` unchanged if they differ from the numbers above — check the current `auth.proto` for the live numbering before editing, and only append new fields at the next unused number (do not renumber existing fields; that would be a breaking change).

In `backend-go/services/auth-service/internal/usecase/query_audit_log.go`, extend the usecase to build an `AuditQueryFilter` (TASK-AUTH-05-03) from its input instead of positional params, and to serialize each returned entry's `Metadata` to `metadata_json` at the gRPC boundary (or leave that serialization to the grpc adapter — match whatever pattern `toProtoUser`/`toProtoSession` already use for other entities in this codebase).

Regenerate stubs:

```bash
cd /opt/repos/orca/backend-go
buf generate proto
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./proto/... ./services/auth-service/...
buf breaking proto --against '.git#branch=main'
go test ./services/auth-service/internal/usecase/... -run TestQueryAuditLog -v
```

Expected: clean build; `buf breaking` reports no breaking changes; `QueryAuditLog` forwards `action`/`actor_id`/`to` into `AuditQueryFilter` correctly when set, and each returned `AuditEntry` carries `target_type`/`ip_address`/`metadata_json`.
