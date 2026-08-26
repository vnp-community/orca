// Package devserveragent is infra-fleet-service's outbound adapter to the
// Dev Server Agent execution plane (agent/) — this service's "defining
// adapter" per specs/backend-go/services/infra-fleet-service.md §6.
//
// # Epic A status: relay-websocket and direct-websocket modes are real;
// # relay-ssh is real for a connection-layer probe + shell.exec only
//
// Per §10 of the design doc ("Protocol decision: Option A"), a full
// implementation covers three connection modes (domain.ConnectionMode:
// relay-ssh, relay-websocket, direct-websocket) over two independently
// wire-compatible-but-not-identical protocol stacks — see
// specs/agent/api/connection-modes.md.
//
// relay-websocket and direct-websocket share Stack B (relay-protocol.ts):
// 13-byte-framed JSON-RPC handshake+call cycle exactly as backend/'s
// DevServerRelayBridge.connectRelayWebSocket + SshChannelMultiplexer do (see
// frame.go/jsonrpc.go/session.go's doc comments for the line-by-line
// correspondence) — relay-websocket dials out with a static
// ORCA_AGENT_TOKEN bearer token, direct-websocket accepts an inbound
// connection via adapter/agentwsserver and hands it to
// AttachInboundSession.
//
// relay-ssh does NOT speak Stack B at all — the design doc's actual
// relay-ssh path is SFTP-deploy-and-launch a `relay.js` binary over SSH,
// then JSON-RPC over its exec channel, and that deploy/launch step remains
// unimplemented (no `relay.js` build artifact reachable from backend-go's
// build — see internal/adapter/sshconn's package doc comment). What IS real
// for relay-ssh, wired in via WithRelaySSH: Health dials the resolved
// domain.SshTarget through sshconn.Connector and runs a trivial command as
// a point-in-time liveness probe (closing the connection after — no
// session reuse, unlike the WS modes' persistent sessions); Exec supports
// ONLY the "shell.exec" method, running the command over the same
// connector-established SSH connection and returning its exit
// outcome — every other method returns a clear, typed error rather than
// silently succeeding, since there is no JSON-RPC agent listening on the
// other end. See this service's README "Known gaps" for the exact
// boundary.
//
// # Two RPC surfaces (unaddressed by this pass)
//
// Even within relay-websocket/direct-websocket, the agent's actual method
// names/param shapes (ports.scan, pty.*, git.*, etc.) are not modeled here —
// Exec is a generic passthrough (method string + params map) with no
// per-method translation layer. See specs/agent/api/agent-rpc-catalog-*.md
// for the real catalog; wiring specific callers (e.g. wscompat's
// devServer.*/fleet.* channels) to specific method names is intentionally
// left to those call sites, not baked into this transport. relay-ssh's
// shell.exec is the one exception — its params shape is pinned to mirror
// workflow-service's actual caller (infrafleetclient.shellExecParams:
// {"script": string, "env": map[string]string]}), since without that this
// mode would have no real caller to match against at all.
package devserveragent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"golang.org/x/crypto/ssh"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// ErrConnectionModeNotImplemented is returned for direct-websocket sessions
// that never attached and for relay-ssh methods other than "shell.exec" (or
// when relay-ssh support wasn't enabled via WithRelaySSH) — see the package
// doc comment.
var ErrConnectionModeNotImplemented = errors.New("devserveragent: connection mode/method not implemented — see package doc comment")

// ErrRelaySSHMethodNotSupported is Exec's typed error for any relay-ssh
// method other than "shell.exec" — there is no JSON-RPC agent listening on
// a relay-ssh connection (no relay.js deployed, see package doc comment),
// so every other method must fail loudly rather than silently succeed or
// return an empty result.
var ErrRelaySSHMethodNotSupported = errors.New(`devserveragent: relay-ssh mode only supports the "shell.exec" method — no JSON-RPC agent (relay.js) is deployed on this connection, see package doc comment`)

// shellExecMethod mirrors infrafleetclient.shellExecMethod
// (workflow-service's real Relay caller for shell steps) — the one relay-ssh
// method this Client actually executes.
const shellExecMethod = "shell.exec"

// shellExecParams mirrors infrafleetclient.shellExecParams field-for-field
// (Script/Env, json tags "script"/"env") — Exec's params argument arrives
// as a decoded map[string]any (see usecase.Relay/the gRPC Relay handler),
// so this type exists to re-marshal+unmarshal into a typed shape rather
// than hand-picking map keys.
type shellExecParams struct {
	Script string            `json:"script"`
	Env    map[string]string `json:"env,omitempty"`
}

