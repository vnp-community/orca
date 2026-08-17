// Package devserveragent is infra-fleet-service's outbound adapter to the
// Dev Server Agent execution plane (agent/) — this service's "defining
// adapter" per specs/backend-go/services/infra-fleet-service.md §6, since
// it's the one substantial deviation from the typical
// postgres/vault/eventbus outbound set every other service has.
//
// # THIS IS A STUB — the biggest known gap in this service
//
// Per §10 of the design doc ("Protocol decision: Option A"), this package
// must implement a Go client for the EXISTING TypeScript wire protocol —
// NOT a new gRPC contract:
//
//   - Three connection modes: direct-websocket, relay-websocket, relay-ssh
//     (domain.ConnectionMode).
//   - A 13-byte-framed JSON-RPC transport shared by both agent stacks:
//     [TYPE u8][SEQ u32BE][ACK u32BE][LEN u32BE], documented in
//     specs/agent/api/connection-modes.md and implemented in TS by
//     agent-wire.ts (Stack A) and protocol.ts (Stack B).
//   - Two independently-implemented RPC surfaces on the agent side (Part A:
//     the local WS-connected dispatcher; Part B: the SSH-deployed
//     "Orca Relay" RelayDispatcher) that frequently diverge in method names
//     and param shapes for the same nominal operation (e.g. pty.create vs.
//     pty.spawn) — see specs/agent/api/README.md and
//     specs/agent/api/gaps-and-findings.md. A real implementation must model
//     this as two distinct method-call surfaces behind one
//     usecase.DevServerAgentClient interface, not a single flat namespace.
//
// Implementing that transport (frame codec, JSON-RPC 2.0 framing, three
// connection-mode transports, the Stack A/B handshake split, SFTP-based
// relay-binary deployment for relay-ssh) is a substantial standalone effort
// — the design doc sketches it as its own sub-package tree (wire/, directws/,
// relayws/, relayssh/, handshake.go, methods.go). It is intentionally NOT
// built in this scaffold. Every Client method below returns ErrNotImplemented
// instead. See this service's README "Known gaps" section and
// specs/backend-go/architecture/08-inter-service-communication.md Option A
// for what a real implementation must do.
//
// The port this Client implements (usecase.DevServerAgentClient) is defined
// in internal/usecase, not here — see that package's ports.go doc comment
// for why.
package devserveragent

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ErrNotImplemented is returned by every Client method. See the package doc
// comment for what a real implementation requires.
var ErrNotImplemented = errors.New("devserveragent: not implemented — see specs/backend-go/architecture/08-inter-service-communication.md Option A (existing TS wire protocol: 13-byte-framed JSON-RPC over relay-ssh/relay-websocket/direct-websocket, see specs/agent/api/connection-modes.md)")

// Client is a stub implementation of usecase.DevServerAgentClient — see the
// package doc comment above. It compiles and wires cleanly into this
// service's composition root (cmd/server/main.go) so the shape of the
// dependency graph is correct even though the transport itself does nothing
// yet.
type Client struct{}

// New constructs a Client. Takes no arguments today because there is no
// transport to configure yet — a real implementation will need per-mode
// dial/listen configuration (direct-websocket target URL, relay-websocket
// listener, relay-ssh SshTarget + SshChannelMultiplexer), see the package
// doc comment.
func New() *Client {
	return &Client{}
}

// Exec would dispatch one JSON-RPC method call (e.g. "ports.scan",
// "pty.spawn"/"pty.create" depending on which agent stack devServer's
// transport mode reaches, "preflight.check") to the Dev Server Agent over
// devServer's resolved connection mode, and decode its JSON-RPC result.
func (c *Client) Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error) {
	return nil, ErrNotImplemented
}

// Health would perform an agent-level reachability/handshake check
// (agent.handshake for direct-websocket, the Stack B version handshake for
// relay-ssh/relay-websocket) — distinct from the SSH-exec-based fleet
// health poll that usecase.GetFleetHealth reads from Postgres.
func (c *Client) Health(ctx context.Context, devServer domain.DevServer) (bool, error) {
	return false, ErrNotImplemented
}
