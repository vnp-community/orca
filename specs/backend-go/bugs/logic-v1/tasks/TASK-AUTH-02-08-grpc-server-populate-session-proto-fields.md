# TASK-AUTH-02-08: `toProtoSession` populates `last_seen_at`/`ip`/`user_agent`

**From Solution:** SOL-AUTH-02
**Priority:** P1
**Service:** `auth-service` (grpc adapter)
**File:** `backend-go/services/auth-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-AUTH-02-02, TASK-AUTH-02-04
**Status:** `[x]` DONE — `toProtoSession` sets `Ip`/`UserAgent`/conditional `LastSeenAt`; new `TestServer_ListSessionsForUser` confirms both a touched (LastSeenAt set) and never-touched (unset) session round-trip correctly.

---

## Context

`authv1.Session` already declares `LastSeenAt`/`Ip`/`UserAgent`, but `toProtoSession` (the converter backing `ListSessionsForUser`) never sets them, so the admin console can't show a real `last_seen_at` for any session. This is the final wiring step that makes SOL-AUTH-02's other changes visible end-to-end.

## Changes to make

In `backend-go/services/auth-service/internal/adapter/grpc/server.go`, locate `toProtoSession` (or the equivalent `domain.Session` → `authv1.Session` converter) and change it to:

```go
func toProtoSession(s domain.Session) *authv1.Session {
	out := &authv1.Session{
		Id:        s.TokenHash,
		UserId:    s.UserID,
		CreatedAt: timestamppb.New(s.CreatedAt),
		ExpiresAt: timestamppb.New(s.ExpiresAt),
		Ip:        s.IP,
		UserAgent: s.UserAgent,
	}
	if s.LastSeenAt != nil {
		out.LastSeenAt = timestamppb.New(*s.LastSeenAt)
	}
	return out
}
```

If no such converter function exists yet under that name, locate wherever `ListSessionsForUser`'s RPC handler builds `authv1.Session` values inline and extract/extend that logic to match the shape above.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/...
go test ./services/auth-service/internal/adapter/grpc/... -run TestServer_ListSessionsForUser -v
```

Expected: a `ListSessionsForUser` response for a session with a non-nil `LastSeenAt`/non-empty `IP`/`UserAgent` carries those fields through to the proto response; a session with `LastSeenAt == nil` (never touched) has `LastSeenAt` unset in the response rather than a zero-value timestamp.
