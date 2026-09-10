# TASK-AWS-02-03: Reject new direct-websocket connections at `MaxConcurrentSessions` (WS 1008)

**From Solution:** SOL-AWS-02
**Priority:** P2
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/capacity.go` (new), `server.go`, `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go`
**Depends on:** TASK-AWS-02-01
**Status:** [x] DONE — `LiveSessionCount` on `Client`, `capacity.go`'s `SessionCounter`, `Server.Sessions`/pre-upgrade capacity check in `ServeHTTP`, wired in main.go; 2 new tests (at-capacity pre-auth reject, disabled-by-default no-regression) green.

---

## Context

BUG-AWS-02's "no capacity-limit handling" finding is a genuine gap (unlike
the version-check/close-code items, it has no TS-era or agent-side
precedent). Add a lightweight circuit-breaker, the same shape
`infra-fleet-service.md` §8 already specifies for `MAX_CONCURRENT_STREAMS`.
Closes with 1008, per SOL-AWS-02's decision not to invent close code 4004.

## Changes to make

Add `LiveSessionCount` to `devserveragent.Client`
(`backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go`),
reading `len(c.sessions)` under the existing mutex — no new state:

```go
// LiveSessionCount reports the number of dev servers this Client currently
// holds a session entry for (handshaked or not — reconnecting sessions
// still occupy a slot) — backs agentwsserver's capacity check
// (TASK-AWS-02-03).
func (c *Client) LiveSessionCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.sessions)
}
```

Create `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/capacity.go`:

```go
package agentwsserver

// SessionCounter is the narrow seam Server needs to enforce
// Cfg.MaxConcurrentSessions — implemented by devserveragent.Client's
// LiveSessionCount.
type SessionCounter interface {
	LiveSessionCount() int
}
```

Extend `Server` with the counter and enforce it in `ServeHTTP`, before the
WS upgrade even completes:

```go
type Server struct {
	Registry *Registry
	Client   InboundSessionAttacher
	Tokens   TokenValidator // TASK-AWS-03-06
	Sessions SessionCounter // TASK-AWS-02-03 — may be the same concrete value as Client
	Cfg      Config
	Logger   *slog.Logger
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.Cfg.MaxConcurrentSessions > 0 && s.Sessions != nil && s.Sessions.LiveSessionCount() >= s.Cfg.MaxConcurrentSessions {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err == nil {
			conn.Close(websocket.StatusPolicyViolation, "Server at capacity") // 1008, not a new 4004 — see SOL-AWS-02
		}
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		s.logger().ErrorContext(r.Context(), "agentwsserver: ws upgrade failed", slog.Any("error", err))
		return
	}
	s.handleConnection(r.Context(), conn)
}
```

In `cmd/server/main.go`, pass `agentClient` (already implements
`LiveSessionCount`) as `Server.Sessions` when constructing
`infraagentwsserver.New(...)` / setting `Server` fields.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/agentwsserver/... ./services/infra-fleet-service/internal/adapter/devserveragent/...
```

Expected: clean build/tests. Add to `server_test.go`: with a fake
`SessionCounter` stubbed at the configured cap, a new connection is closed
1008 before the handshake read even starts (assert no `Registry.Consume`
call — the reject must be pre-auth, not post); a `MaxConcurrentSessions <=
0` config disables the check entirely (existing behavior, no regression).
