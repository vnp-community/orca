// Package devserveragent is infra-fleet-service's outbound adapter to the
// Dev Server Agent execution plane (agent/) — this service's "defining
// adapter" per specs/backend-go/services/infra-fleet-service.md §6.
//
// # Epic A status: all three connection modes are real
//
// Per §10 of the design doc ("Protocol decision: Option A"), a full
// implementation covers three connection modes (domain.ConnectionMode:
// relay-ssh, relay-websocket, direct-websocket) over the same
// 13-byte-framed JSON-RPC wire protocol (Stack B, relay-protocol.ts) — see
// frame.go/jsonrpc.go/session.go's doc comments for the line-by-line
// correspondence.
//
//   - relay-websocket: Orca dials out to the agent's own WebSocket server
//     (agent-connection-relay.ts), authenticating with a per-DevServer
//     bearer token resolved fresh on every dial (AgentTokenSource, see
//     TASK-AWS-01-03/SOL-AWS-01).
//   - direct-websocket: the agent dials in to adapter/agentwsserver's
//     inbound WS server, authenticating with a single-use, SHA-256-hashed
//     token slot; a successful handshake there calls
//     Client.AttachInboundSession.
//   - relay-ssh: adapter/sshrelay resolves the DevServer's SSHTargetID,
//     opens a real Vault-cert-authenticated SSH connection
//     (adapter/sshconn), SFTP-deploys agent/out/agent.js, launches it over
//     the SSH exec channel in `--stdio` mode (a third agent-side connection
//     mode added specifically for this — see agent/src/relay/agent-connection-stdio.ts),
//     and runs the receiver-side agent.handshake exchange (no token check —
//     the SSH connection itself is the trust boundary, matching the design
//     doc §"relay-ssh" auth model). See Client.SshProvisioner's doc comment
//     for exactly what wires this in.
//
// Every mode ends up in the same place: a *session (session.go) holding a
// live Transport (transport.go) — relay-websocket/direct-websocket use a
// wsTransport, relay-ssh's Transport is implemented by adapter/sshrelay
// over the SSH exec channel's stdio (with its own incremental frame
// decoder, since unlike a WebSocket, SSH exec-channel stdio delivers
// arbitrary-sized chunks with no message boundary). Exec/Health are
// mode-agnostic from here — no relay-ssh-specific branch exists in either.
//
// # Method surface is a generic passthrough
//
// Exec is a generic passthrough (method string + params map) with no
// per-method translation layer, for every mode — see
// specs/agent/api/agent-rpc-catalog-*.md for the real catalog; wiring
// specific callers (e.g. wscompat's devServer.*/fleet.* channels) to
// specific method names is intentionally left to those call sites, not
// baked into this transport.
package devserveragent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/coder/websocket"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// ErrConnectionModeNotImplemented is returned for a devServer.Mode this
// Client doesn't recognize, and for relay-ssh when no SshProvisioner was
// configured via WithRelaySSH — see the package doc comment.
var ErrConnectionModeNotImplemented = errors.New("devserveragent: connection mode not implemented — see package doc comment")

// SshProvisioner is relay-ssh mode's session-establishment port —
// implemented by adapter/sshrelay.Provisioner (deploy+launch+handshake over
// a real SSH connection). Provision must return a live, already-handshaked
// Transport and the agent's HandshakeInfo; Client.getOrProvisionSession
// attaches it exactly like the other two modes' connection-establishment
// paths. Defined here (consumer-side), not in adapter/sshrelay, per this
// codebase's Dependency Inversion convention (see e.g.
// usecase/ports.go's own doc comment on why a port is defined where it's
// consumed).
type SshProvisioner interface {
	Provision(ctx context.Context, devServer domain.DevServer) (Transport, HandshakeInfo, error)
}

// sshReattacherAndProvisioner is what WithRelaySSH actually requires: a
// single concrete value (adapter/sshrelay.Provisioner) implementing both
// narrow ports — SshProvisioner for the first connect, SshReattacher for
// every reconnect after (relaySSHReconnect, session.go).
type sshReattacherAndProvisioner interface {
	SshProvisioner
	SshReattacher
}

