package devserveragent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// handshakeParams mirrors AgentHandshakeParams as sent by
// runOrcaInitiatorHandshake (ws-handshake.ts) — only the orcaVersion field,
// since Orca is the initiator in relay-websocket mode (the agent sends its
// own platform/arch/capabilities back in the RESULT, not the request).
type handshakeParams struct {
	OrcaVersion string `json:"orcaVersion"`
}

// HandshakeInfo mirrors WsHandshakeInfo — what the agent's handshake result
// tells Orca about itself. Exported since adapter/agentwsserver (the
// direct-websocket inbound server) constructs one after its own handshake
// exchange and passes it to Client.AttachInboundSession.
type HandshakeInfo struct {
	Platform     string   `json:"platform"`
	Arch         string   `json:"arch"`
	NodeVersion  string   `json:"nodeVersion"`
	AgentVersion string   `json:"agentVersion"`
	SessionID    string   `json:"sessionId"`
	Capabilities []string `json:"capabilities"`
}

// pendingCall is one in-flight JSON-RPC request awaiting its response.
type pendingCall struct {
	resultCh chan JSONRPCResponse
}

// session is one persistent connection to a single dev server's agent —
// dial (or accept), handshake, read loop, keepalive, and reconnect live
// here, mirroring DevServerRelayBridge.connectRelayWebSocket +
// SshChannelMultiplexer's combined responsibilities. Used for both
// relay-websocket (Orca dials out) and direct-websocket (the agent dials
// in, Orca only accepts) — everything past attachConnection is identical
// either way; only how conn/info was obtained differs, tracked by inbound.
type session struct {
	cfg    Config
	host   string
	logger *slog.Logger

	mu             sync.Mutex
	conn           *websocket.Conn
	handshaked     bool
	handshakeInfo  HandshakeInfo
	nextFrameID    uint32
	nextRequestID  uint32
	highestPeerSeq uint32
	pending        map[uint32]*pendingCall
	closed         bool

	// inbound marks a direct-websocket session (the agent dialed in via
	// adapter/agentwsserver) — backgroundReconnect must not attempt an
	// outbound connect() for these; there is nothing to dial, Orca can only
	// wait for the agent to dial in again.
	inbound bool

	reconnectAttempt int

	closeCh chan struct{}
	closeMu sync.Mutex
}

// wsURL builds ws://host:port/orca-relay — the fixed path
// agent-connection-relay.ts's WebSocketServer listens on.
func (s *session) wsURL() string {
	return fmt.Sprintf("ws://%s:%d/orca-relay", s.host, s.cfg.Port)
}

func newSession(host string, cfg Config, logger *slog.Logger) *session {
	return &session{
		cfg:         cfg,
		host:        host,
		logger:      logger,
		pending:     make(map[uint32]*pendingCall),
		nextFrameID: 1,
		closeCh:     make(chan struct{}),
	}
}

// connect dials the agent and runs the initiator handshake. Safe to call
// again after a disconnect (reconnect path) — each call replaces s.conn.
func (s *session) connect(ctx context.Context) error {
	if s.cfg.Token == "" {
		return fmt.Errorf("devserveragent: ORCA_AGENT_TOKEN is not configured — relay-websocket mode requires it (see specs/agent/api/connection-modes.md §2)")
	}

	dialCtx, cancel := context.WithTimeout(ctx, s.cfg.DialTimeout)
	defer cancel()

	header := http.Header{}
	header.Set("Authorization", "Bearer "+s.cfg.Token)
	conn, _, err := websocket.Dial(dialCtx, s.wsURL(), &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		return fmt.Errorf("devserveragent: dial %s: %w", s.wsURL(), err)
	}

	info, err := s.runInitiatorHandshake(ctx, conn)
	if err != nil {
		conn.Close(websocket.StatusProtocolError, "handshake failed")
		return err
	}

	s.attachConnection(conn, info)
	return nil
}

// attachConnection installs conn as this session's live connection and
// starts the read/keepalive loops — the shared tail of connect() (outbound
// dial, Orca-initiated handshake) and Client.AttachInboundSession
// (direct-websocket mode: conn arrived inbound and was already
// handshake/token-validated by adapter/agentwsserver before this is
// called). Everything past this point — readLoop, keepAliveLoop, call,
// backgroundReconnect — is direction-agnostic; only how conn/info was
// obtained differs.
func (s *session) attachConnection(conn *websocket.Conn, info HandshakeInfo) {
	s.mu.Lock()
	s.conn = conn
	s.handshaked = true
	s.handshakeInfo = info
	s.nextFrameID = 1
	s.nextRequestID = 1
	s.highestPeerSeq = 0
	s.mu.Unlock()

	go s.readLoop(conn)
	go s.keepAliveLoop(conn)
}

