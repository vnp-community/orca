# TASK-AWS-01-03: Resolve relay-websocket's bearer token per-dial, remove `ORCA_AGENT_TOKEN`

**From Solution:** SOL-AWS-01
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/config.go`, `client.go`, `session.go`
**Depends on:** TASK-AWS-01-02, TASK-AWS-03-04, TASK-AWS-03-05
**Status:** [x] DONE — removed `Config.Token`/`ORCA_AGENT_TOKEN`; added `Client.AgentTokenSource`/`WithAgentTokens`, `session.tokenSource`, updated `connect`/`backgroundReconnect` to resolve per-dial; wired `agentTokenSource` composition-root adapter (over `agentTokenStore` + `credentialBrokerClient`) into main.go; 3 new regression tests (no-token fails without dialing, two DevServers produce two distinct Authorization headers, revoked token's next dial fails closed with no reconnect) all green; `grep ORCA_AGENT_TOKEN` returns nothing.

---

## Context

`Config.Token`/`ORCA_AGENT_TOKEN` (`config.go:16-24,86`) is one
process-wide value reused for every relay-websocket `DevServer` — the bug
this task fixes. Replace it with a per-`DevServer` lookup through
`AgentTokenRepository.ActiveForDevServer` (TASK-AWS-03-04) +
`CredentialBrokerClient.ResolveCredential` (TASK-AWS-01-02), resolved fresh
on every dial (not cached across process restarts) so a revoked token is
honored on the very next reconnect.

## Changes to make

In `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/config.go`,
remove `Token` from `Config` and `LoadConfigFromEnv`'s `ORCA_AGENT_TOKEN`
read (delete lines `config.go:28-32` field, and `config.go:86`
`cfg.Token = os.Getenv("ORCA_AGENT_TOKEN")`). Update the package/type doc
comment (`config.go:9-24`) to remove the now-stale
"deployment-wide shared secret" rationale — replace with a pointer to this
task and SOL-AWS-01.

In `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go`,
add the resolution port and thread it into `Client`:

```go
// AgentTokenSource is the narrow seam Client needs to resolve a
// relay-websocket DevServer's current bearer token — implemented over
// usecase.AgentTokenRepository + usecase.CredentialBrokerClient
// (TASK-AWS-03-04, TASK-AWS-01-02). Defined here per this package's
// existing "accept interfaces, return structs" convention (see
// SshProvisioner's doc comment).
type AgentTokenSource interface {
	// TokenFor resolves devServer's current active relay-websocket bearer
	// token, resolved fresh — not cached across process restarts — so a
	// revoked token is honored on the very next dial with no deploy.
	TokenFor(ctx context.Context, devServer domain.DevServer) (string, error)
}
```

```go
type Client struct {
	cfg    Config
	logger *slog.Logger

	mu       sync.Mutex
	sessions map[string]*session

	sshProvisioner SshProvisioner
	// tokens resolves relay-websocket bearer tokens per dial — nil means
	// relay-websocket dev servers always fail to connect (WithAgentTokens
	// was not passed to New), matching sshProvisioner's nil-means-disabled
	// convention.
	tokens AgentTokenSource
}

