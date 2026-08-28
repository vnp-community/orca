# TASK-AUTH-01-01: Add `ip`/`user_agent` fields to `LoginRequest`

**From Solution:** SOL-AUTH-01
**Priority:** P0 — everything else in this solution depends on the generated stub fields from this
**Service:** `auth-service` (proto)
**File:** `backend-go/proto/orca/auth/v1/auth.proto`
**Depends on:** none
**Status:** `[x]` DONE — added `ip`/`user_agent` to `LoginRequest`, regenerated via `make proto-gen`; `go build ./proto/...` clean, accessors `GetIp()`/`GetUserAgent()` present.

---

## Context

`api-gateway` needs to pass the resolved client IP and `User-Agent` into `auth-service`'s `Login` RPC so the failure-path audit entry (TASK-AUTH-01-02) and the session's `ip`/`user_agent` columns (see SOL-AUTH-02) can be populated. `LoginRequest` currently only carries `email`/`password`. This is additive-only — no breaking change.

## Changes to make

In `backend-go/proto/orca/auth/v1/auth.proto`, change:

```protobuf
message LoginRequest {
  string email = 1;
  string password = 2;
}
```

to:

```protobuf
message LoginRequest {
  string email = 1;
  string password = 2;
  // ip/user_agent are populated by api-gateway from the terminating HTTP
  // request (real client IP behind any reverse proxy, User-Agent header) —
  // never trusted from an external caller, since Login is only ever called
  // internally by api-gateway over mTLS. See auth-service.md's "who calls
  // whom" contract (§7) — no other service calls Login.
  string ip = 3;
  string user_agent = 4;
}
```

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

Expected: clean build, `buf breaking` reports no breaking changes (only additions), and `proto/gen/go/orca/auth/v1/auth.pb.go`'s `LoginRequest` now has `GetIp()`/`GetUserAgent()` accessors.