// Client implements usecase.DevServerAgentClient for all three connection
// modes, keeping one persistent session per dev server ID (reused across
// calls; established lazily on first use). A dropped relay-websocket
// session recovers on its own via session.go's backgroundReconnect, with
// getOrCreateSession's lazy redial as the fallback for a call that arrives
// mid-backoff. direct-websocket/relay-ssh sessions are managedExternally —
// see session.go's doc comment on backgroundReconnect for why they don't
// auto-reconnect the same way.
type Client struct {
	cfg    Config
	logger *slog.Logger

	mu       sync.Mutex
	sessions map[string]*session

	// sshProvisioner is nil unless WithRelaySSH was passed to New —
	// relay-ssh mode returns ErrConnectionModeNotImplemented until it is.
	sshProvisioner SshProvisioner

	// sshReattacher is the same value as sshProvisioner (both narrow ports
	// implemented by the one *sshrelay.Provisioner WithRelaySSH is given) —
	// getOrProvisionSession sets it on every relay-ssh session so
	// relaySSHReconnect can call Reattach later.
	sshReattacher SshReattacher

	// tokens resolves relay-websocket bearer tokens per dial — nil means
	// relay-websocket dev servers always fail to connect (WithAgentTokens
	// was not passed to New), matching sshProvisioner's nil-means-disabled
	// convention.
	tokens AgentTokenSource
}

var _ usecase.LiveSessionCloser = (*Client)(nil)

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

// Option configures optional Client behavior beyond relay-websocket/
// direct-websocket, which need none — currently only WithRelaySSH.
type Option func(*Client)

// WithRelaySSH enables relay-ssh mode by supplying the provisioner that
// deploys/launches/attaches a session for a given DevServer — see
// adapter/sshrelay.Provisioner (the production implementation, over a real
// SSH connection via adapter/sshconn), which implements both SshProvisioner
// (first connect) and SshReattacher (every reconnect after — see
// session.go's relaySSHReconnect).
func WithRelaySSH(provisioner sshReattacherAndProvisioner) Option {
	return func(c *Client) {
		c.sshProvisioner = provisioner
		c.sshReattacher = provisioner
	}
}

// WithAgentTokens enables relay-websocket mode by supplying the
// per-DevServer token resolver — see AgentTokenSource's doc comment.
func WithAgentTokens(tokens AgentTokenSource) Option {
	return func(c *Client) {
		c.tokens = tokens
	}
}

// New constructs a Client. WithAgentTokens must be passed for any
// relay-websocket dev server to be reachable — see AgentTokenSource's doc
// comment for why this is resolved per-dial rather than a single
// deployment-wide config value.
func New(cfg Config, logger *slog.Logger, opts ...Option) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	c := &Client{cfg: cfg, logger: logger, sessions: make(map[string]*session)}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// getOrCreateSession returns devServer's persistent session, dispatching by
// connection mode.
func (c *Client) getOrCreateSession(ctx context.Context, devServer domain.DevServer) (*session, error) {
	switch devServer.Mode {
	case domain.ConnectionModeRelayWebSocket:
		return c.getOrDialSession(ctx, devServer)
	case domain.ConnectionModeDirectWebSocket:
		return c.getInboundSession(devServer)
	case domain.ConnectionModeRelaySSH:
		return c.getOrProvisionSession(ctx, devServer)
	default:
		return nil, fmt.Errorf("%w: devServer.Mode=%q", ErrConnectionModeNotImplemented, devServer.Mode)
	}
}

// getOrDialSession is relay-websocket's original path: create the session
// lazily on first use, (re)dial if the previous connection dropped. The
// bearer token is resolved fresh via c.tokens on every dial — never cached
// across process restarts — so a revoked token is honored on the very next
// reconnect attempt (see AgentTokenSource's doc comment, TASK-AWS-01-03).
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
		sess.tokenSource = func(ctx context.Context) (string, error) { return c.tokens.TokenFor(ctx, devServer) }
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

// getInboundSession is direct-websocket's path. There is nothing to
// lazily (re)connect here — the agent must dial in on its own (via
// adapter/agentwsserver, which calls AttachInboundSession on success). An
// absent or dropped session is a real "not reachable right now" condition,
// not something this call can fix by dialing anywhere.
func (c *Client) getInboundSession(devServer domain.DevServer) (*session, error) {
	c.mu.Lock()
	sess, ok := c.sessions[devServer.ID]
	c.mu.Unlock()
	if !ok || !sess.isHandshaked() {
		return nil, fmt.Errorf("devserveragent: no live inbound connection from dev server %q (direct-websocket mode) — the agent must dial in first", devServer.ID)
	}
	return sess, nil
}

