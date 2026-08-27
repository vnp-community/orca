package agentwsserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/devserveragent"
)

const (
	// agentHandshakeMethod is the JSON-RPC method the agent's first frame
	// must carry — same wire method name as relay-websocket's own
	// agent.handshake (see devserveragent/session.go), just server role
	// instead of client role here.
	agentHandshakeMethod = "agent.handshake"

	// handshakeTimeout mirrors AGENT_TIMEOUT_MS (20s) — how long Server
	// waits for the agent's first frame to arrive and decode as a valid
	// agent.handshake request.
	handshakeTimeout = 20 * time.Second

	// authFailedCode/authFailedMessage mirror AgentErrorCode.AuthFailed and
	// runOrcaReceiverHandshake's exact rejection message (ws-handshake.ts).
	authFailedCode    = -33101
	authFailedMessage = "Authentication failed: invalid or unregistered agent token"

	base36Chars = "0123456789abcdefghijklmnopqrstuvwxyz"
)

// InboundSessionAttacher is the narrow seam Server needs from
// devserveragent.Client — "accept interfaces, return structs" (already the
// convention this codebase uses elsewhere, e.g. wscompat.SessionValidator).
// Defined here, not in devserveragent, since devserveragent.Client is a
// concrete struct today and this package only ever calls this one method.
type InboundSessionAttacher interface {
	AttachInboundSession(devServerID, host string, conn *websocket.Conn, info devserveragent.HandshakeInfo)
}

// inboundHandshakeParams mirrors AgentHandshakeParams as sent by the agent
// in direct-websocket mode (ws-handshake.ts's runOrcaReceiverHandshake) —
// unlike relay-websocket's handshakeParams (Orca→agent, orcaVersion only),
// this is agent→Orca and carries the bearer token plus the agent's
// self-reported platform/capabilities.
type inboundHandshakeParams struct {
	AgentToken   string   `json:"agentToken"`
	DevServerID  string   `json:"devServerId"`
	Platform     string   `json:"platform"`
	Arch         string   `json:"arch"`
	NodeVersion  string   `json:"nodeVersion"`
	AgentVersion string   `json:"agentVersion"`
	Capabilities []string `json:"capabilities"`
}

// handshakeOKResult mirrors AgentHandshakeResult — the result Orca returns
// on a successful direct-websocket handshake.
type handshakeOKResult struct {
	OK          bool   `json:"ok"`
	OrcaVersion string `json:"orcaVersion"`
	SessionID   string `json:"sessionId"`
}

// Server accepts the agent's inbound WebSocket upgrade (mounted at /agent
// by the composition root) and runs the receiver-side agent.handshake
// exchange — the Go port of AgentWebSocketServer.handleConnection +
// runOrcaReceiverHandshake (agent-ws-server.ts / ws-handshake.ts). On a
// successful, token-validated handshake it hands the live connection to
// Client.AttachInboundSession and is done; everything past that point
// (read/keepalive loops, Exec/Health routing) belongs to devserveragent's
// session, not this package.
type Server struct {
	Registry *Registry
	Client   InboundSessionAttacher
	Cfg      Config
	Logger   *slog.Logger
}

// New constructs a Server.
func New(registry *Registry, client InboundSessionAttacher, cfg Config, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{Registry: registry, Client: client, Cfg: cfg, Logger: logger}
}

// ServeHTTP upgrades the request to a WebSocket and runs the one-shot
// handshake exchange. Mount this at the fixed path "/agent"
// (AGENT_WS_PATH in the TS reference) on the composition root's HTTP mux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// InsecureSkipVerify: no CORS/origin allow-list wired yet in this
	// scaffold pass, matching wscompat.Handler's identical posture — this
	// is an agent dialing in over its own configured URL, not a browser
	// page subject to third-party origin abuse.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		s.logger().ErrorContext(r.Context(), "agentwsserver: ws upgrade failed", slog.Any("error", err))
		return
	}
	s.handleConnection(r.Context(), conn)
}

func (s *Server) logger() *slog.Logger {
	if s.Logger == nil {
		return slog.Default()
	}
	return s.Logger
}

