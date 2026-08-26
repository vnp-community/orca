# TASK-001: Add admin RPCs to `auth.proto`

**From Solution:** SOL-001
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `auth-service`
**File:** `backend-go/proto/orca/auth/v1/auth.proto`
**Depends on:** none
**Status:** `[x]` DONE — all 10 RPCs and their messages are present in `auth.proto`, `go build ./proto/...` is clean, and generated stubs (`proto/gen/go/orca/auth/v1/*.pb.go`) already reflect them.

---

## Context

`SOL-001` maps `/admin/api/*`'s 12 routes onto `auth-service.md`'s
already-specified admin RPC list. None of the new RPCs exist in the proto
yet (only `CreateUser`/`ListUsers`/`UpdateUserRole`/`RevokeSession`/
`QueryAuditLog` do). This task adds them — additive only, no breaking
change, so `buf breaking` stays clean.

## Changes to make

Add to the `AuthService` service block in `auth.proto`:

```protobuf
rpc DeactivateUser(DeactivateUserRequest) returns (DeactivateUserResponse);
rpc ReactivateUser(ReactivateUserRequest) returns (ReactivateUserResponse);
rpc ListSessionsForUser(ListSessionsForUserRequest) returns (ListSessionsForUserResponse);
rpc ForceRevokeAllSessionsForUser(ForceRevokeAllSessionsForUserRequest) returns (ForceRevokeAllSessionsForUserResponse);
rpc CreateAccessPolicy(CreateAccessPolicyRequest) returns (AccessPolicy);
rpc GetAccessPolicy(GetAccessPolicyRequest) returns (AccessPolicy);
rpc ListAccessPolicies(ListAccessPoliciesRequest) returns (ListAccessPoliciesResponse);
rpc UpdateAccessPolicy(UpdateAccessPolicyRequest) returns (AccessPolicy);
rpc DeleteAccessPolicy(DeleteAccessPolicyRequest) returns (google.protobuf.Empty);
rpc GetAdminStats(GetAdminStatsRequest) returns (GetAdminStatsResponse);
```

Add messages (append to the bottom of the file):

```protobuf
message DeactivateUserRequest { string user_id = 1; }
message DeactivateUserResponse { User user = 1; }
message ReactivateUserRequest { string user_id = 1; }
message ReactivateUserResponse { User user = 1; }

message ListSessionsForUserRequest { string user_id = 1; }
message ListSessionsForUserResponse { repeated Session sessions = 1; }
message Session {
  string id = 1;            // opaque token hash, never the raw token
  string user_id = 2;
  google.protobuf.Timestamp created_at = 3;
  google.protobuf.Timestamp expires_at = 4;
  google.protobuf.Timestamp last_seen_at = 5;
  string ip = 6;
  string user_agent = 7;
}
message ForceRevokeAllSessionsForUserRequest { string user_id = 1; }
message ForceRevokeAllSessionsForUserResponse { int32 revoked_count = 1; }

message AccessPolicy {
  string id = 1;
  string name = 2;
  string kind = 3;           // "role-definition" | "rate-tier" | ...
  string document_json = 4;  // JSONB document, serialized
  int32 version = 5;
  string updated_by = 6;
  google.protobuf.Timestamp updated_at = 7;
}
message CreateAccessPolicyRequest { string name = 1; string kind = 2; string document_json = 3; }
message GetAccessPolicyRequest { string id = 1; }
message ListAccessPoliciesRequest { string page_token = 1; int32 page_size = 2; }
message ListAccessPoliciesResponse { repeated AccessPolicy policies = 1; string next_page_token = 2; }
message UpdateAccessPolicyRequest { string id = 1; string document_json = 2; int32 expected_version = 3; }
message DeleteAccessPolicyRequest { string id = 1; }

message GetAdminStatsRequest {}
message GetAdminStatsResponse {
  int32 total_users = 1;
  int32 active_sessions = 2;
  int32 total_policies = 3;
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

Expected: clean build, `buf breaking` reports no breaking changes (only additions).
