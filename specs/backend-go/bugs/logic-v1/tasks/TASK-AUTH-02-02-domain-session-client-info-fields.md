# TASK-AUTH-02-02: `domain.Session` gains `LastSeenAt`/`IP`/`UserAgent` + `WithClientInfo`

**From Solution:** SOL-AUTH-02
**Priority:** P0
**Service:** `auth-service` (domain)
**File:** `backend-go/services/auth-service/internal/domain/session.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`domain.Session` has no `LastSeenAt`/`IP`/`UserAgent` fields today, even though the proto `Session` message and the target schema already have them. `NewSession` keeps its existing five required params — the new fields are metadata, not an invariant `NewSession`'s error returns need to enforce, so they're set via a `WithClientInfo` builder method instead of growing the constructor.

## Changes to make

In `backend-go/services/auth-service/internal/domain/session.go`, change the `Session` struct:

```go
type Session struct {
	TokenHash  string
	UserID     string
	TenantID   string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	LastSeenAt *time.Time // nil until first touch — see ValidateSession (TASK-AUTH-02-06)
	IP         string     // may be "" for a session created before this migration, or if unresolved
	UserAgent  string
}
```

`NewSession`'s body and required-field validation are unchanged — it still constructs a zero-value `LastSeenAt`/`IP`/`UserAgent`. Add below `NewSession`:

```go
// WithClientInfo sets IP/UserAgent on a Session after construction —
// metadata, not an invariant NewSession's error returns need to enforce.
func (s Session) WithClientInfo(ip, userAgent string) Session {
	s.IP, s.UserAgent = ip, userAgent
	return s
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/...
go test ./services/auth-service/internal/domain/... -run TestSession -v
```

Expected: build succeeds; add a `WithClientInfo` test asserting it sets `IP`/`UserAgent` without disturbing other fields, and that existing `NewSession` invariant tests (`ErrEmptyTokenHash`, `ErrEmptyUser`, `ErrZeroExpiry`, `ErrExpiryBeforeCreation`) still pass against the widened struct.