// getOrProvisionSession is relay-ssh's path: reuse an already-live session
// (a deployed+launched agent.js --stdio process bridged over one SSH
// connection) if one exists, otherwise ask the configured SshProvisioner to
// deploy/launch/handshake a fresh one. Unlike relay-websocket's redial,
// re-provisioning means a brand new SSH connection + SFTP deploy + process
// launch — real cost, which is exactly why a live session is reused
// whenever possible rather than re-provisioned per call.
func (c *Client) getOrProvisionSession(ctx context.Context, devServer domain.DevServer) (*session, error) {
	c.mu.Lock()
	sess, ok := c.sessions[devServer.ID]
	c.mu.Unlock()
	if ok && sess.isHandshaked() {
		return sess, nil
	}

	if c.sshProvisioner == nil {
		return nil, fmt.Errorf("%w: relay-ssh support was not enabled (see WithRelaySSH)", ErrConnectionModeNotImplemented)
	}
	t, info, err := c.sshProvisioner.Provision(ctx, devServer)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	sess, ok = c.sessions[devServer.ID]
	if !ok {
		sess = newSession(devServer.Host, c.cfg, c.logger)
		c.sessions[devServer.ID] = sess
	}
	c.mu.Unlock()

	// s.mu-guarded: relaySSHReconnect (a goroutine spawned by a PRIOR
	// attachTransport, if this session already existed) may concurrently
	// read managedMode/relaySSHDevServer/reattacher.
	sess.mu.Lock()
	sess.managedMode = managedModeRelaySSHReattach
	sess.relaySSHDevServer = devServer
	sess.reattacher = c.sshReattacher
	sess.mu.Unlock()

	sess.attachTransport(t, info)
	return sess, nil
}

// AttachInboundSession registers an already-authenticated inbound
// WebSocket connection as devServerID's live session — called by
// adapter/agentwsserver once its handshake + token-slot validation
// succeeds (direct-websocket mode: the agent dialed Orca, not the other
// way around). Reuses the exact same readLoop/keepAliveLoop/call machinery
// connect() uses for the outbound case; only how the transport/info was
// obtained differs. Safe to call again for a reconnecting agent — reuses
// the same session object so in-flight callers of Exec/Health naturally
// see the new live connection once attached.
func (c *Client) AttachInboundSession(devServerID, host string, conn *websocket.Conn, info HandshakeInfo) {
	c.mu.Lock()
	sess, ok := c.sessions[devServerID]
	if !ok {
		sess = newSession(host, c.cfg, c.logger)
		c.sessions[devServerID] = sess
	}
	c.mu.Unlock()

	sess.mu.Lock()
	sess.managedMode = managedModeInboundOnly
	sess.mu.Unlock()

	sess.attachTransport(newWSTransport(conn, c.logger), info)
}

// LiveSessionCount reports the number of dev servers this Client currently
// holds a session entry for (handshaked or not — reconnecting sessions
// still occupy a slot) — backs agentwsserver's capacity check
// (TASK-AWS-02-03).
func (c *Client) LiveSessionCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.sessions)
}

// CloseSessionsForDevServerToken implements usecase.LiveSessionCloser.
// direct-websocket only: this Client tracks at most one live session per
// devServerID (see the Client doc comment), so "close the session
// authenticated as tokenID" reduces to "close devServerID's current
// session" — the token/session binding itself isn't tracked separately
// (a revoked token's *next* handshake attempt is what actually enforces
// revocation; this call just also drops the currently-live connection, if
// any, per SOL-AWS-03's immediate-effect guarantee).
func (c *Client) CloseSessionsForDevServerToken(ctx context.Context, devServerID, tokenID string) (int, error) {
	c.mu.Lock()
	sess, ok := c.sessions[devServerID]
	c.mu.Unlock()
	if !ok || !sess.isHandshaked() {
		return 0, nil
	}
	sess.close()
	return 1, nil
}

// HandshakeInfoFor returns the live session's most recently attached
// HandshakeInfo for devServerID — backs ResolveConnection's optional
// Node-version enrichment (TASK-INT-03-02). found=false covers both "no
// session at all" and "session exists but isn't currently handshaked",
// mirroring LiveSessionCount's mutex-guarded read shape.
func (c *Client) HandshakeInfoFor(devServerID string) (HandshakeInfo, bool) {
	c.mu.Lock()
	sess, ok := c.sessions[devServerID]
	c.mu.Unlock()
	if !ok {
		return HandshakeInfo{}, false
	}
	return sess.handshakeInfoSnapshot()
}

