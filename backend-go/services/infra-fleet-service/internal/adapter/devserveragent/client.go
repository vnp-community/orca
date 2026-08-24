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
//     (agent-connection-relay.ts), authenticating with a static
//     ORCA_AGENT_TOKEN bearer token.
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
}

// Option configures optional Client behavior beyond relay-websocket/
// direct-websocket, which need none — currently only WithRelaySSH.
type Option func(*Client)

// WithRelaySSH enables relay-ssh mode by supplying the provisioner that
// deploys/launches/attaches a session for a given DevServer — see
// adapter/sshrelay.Provisioner (the production implementation, over a real
// SSH connection via adapter/sshconn) and SshProvisioner's doc comment.
func WithRelaySSH(provisioner SshProvisioner) Option {
	return func(c *Client) {
		c.sshProvisioner = provisioner
	}
}

// New constructs a Client. cfg.Token (ORCA_AGENT_TOKEN) must be set for any
// relay-websocket dev server to be reachable — see Config's doc comment for
// why this is deployment-wide config rather than a per-DevServer field.
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
// lazily on first use, (re)dial if the previous connection dropped.
func (c *Client) getOrDialSession(ctx context.Context, devServer domain.DevServer) (*session, error) {
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
	if err := sess.connect(ctx); err != nil {
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
		sess.managedExternally = true
		c.sessions[devServer.ID] = sess
	}
	c.mu.Unlock()

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
		sess.managedExternally = true
		c.sessions[devServerID] = sess
	}
	c.mu.Unlock()

	sess.attachTransport(newWSTransport(conn, c.logger), info)
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