// runInitiatorHandshake sends agent.handshake (frame id=1, ack=0, exactly
// as runOrcaInitiatorHandshake does) and waits for the agent's response —
// a one-shot exchange run before the persistent read loop starts, matching
// the TS side's detach-after-settle discipline (see ws-handshake.ts's
// BUG-FE-PTY-001 fix comment: this function owns its own single read here,
// it never leaves a handshake-only listener attached once done).
func (s *session) runInitiatorHandshake(ctx context.Context, conn *websocket.Conn) (HandshakeInfo, error) {
	hctx, cancel := context.WithTimeout(ctx, s.cfg.HandshakeTimeout)
	defer cancel()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "agent.handshake"}
	params, err := json.Marshal(handshakeParams{OrcaVersion: s.cfg.OrcaVersion})
	if err != nil {
		return HandshakeInfo{}, err
	}
	req.Params = params

	frame, err := EncodeJSONRPCFrame(req, 1, 0)
	if err != nil {
		return HandshakeInfo{}, err
	}
	if err := conn.Write(hctx, websocket.MessageBinary, frame); err != nil {
		return HandshakeInfo{}, fmt.Errorf("devserveragent: sending agent.handshake: %w", err)
	}

	for {
		_, data, err := conn.Read(hctx)
		if err != nil {
			return HandshakeInfo{}, fmt.Errorf("devserveragent: handshake: %w", err)
		}
		decoded, err := DecodeFrame(data)
		if err != nil {
			continue // malformed frame — wait for next, same tolerance as the TS decoder
		}
		if decoded.Type != MessageTypeRegular {
			continue // ignore keepalives, matching ws-handshake.ts
		}
		resp, ok, err := ParseJSONRPCResponse(decoded.Payload)
		if err != nil || !ok {
			continue // not a response (or invalid JSON) — wait for next frame
		}
		if resp.Error != nil {
			return HandshakeInfo{}, fmt.Errorf("devserveragent: agent rejected handshake: %s (code: %d)", resp.Error.Message, resp.Error.Code)
		}
		var info HandshakeInfo
		if len(resp.Result) > 0 {
			_ = json.Unmarshal(resp.Result, &info) // best-effort, matching the TS side's per-field `?? default` fallbacks
		}
		if info.Platform == "" {
			info.Platform = "linux"
		}
		return info, nil
	}
}

// readLoop decodes every subsequent frame and routes JSON-RPC responses to
// their pending caller by ID. Runs until the connection closes.
func (s *session) readLoop(conn *websocket.Conn) {
	ctx := context.Background()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			s.handleDisconnect(conn, err)
			return
		}
		decoded, err := DecodeFrame(data)
		if err != nil {
			s.logger.Warn("devserveragent: dropping malformed frame", slog.Any("error", err))
			continue
		}

		s.mu.Lock()
		if decoded.ID > s.highestPeerSeq {
			s.highestPeerSeq = decoded.ID
		}
		s.mu.Unlock()

		if decoded.Type != MessageTypeRegular {
			continue // keepalive — liveness alone (peer-seq bookkeeping above) is enough
		}

		resp, ok, err := ParseJSONRPCResponse(decoded.Payload)
		if err != nil || !ok {
			continue // a notification or malformed payload — this client issues no onRequest/onNotification handlers yet (see README "Known gaps")
		}

		s.mu.Lock()
		call := s.pending[resp.ID]
		delete(s.pending, resp.ID)
		s.mu.Unlock()
		if call != nil {
			call.resultCh <- resp
		}
	}
}

// keepAliveLoop sends a KeepAlive frame every cfg.KeepAliveInterval —
// mirrors SshChannelMultiplexer.startKeepalive(). Exits once the session is
// marked closed.
func (s *session) keepAliveLoop(conn *websocket.Conn) {
	ticker := time.NewTicker(s.cfg.KeepAliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			if s.conn != conn || s.closed {
				s.mu.Unlock()
				return
			}
			id := s.nextFrameID
			s.nextFrameID++
			ack := s.highestPeerSeq
			s.mu.Unlock()

			writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := conn.Write(writeCtx, websocket.MessageBinary, EncodeKeepAliveFrame(id, ack))
			cancel()
			if err != nil {
				return // readLoop's next Read() will observe the same failure and drive reconnect
			}
		case <-s.closeCh:
			return
		}
	}
}

// handleDisconnect fails every pending call on this connection, clears
// session state so the next Call()/Health() triggers a fresh connect(), and
// spawns backgroundReconnect so a dropped session recovers on its own
// instead of staying dead until a caller happens to retry it.
func (s *session) handleDisconnect(conn *websocket.Conn, cause error) {
	s.mu.Lock()
	if s.conn != conn {
		s.mu.Unlock()
		return // already superseded by a newer connection
	}
	s.conn = nil
	s.handshaked = false
	pending := s.pending
	s.pending = make(map[uint32]*pendingCall)
	s.mu.Unlock()

	for _, call := range pending {
		call.resultCh <- JSONRPCResponse{Error: &JSONRPCError{Code: -32000, Message: fmt.Sprintf("devserveragent: connection lost: %v", cause)}}
	}

	go s.backgroundReconnect()
}