// CancelReconnect stops devServerID's session's relaySSHReconnect (or
// relay-websocket backgroundReconnect) loop immediately, mirroring
// session.close()'s existing closeCh-signaling shape — the session itself
// is not closed, only its in-flight reconnect attempt is abandoned, so a
// later Exec/Health call still triggers a fresh getOrProvisionSession/
// getOrDialSession rather than staying permanently dead.
func (c *Client) CancelReconnect(devServerID string) {
	c.mu.Lock()
	sess, ok := c.sessions[devServerID]
	c.mu.Unlock()
	if !ok {
		return
	}
	sess.cancelReconnect()
}

// Exec dispatches one JSON-RPC method call (e.g. "ports.scan",
// "preflight.check", "shell.exec") to the Dev Server Agent over devServer's
// resolved transport and decodes its JSON-RPC result into a map — the
// package doc comment's "method/params are passed through verbatim, no
// per-method translation" contract, uniform across all three modes.
func (c *Client) Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error) {
	sess, err := c.getOrCreateSession(ctx, devServer)
	if err != nil {
		return nil, err
	}
	result, err := sess.call(ctx, method, params)
	if err != nil {
		// JSON-RPC standard "method not found" (-32601): the agent answered,
		// it just doesn't implement method on this build — a permanent,
		// typed condition callers must distinguish from a transport/timeout
		// failure. See domain.ErrAgentMethodNotFound's doc comment.
		var rpcErr *JSONRPCError
		if errors.As(err, &rpcErr) && rpcErr.Code == jsonrpcMethodNotFoundCode {
			return nil, fmt.Errorf("%w: %v", domain.ErrAgentMethodNotFound, err)
		}
		return nil, err
	}
	if len(result) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("devserveragent: decoding %q result: %w", method, err)
	}
	return out, nil
}

// Health performs a connect+handshake check (or reuses an already-live
// session) — distinct from the SSH-exec-based fleet health poll that
// usecase.GetFleetHealth reads from Postgres. A connect/provision failure
// of any kind is reported as (false, nil) — "not reachable" is the
// expected/common answer this method exists to give, not an error
// condition the caller must additionally branch on. Uniform across all
// three modes.
func (c *Client) Health(ctx context.Context, devServer domain.DevServer) (bool, error) {
	sess, err := c.getOrCreateSession(ctx, devServer)
	if err != nil {
		c.logger.DebugContext(ctx, "devserveragent: health check unreachable", slog.String("devServerId", devServer.ID), slog.Any("error", err))
		return false, nil
	}
	return sess.isHandshaked(), nil
}

// LastHandshakeInfo returns the HandshakeInfo captured at the most recent
// successful attachTransport for devServerID, if a live, handshaked session
// exists. Cheap in-memory lookup — no round trip to the remote host.
// EstablishConnection (TASK-FLEET-04-03) uses this to persist
// handshake-derived facts (platform/arch/node version) after a successful
// connect, without a second round trip.
func (c *Client) LastHandshakeInfo(devServerID string) (usecase.HandshakeInfo, bool) {
	c.mu.Lock()
	sess, ok := c.sessions[devServerID]
	c.mu.Unlock()
	if !ok {
		return usecase.HandshakeInfo{}, false
	}
	info, found := sess.handshakeInfoSnapshot()
	if !found {
		return usecase.HandshakeInfo{}, false
	}
	// Convert at the adapter boundary — usecase.HandshakeInfo is a
	// deliberate duplicate of this package's own HandshakeInfo (see
	// usecase/ports.go's doc comment: usecase must never import this
	// adapter package, which already imports usecase to implement
	// DevServerAgentClient).
	return usecase.HandshakeInfo{
		Platform: info.Platform, Arch: info.Arch, NodeVersion: info.NodeVersion, AgentVersion: info.AgentVersion,
	}, true
}

// Note: relay-ssh's liveness check is NOT a separate dial-and-probe path —
// Health above already covers it uniformly via getOrCreateSession's
// mode dispatch to getOrProvisionSession/sshProvisioner.Provision, the same
// as Exec. An earlier draft of this method had its own dialRelaySSH/
// relaySSHHealth pair against a sshConnector/sshTargetResolver field pair;
// that was superseded by the sshProvisioner abstraction above and dropped
// as dead code during TASK-192's merge.

