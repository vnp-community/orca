# TASK-AUTH-04-01: `CreateUserRequest.password`, `ListSessions` RPC, `UpdateUser` RPC

**From Solution:** SOL-AUTH-04
**Priority:** P0 — every later task in this set depends on generated stubs from this
**Service:** `auth-service` (proto)
**File:** `backend-go/proto/orca/auth/v1/auth.proto`
**Depends on:** none
**Status:** `[x]` DONE — added `CreateUserRequest.password`, `ListSessions`/`UpdateUser` RPCs + messages; `make proto-gen` regenerated stubs; `go build ./proto/...` clean.

---

## Context

`docs/logic/auth/BL-AUTH-04-admin-user-crud.md`'s spec has `POST /admin/api/users` take `{email, name, role, password}`, but `CreateUserRequest` has no `password` field, so `create_user.go` generates a random password that's discarded — the created account is permanently unusable. `auth-service.md` §3 also already names `UpdateUser` in the RPC surface (only the narrower `UpdateUserRole` exists today), and the admin dashboard needs a cross-user, tenant-scoped session listing RPC that doesn't exist yet (`ListSessionsForUser` is single-user-scoped only). This task adds all three additively.

## Changes to make

In `backend-go/proto/orca/auth/v1/auth.proto`, change `CreateUserRequest`:

```protobuf
message CreateUserRequest {
  string email = 1;
  string name = 2;
  string tenant_id = 3;
  Role role = 4;
  // password: the admin-chosen initial credential, per this RPC's spec
  // contract (docs/logic/auth/BL-AUTH-04-admin-user-crud.md). Communicated
  // to the new user out-of-band (email/Slack/etc.) by the admin — this
  // service has no email-sending capability and none is added by this
  // change. Contrast with Bootstrap.EnsureAdmin, which generates+prints its
  // own password because no admin actor exists yet to supply one for the
  // very first account.
  string password = 5;
}
```

Add to the `AuthService` service block:

```protobuf
// ListSessions is the cross-user, tenant-scoped admin session-dashboard
// RPC — distinct from ListSessionsForUser (single-user scope, kept as-is).
// Scope addition beyond this service's original RPC surface (auth-service.md
// §3 only lists ListSessionsForUser).
rpc ListSessions(ListSessionsRequest) returns (ListSessionsResponse);

// UpdateUser — the RPC auth-service.md §3 already names but the current
// codebase never implemented (only the narrower UpdateUserRole exists).
rpc UpdateUser(UpdateUserRequest) returns (UpdateUserResponse);
```

Add messages (append to the bottom of the file):

```protobuf
message ListSessionsRequest {
  string tenant_id = 1;  // ignored if caller-supplied — resolved from the
                          // admin's own validated identity server-side
  string page_token = 2;
  int32  page_size  = 3;
}
message ListSessionsResponse {
  repeated SessionWithUser sessions = 1;
  string next_page_token = 2;
}
// SessionWithUser avoids an N+1 user lookup per session row in the admin
// dashboard — email is denormalized into the response via a JOIN, not a
// second round trip per row.
message SessionWithUser {
  Session session = 1;
  string user_email = 2;
}

// UpdateUserRequest — wrapper types distinguish "field omitted" from "field
// explicitly set to empty string" for a true partial update.
message UpdateUserRequest {
  string user_id = 1;
  google.protobuf.StringValue email = 2;
  google.protobuf.StringValue name  = 3;
  optional Role role = 4; // proto3 `optional` scalar — present/absent distinguishable
}
message UpdateUserResponse {
  User user = 1;
}
```

Ensure `google/protobuf/wrappers.proto` is imported for `StringValue` if not already.

Regenerate stubs:

```bash
cd /opt/repos/orca/backend-go
buf generate proto
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./proto/...
buf breaking proto --against '.git#branch=main'
```

Expected: clean build; `buf breaking` reports no breaking changes (only additions); `proto/gen/go/orca/auth/v1/auth.pb.go`'s `CreateUserRequest` now has `GetPassword()`, and `AuthServiceClient`/`AuthServiceServer` now declare `ListSessions`/`UpdateUser`.