// Client implements usecase.DevServerAgentClient for relay-websocket and
// direct-websocket modes for real (persistent, reused sessions — see
// getOrCreateSession), and optionally for relay-ssh's shell.exec/Health via
// WithRelaySSH (a fresh, non-reused SSH connection per call — see Health's
// and Exec's doc comments). A dropped WS session recovers on its own via
// session.go's backgroundReconnect, with getOrCreateSession's lazy redial as
// the fallback for a call that arrives mid-backoff — see both doc comments.
type Client struct {
	cfg    Config
	logger *slog.Logger

	mu       sync.Mutex
	sessions map[string]*session

	// sshConnector and sshTargetResolver are both nil unless WithRelaySSH
	// was passed to New — relay-ssh's Health/Exec behave exactly as before
	// (not implemented) until both are configured.
	sshConnector      *sshconn.Connector
	sshTargetResolver usecase.SshTargetResolver
}

// Option configures optional Client behavior beyond relay-websocket/
// direct-websocket, which need none — currently only WithRelaySSH.
type Option func(*Client)

// WithRelaySSH enables relay-ssh mode's Health (connection probe) and Exec
// ("shell.exec" only) support — see the package doc comment for exactly
// what this does and doesn't cover. resolver looks up a DevServer's
// SSHTargetID into the domain.SshTarget connector dials; typically
// implemented by postgres.SshTargetStore.
func WithRelaySSH(connector *sshconn.Connector, resolver usecase.SshTargetResolver) Option {
	return func(c *Client) {
		c.sshConnector = connector
		c.sshTargetResolver = resolver
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
// connection mode — relay-websocket dials out (and may lazily
// (re)connect here), direct-websocket only ever waits for an
// already-attached inbound session (see AttachInboundSession); Orca cannot
// dial an agent that must dial in.
func (c *Client) getOrCreateSession(ctx context.Context, devServer domain.DevServer) (*session, error) {
	switch devServer.Mode {
	case domain.ConnectionModeRelayWebSocket:
		return c.getOrDialSession(ctx, devServer)
	case domain.ConnectionModeDirectWebSocket:
		return c.getInboundSession(devServer)
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

// AttachInboundSession registers an already-authenticated inbound
// WebSocket connection as devServerID's live session — called by
// adapter/agentwsserver once its handshake + token-slot validation
// succeeds (direct-websocket mode: the agent dialed Orca, not the other
// way around). Reuses the exact same readLoop/keepAliveLoop/call machinery
// connect() uses for the outbound case; only how conn/info was obtained
// differs. Safe to call again for a reconnecting agent — reuses the same
// session object so in-flight callers of Exec/Health naturally see the new
// live connection once attached.
func (c *Client) AttachInboundSession(devServerID, host string, conn *websocket.Conn, info HandshakeInfo) {
	c.mu.Lock()
	sess, ok := c.sessions[devServerID]
	if !ok {
		sess = newSession(host, c.cfg, c.logger)
		sess.inbound = true
		c.sessions[devServerID] = sess
	}
	c.mu.Unlock()

	sess.attachConnection(conn, info)
}

// Exec dispatches one method call to devServer's resolved transport.
// relay-websocket/direct-websocket: a JSON-RPC call (e.g. "ports.scan",
// "preflight.check") over the persistent session, decoded into a map — the
// package doc comment's "method/params are passed through verbatim, no
// per-method translation" contract. relay-ssh: ONLY "shell.exec" is
// supported (see relaySSHExec) — every other method is
// ErrRelaySSHMethodNotSupported, never a silent success/empty result.
func (c *Client) Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error) {
	if devServer.Mode == domain.ConnectionModeRelaySSH {
		return c.relaySSHExec(ctx, devServer, method, params)
	}

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

// relaySSHExec is Exec's relay-ssh path: decode+validate shellExecParams,
// dial the resolved SshTarget, run the script over a fresh SSH connection
// (closed after — see dialRelaySSH's doc comment on why this isn't a
// reused session), and shape the outcome into the same
// {"exitCode","stdout","stderr","error"} result map
// infrafleetclient.execResult expects to decode, regardless of transport.
func (c *Client) relaySSHExec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error) {
	if method != shellExecMethod {
		return nil, fmt.Errorf("%w (got %q)", ErrRelaySSHMethodNotSupported, method)
	}

	var p shellExecParams
	if err := remarshalParams(params, &p); err != nil {
		return nil, fmt.Errorf("devserveragent: decoding %q params: %w", method, err)
	}
	if p.Script == "" {
		return nil, fmt.Errorf(`devserveragent: %q params missing required "script"`, method)
	}

	conn, err := c.dialRelaySSH(ctx, devServer)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	stdout, stderr, runErr := conn.RunCommand(ctx, buildShellCommand(p))
	errMsg := ""
	if runErr != nil {
		errMsg = runErr.Error()
	}
	return map[string]any{
		"exitCode": exitCodeFromRunErr(runErr),
		"stdout":   stdout,
		"stderr":   stderr,
		"error":    errMsg,
	}, nil
}

// Health performs a reachability/liveness check appropriate to devServer's
// transport — distinct from the SSH-exec-based fleet health poll that
// usecase.GetFleetHealth reads from Postgres. A connect failure of any kind
// is reported as (false, nil) across every mode — "not reachable" is the
// expected/common answer this method exists to give, not an error
// condition the caller must additionally branch on.
func (c *Client) Health(ctx context.Context, devServer domain.DevServer) (bool, error) {
	if devServer.Mode == domain.ConnectionModeRelaySSH {
		return c.relaySSHHealth(ctx, devServer), nil
	}
	if devServer.Mode != domain.ConnectionModeRelayWebSocket && devServer.Mode != domain.ConnectionModeDirectWebSocket {
		return false, nil
	}
	sess, err := c.getOrCreateSession(ctx, devServer)
	if err != nil {
		c.logger.DebugContext(ctx, "devserveragent: health check unreachable", slog.String("devServerId", devServer.ID), slog.Any("error", err))
		return false, nil
	}
	return sess.isHandshaked(), nil
}

// relaySSHHealth dials devServer's resolved SshTarget via sshConnector and
// runs a trivial command to prove liveness, closing the connection
// afterward — sshconn has no session-reuse machinery, so this is a
// point-in-time probe, not a persistent check like the WS modes' isHandshaked.
// Any failure (relay-ssh not enabled via WithRelaySSH, target unresolvable,
// dial failure, command failure) reports false, matching Health's "not
// reachable is the expected common answer" contract for every mode.
func (c *Client) relaySSHHealth(ctx context.Context, devServer domain.DevServer) bool {
	conn, err := c.dialRelaySSH(ctx, devServer)
	if err != nil {
		c.logger.DebugContext(ctx, "devserveragent: relay-ssh health check unreachable", slog.String("devServerId", devServer.ID), slog.Any("error", err))
		return false
	}
	defer func() { _ = conn.Close() }()

	if _, _, err := conn.RunCommand(ctx, "true"); err != nil {
		c.logger.DebugContext(ctx, "devserveragent: relay-ssh health check command failed", slog.String("devServerId", devServer.ID), slog.Any("error", err))
		return false
	}
	return true
}

// dialRelaySSH resolves devServer.SSHTargetID via sshTargetResolver and
// dials it via sshConnector — the common setup Health and Exec's
// shell.exec path both need. Returns a clear, wrapped error (never silently
// false/empty) so callers can decide how to surface it: relaySSHHealth
// folds it into false; relaySSHExec propagates it as-is.
func (c *Client) dialRelaySSH(ctx context.Context, devServer domain.DevServer) (*sshconn.Connection, error) {
	if c.sshConnector == nil || c.sshTargetResolver == nil {
		return nil, fmt.Errorf("%w: relay-ssh support was not enabled (see WithRelaySSH)", ErrConnectionModeNotImplemented)
	}
	if devServer.SSHTargetID == "" {
		return nil, fmt.Errorf("devserveragent: dev server %q has no ssh_target_id", devServer.ID)
	}
	target, err := c.sshTargetResolver.Get(ctx, devServer.TenantID, devServer.SSHTargetID)
	if err != nil {
		return nil, fmt.Errorf("devserveragent: resolving ssh target %q: %w", devServer.SSHTargetID, err)
	}
	conn, err := c.sshConnector.Connect(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("devserveragent: dialing relay-ssh target %q: %w", devServer.SSHTargetID, err)
	}
	return conn, nil
}

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

// remarshalParams re-encodes a decoded map[string]any into dest via a JSON
// round trip — Exec's params argument arrives pre-decoded (see
// usecase.Relay/the gRPC Relay handler's json.Unmarshal of params_json), so
// this recovers the typed shellExecParams struct from it.
func remarshalParams(params map[string]any, dest any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

// buildShellCommand turns shellExecParams into the single command string
// RunCommand's session.Run sends over the SSH connection: env vars first,
// as POSIX export lines with the value single-quoted (see shellQuote) so a
// value containing a quote can't break out of the assignment, then the
// script itself. Keys are sorted so the resulting command is deterministic
// (useful for tests and logs), not for any security reason.
func buildShellCommand(p shellExecParams) string {
	var b bytes.Buffer
	for _, name := range slices.Sorted(maps.Keys(p.Env)) {
		fmt.Fprintf(&b, "export %s=%s\n", name, shellQuote(p.Env[name]))
	}
	b.WriteString(p.Script)
	return b.String()
}

// shellQuote wraps s in single quotes for POSIX shell, escaping any
// embedded single quote by closing the quote, emitting an escaped literal
// quote, then reopening the quote.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// exitCodeFromRunErr extracts the remote command's real exit status from a
// RunCommand error when the SSH layer reported one (*ssh.ExitError), falling
// back to 1 for any other failure (e.g. the session/connection itself
// failed) — mirrors infrafleetclient.execResult's `exitCode ?? 0` convention
// applied to the failure side.
func exitCodeFromRunErr(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus()
	}
	return 1
}