// WithAgentTokens enables relay-websocket mode by supplying the
// per-DevServer token resolver — see AgentTokenSource's doc comment.
func WithAgentTokens(tokens AgentTokenSource) Option {
	return func(c *Client) {
		c.tokens = tokens
	}
}
```

Update `getOrDialSession` to resolve and pass the token instead of relying
on `c.cfg.Token`:

```go
func (c *Client) getOrDialSession(ctx context.Context, devServer domain.DevServer) (*session, error) {
	if c.tokens == nil {
		return nil, fmt.Errorf("devserveragent: relay-websocket support was not enabled (see WithAgentTokens)")
	}
	token, err := c.tokens.TokenFor(ctx, devServer)
	if err != nil {
		return nil, fmt.Errorf("devserveragent: resolving agent token for dev server %s: %w", devServer.ID, err)
	}

	c.mu.Lock()
	sess, ok := c.sessions[devServer.ID]
	if !ok {
		sess = newSession(devServer.Host, c.cfg, c.logger)
		c.sessions[devServer.ID] = sess
	}
	c.mu.Unlock()

	if sess.isHandshaked() {
		return sess, nil
	}
	if err := sess.connect(ctx, token); err != nil {
		return nil, err
	}
	return sess, nil
}
```

In `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go`,
change `connect` to accept the resolved token instead of reading
`s.cfg.Token` (`session.go:111-134`):

```go
// connect dials the agent and runs the initiator handshake using token
// (resolved per-dial by the caller — see Client.AgentTokenSource). Safe to
// call again after a disconnect (reconnect path).
func (s *session) connect(ctx context.Context, token string) error {
	if token == "" {
		return fmt.Errorf("devserveragent: no relay-websocket token resolved for this dev server (see SOL-AWS-01)")
	}

	dialCtx, cancel := context.WithTimeout(ctx, s.cfg.DialTimeout)
	defer cancel()

	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	conn, _, err := websocket.Dial(dialCtx, s.wsURL(), &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		return fmt.Errorf("devserveragent: dial %s: %w", s.wsURL(), err)
	}

	info, err := s.runInitiatorHandshake(ctx, conn)
	if err != nil {
		conn.Close(websocket.StatusProtocolError, "handshake failed")
		return err
	}

	s.attachTransport(newWSTransport(conn, s.logger), info)
	return nil
}
```

`backgroundReconnect` (`session.go:420-473`) also calls `s.connect(ctx)` —
it has no `AgentTokenSource` in scope (it lives on `*session`, not
`*Client`). Store the resolved token on the session at dial time
(`s.token`, set at the top of `connect`, protected by `s.mu` like the rest
of the session's mutable state) and have `backgroundReconnect` re-resolve
via a small callback field instead of a bare string, so a token rotated
between reconnect attempts is picked up:

```go
// session gains:
tokenSource func(ctx context.Context) (string, error) // set by Client at newSession time for relay-websocket only; nil for direct-websocket/relay-ssh

// backgroundReconnect's inner loop, replacing `err := s.connect(ctx)`:
token := ""
if s.tokenSource != nil {
	token, err = s.tokenSource(ctx)
	if err != nil {
		s.reconnectAttempt++
		continue // treat as a failed attempt, same backoff as a dial failure
	}
}
err = s.connect(ctx, token)
```

Wire `newSession`'s relay-websocket call site (`Client.getOrDialSession`)
to pass a closure capturing `devServer`:
`newSession(devServer.Host, c.cfg, c.logger)` gains a fourth argument, or
sets `sess.tokenSource` right after construction —
`sess.tokenSource = func(ctx context.Context) (string, error) { return c.tokens.TokenFor(ctx, devServer) }`.

Finally, in `cmd/server/main.go`, pass
`infradevserveragent.WithAgentTokens(agentTokenSource)` alongside the
existing `WithRelaySSH(provisioner)` option when constructing `agentClient`
— `agentTokenSource` is a small adapter over `agentTokenStore`
(TASK-AWS-03-04) + `credentialBrokerClient` (TASK-AWS-01-02) implementing
`TokenFor` per this task's `AgentTokenSource` interface.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/devserveragent/...
grep -rn "ORCA_AGENT_TOKEN" services/infra-fleet-service/internal/adapter/devserveragent/
```

Expected: clean build/tests; the `grep` for `ORCA_AGENT_TOKEN` returns
nothing (fully removed, not just unused). Add to `client_test.go`: dialing
a relay-websocket `DevServer` with no registered token fails with a clear
error, no dial attempted; two different `DevServer`s with two different
tokens produce two different `Authorization` headers (regression guard
against the shared-token bug); a revoked token's *next* dial attempt fails
closed with no process restart.