// backgroundReconnect retries connect() with backoffDelay-paced attempts
// (mirroring DevServerRelayBridge's exponential-backoff loop) until it
// succeeds or the session is closed. getOrCreateSession's existing lazy
// redial remains the fallback for a call that arrives mid-backoff — this
// loop just means a dropped session doesn't sit dead until one does.
func (s *session) backgroundReconnect() {
	if s.inbound {
		// direct-websocket: there is nothing to dial. The agent owns
		// reconnection on its side (see specs/agent/api/connection-modes.md
		// §7's RECONNECT_DELAYS_MS) and will re-establish via a fresh
		// POST /api/agent-token + inbound handshake, which calls
		// Client.AttachInboundSession directly — bypassing this loop
		// entirely, not resuming it.
		return
	}
	for {
		s.mu.Lock()
		alreadyLive := s.handshaked && s.conn != nil
		closed := s.closed
		attempt := s.reconnectAttempt
		s.mu.Unlock()
		if alreadyLive || closed {
			return
		}

		delay := backoffDelay(s.cfg, attempt)
		select {
		case <-time.After(delay):
		case <-s.closeCh:
			return
		}

		s.mu.Lock()
		if s.closed || (s.handshaked && s.conn != nil) {
			s.mu.Unlock()
			return // superseded by a lazy reconnect (getOrCreateSession) while we waited
		}
		s.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.DialTimeout+s.cfg.HandshakeTimeout)
		err := s.connect(ctx)
		cancel()

		s.mu.Lock()
		if err != nil {
			s.reconnectAttempt++
		} else {
			s.reconnectAttempt = 0
		}
		s.mu.Unlock()

		if err == nil {
			return
		}
		if s.logger != nil {
			s.logger.Warn("devserveragent: background reconnect attempt failed", slog.String("host", s.host), slog.Int("attempt", attempt), slog.Any("error", err))
		}
	}
}

// call sends a JSON-RPC request over the live connection and waits for its
// response, honoring ctx and cfg.RequestTimeout (whichever is shorter).
func (s *session) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	s.mu.Lock()
	if s.conn == nil || !s.handshaked {
		s.mu.Unlock()
		return nil, fmt.Errorf("devserveragent: not connected")
	}
	conn := s.conn
	reqID := s.nextRequestID
	s.nextRequestID++
	frameID := s.nextFrameID
	s.nextFrameID++
	ack := s.highestPeerSeq
	call := &pendingCall{resultCh: make(chan JSONRPCResponse, 1)}
	s.pending[reqID] = call
	s.mu.Unlock()

	var paramsRaw json.RawMessage
	if params != nil {
		encoded, err := json.Marshal(params)
		if err != nil {
			s.dropPending(reqID)
			return nil, err
		}
		paramsRaw = encoded
	}
	req := JSONRPCRequest{JSONRPC: "2.0", ID: reqID, Method: method, Params: paramsRaw}
	frame, err := EncodeJSONRPCFrame(req, frameID, ack)
	if err != nil {
		s.dropPending(reqID)
		return nil, err
	}

	callCtx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
	defer cancel()

	if err := conn.Write(callCtx, websocket.MessageBinary, frame); err != nil {
		s.dropPending(reqID)
		return nil, fmt.Errorf("devserveragent: sending %q: %w", method, err)
	}

	select {
	case resp := <-call.resultCh:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-callCtx.Done():
		s.dropPending(reqID)
		return nil, fmt.Errorf("devserveragent: request %q timed out: %w", method, callCtx.Err())
	}
}

func (s *session) dropPending(id uint32) {
	s.mu.Lock()
	delete(s.pending, id)
	s.mu.Unlock()
}

func (s *session) isHandshaked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.handshaked && s.conn != nil
}

// close tears down the session — used when a devServer is removed or the
// service shuts down.
func (s *session) close() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	conn := s.conn
	s.mu.Unlock()
	close(s.closeCh)
	if conn != nil {
		conn.Close(websocket.StatusNormalClosure, "session closed")
	}
}

// backoffDelay mirrors calcBackoffDelay: 2s * 2^attempt, capped at
// ReconnectMaxDelay, +0-1s jitter.
func backoffDelay(cfg Config, attempt int) time.Duration {
	delay := cfg.ReconnectBaseDelay * time.Duration(1<<uint(attempt))
	if delay > cfg.ReconnectMaxDelay || delay <= 0 {
		delay = cfg.ReconnectMaxDelay
	}
	jitter := time.Duration(rand.Int63n(int64(time.Second)))
	return delay + jitter
}
