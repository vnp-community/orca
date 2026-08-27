// Package agentwsserver is infra-fleet-service's INBOUND counterpart to
// internal/adapter/devserveragent's outbound relay-websocket client — this
// is direct-websocket mode: the dev server agent dials INTO Orca, instead
// of Orca dialing out to the agent's own WebSocket server. See
// docs/execution-plan.md §2 Epic A for the three-mode taxonomy
// (direct-websocket / relay-websocket / relay-ssh) and
// devserveragent's package doc comment for why relay-websocket shipped
// first and what direct-websocket additionally required (this package).
//
// # Three pieces, one flow
//
//   - token_endpoint.go's TokenIssuer serves POST/GET /api/agent-token — a
//     deployment script (or an operator) calls POST to mint a single-use
//     agent token and registers it as a pending slot.
//   - slots.go's Registry is that in-memory pending-slot store — one entry
//     per issued-but-not-yet-consumed token, keyed by SHA-256(token) so the
//     plaintext token is never held in the long-lived map (mirrors
//     agent-ws-server.ts's FIX TASK-AWS-002 comment).
//   - server.go's Server serves the agent's inbound WS upgrade at /agent:
//     it waits for the agent's own agent.handshake request, validates its
//     token against the Registry, and on success hands the live connection
//     to devserveragent.Client.AttachInboundSession — which owns every wire
//     concern past that point (read/keepalive loops, Exec/Health routing).
//     This package does not reimplement any of that; it only accepts the
//     upgrade, runs the receiver-side handshake, and validates the token.
//
// Ported line-for-line (protocol shape, error codes, HTTP status/body
// shapes) from the TS reference this mode mirrors: backend/'s
// agent-token-routes.ts and desktop/'s dev-server/agent-ws-server.ts +
// dev-server/ws-handshake.ts's runOrcaReceiverHandshake — see each file's
// own doc comment for the exact correspondence.
package agentwsserver