// handleConnection waits for the agent's first frame, requires it to be an
// agent.handshake JSON-RPC request, validates its token against Registry,
// and either rejects (auth failure or protocol violation — no retry, the
// connection is abandoned) or attaches the connection as devServerID's live
// inbound session.
func (s *Server) handleConnection(ctx context.Context, conn *websocket.Conn) {
	hctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()

	_, data, err := conn.Read(hctx)
	if err != nil {
		conn.Close(websocket.StatusPolicyViolation, "Protocol violation")
		return
	}

	frame, err := devserveragent.DecodeFrame(data)
	if err != nil || frame.Type != devserveragent.MessageTypeRegular {
		conn.Close(websocket.StatusPolicyViolation, "Protocol violation")
		return
	}

	var req devserveragent.JSONRPCRequest
	if err := json.Unmarshal(frame.Payload, &req); err != nil || req.Method != agentHandshakeMethod {
		conn.Close(websocket.StatusPolicyViolation, fmt.Sprintf("Protocol violation: first message must be '%s'", agentHandshakeMethod))
		return
	}

	var params inboundHandshakeParams
	if len(req.Params) > 0 {
		_ = json.Unmarshal(req.Params, &params) // best-effort — matches ws-handshake.ts's per-field `?? default` fallbacks
	}

	devServerID, ok := s.Registry.Consume(params.AgentToken)
	if !ok {
		s.rejectHandshake(hctx, conn, req.ID)
		return
	}

	sessionID := newSessionID()
	if err := s.acknowledgeHandshake(hctx, conn, req.ID, sessionID); err != nil {
		conn.Close(websocket.StatusInternalError, "handshake ack failed")
		return
	}

	info := devserveragent.HandshakeInfo{
		Platform:     firstNonEmpty(params.Platform, "linux"),
		Arch:         firstNonEmpty(params.Arch, "x64"),
		NodeVersion:  firstNonEmpty(params.NodeVersion, "unknown"),
		AgentVersion: firstNonEmpty(params.AgentVersion, "unknown"),
		SessionID:    sessionID,
		Capabilities: params.Capabilities,
	}

	// host is irrelevant for inbound sessions — devserveragent never dials
	// out for direct-websocket mode, see AttachInboundSession's doc
	// comment: the agent dialed Orca, not the other way around.
	s.Client.AttachInboundSession(devServerID, "", conn, info)
}

// rejectHandshake sends a JSON-RPC AuthFailed error frame, then closes the
// WS with code 1008 (Policy Violation) — matches runOrcaReceiverHandshake's
// FIX BUG-DS-AWS comment: an explicit 1008 lets the agent's own reconnect
// logic distinguish "token rejected" from a generic drop and force a token
// renewal instead of retrying the same dead token forever.
func (s *Server) rejectHandshake(ctx context.Context, conn *websocket.Conn, requestID uint32) {
	resp := devserveragent.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      requestID,
		Error:   &devserveragent.JSONRPCError{Code: authFailedCode, Message: authFailedMessage},
	}
	if frame, err := devserveragent.EncodeJSONRPCFrame(resp, 1, 0); err == nil {
		_ = conn.Write(ctx, websocket.MessageBinary, frame)
	}
	conn.Close(websocket.StatusPolicyViolation, authFailedMessage)
}

// acknowledgeHandshake sends the {ok:true, orcaVersion, sessionId} success
// result for requestID.
func (s *Server) acknowledgeHandshake(ctx context.Context, conn *websocket.Conn, requestID uint32, sessionID string) error {
	result, err := json.Marshal(handshakeOKResult{OK: true, OrcaVersion: s.Cfg.OrcaVersion, SessionID: sessionID})
	if err != nil {
		return err
	}
	resp := devserveragent.JSONRPCResponse{JSONRPC: "2.0", ID: requestID, Result: result}
	frame, err := devserveragent.EncodeJSONRPCFrame(resp, 1, 0)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageBinary, frame)
}

func firstNonEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// newSessionID builds "sess-<unix-millis>-<6 random base36 chars>",
// matching runOrcaReceiverHandshake's `sess-${Date.now()}-${Math.random()...}`.
func newSessionID() string {
	suffix := make([]byte, 6)
	for i := range suffix {
		suffix[i] = base36Chars[rand.Intn(len(base36Chars))]
	}
	return fmt.Sprintf("sess-%d-%s", time.Now().UnixMilli(), string(suffix))
}