// StreamPty subscribes to ptyID's pty.data/pty.exit/pty.replay notifications
// over devServer's persistent session (see session.go's subscribePty/
// routeNotification) and translates them into usecase.PtyEvent. relay-ssh
// mode has no persistent session (see package doc comment) so this always
// errors for it, same as getInboundSession's absent-session case.
//
// The returned channel is closed once unsubscribe runs (or ctx is done,
// whichever first) — every caller (usecase.AttachPty, usecase.WaitTerminalSession)
// MUST call unsubscribe exactly once, typically via defer, to release the
// session-level subscription slot.
func (c *Client) StreamPty(ctx context.Context, devServer domain.DevServer, ptyID string) (<-chan usecase.PtyEvent, func(), error) {
	if devServer.Mode == domain.ConnectionModeRelaySSH {
		return nil, nil, fmt.Errorf("%w: relay-ssh mode has no pty.* JSON-RPC surface (no relay.js deployed)", ErrConnectionModeNotImplemented)
	}
	sess, err := c.getOrCreateSession(ctx, devServer)
	if err != nil {
		return nil, nil, err
	}

	raw := sess.subscribePty(ptyID)
	out := make(chan usecase.PtyEvent, 64)
	done := make(chan struct{})
	var closeOnce sync.Once

	go func() {
		defer close(out)
		for {
			select {
			case n, ok := <-raw:
				if !ok {
					return
				}
				out <- usecase.PtyEvent{PtyID: n.PtyID, Data: n.Data, Exited: n.Exited, ExitCode: n.ExitCode}
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	unsubscribe := func() {
		closeOnce.Do(func() {
			close(done)
			sess.unsubscribePty(ptyID, raw)
		})
	}
	return out, unsubscribe, nil
}

// ExecStream dispatches one streaming JSON-RPC method call (currently only
// "git.execStream", TASK-PW-03-08/SOL-PW-03) whose result arrives as
// multiple response frames replying to the original request id instead of
// one — see session.go's callStream/isTerminalStreamResponse for the wire
// mechanics, distinct from StreamPty's out-of-band notification demux
// above (routeNotification keyed by pty id): these are ordinary JSON-RPC
// response frames sharing one request id, not notifications.
//
// relay-ssh mode has no persistent session (same restriction as StreamPty)
// so this always errors for it — usecase.RelayStream's caller
// (git-gateway-service's PushStream/PullStream) is expected to have
// already rejected relay-ssh before ever reaching here (SOL-PW-03's
// domain.ErrGitOpUnsupportedOverSSHRelay check), so this is a defense in
// depth, not the primary guard.
//
// The returned channel is closed once the agent's terminal frame is
// observed, the session disconnects, or ctx is cancelled — every caller
// MUST still call the returned unsubscribe func (typically via defer) to
// release the pending-call slot on an early return.
func (c *Client) ExecStream(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (<-chan map[string]any, func(), error) {
	if devServer.Mode == domain.ConnectionModeRelaySSH {
		return nil, nil, fmt.Errorf("%w: relay-ssh mode has no streaming JSON-RPC surface (no relay.js deployed)", ErrConnectionModeNotImplemented)
	}
	sess, err := c.getOrCreateSession(ctx, devServer)
	if err != nil {
		return nil, nil, err
	}

	raw, unsubscribe, err := sess.callStream(ctx, method, params)
	if err != nil {
		return nil, nil, err
	}

	out := make(chan map[string]any, 64)
	go func() {
		defer close(out)
		for {
			select {
			case resp, ok := <-raw:
				if !ok {
					return
				}
				if resp.Error != nil {
					out <- map[string]any{"type": "stream.end", "error": resp.Error.Error()}
					return
				}
				var frame map[string]any
				if err := json.Unmarshal(resp.Result, &frame); err != nil {
					continue // malformed frame — skip rather than abort the whole stream
				}
				out <- frame
				if t, _ := frame["type"].(string); t == "stream.end" {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, unsubscribe, nil
}

// Close tears down every open session — call on service shutdown.
func (c *Client) Close() {
	c.mu.Lock()
	sessions := c.sessions
	c.sessions = make(map[string]*session)
	c.mu.Unlock()
	for _, sess := range sessions {
		sess.close()
	}
}
