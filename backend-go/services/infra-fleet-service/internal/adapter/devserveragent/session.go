package devserveragent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ErrRelayDetachedProcessGone is devserveragent's local sentinel for
// sshrelay.ErrDetachedProcessGone — wrapped at the adapter boundary by
// SshReattacher implementations (see adapter/sshrelay.Provisioner.Reattach)
// so this package never has to import adapter/sshrelay directly (wrong
// dependency direction per this codebase's Dependency Inversion convention).
var ErrRelayDetachedProcessGone = errors.New("devserveragent: relay-ssh detached process is no longer running")

// SshReattacher is relay-ssh's background-reconnect port, mirroring
// SshProvisioner's shape — implemented by adapter/sshrelay.Provisioner.
type SshReattacher interface {
	Reattach(ctx context.Context, devServer domain.DevServer, sockPath string) (Transport, error)
}

// handshakeParams mirrors AgentHandshakeParams as sent by
// runOrcaInitiatorHandshake (ws-handshake.ts) — only the orcaVersion field,
// since Orca is the initiator in relay-websocket mode (the agent sends its
// own platform/arch/capabilities back in the RESULT, not the request).
type handshakeParams struct {
	OrcaVersion string `json:"orcaVersion"`
}

// HandshakeInfo mirrors WsHandshakeInfo — what the agent's handshake result
// tells Orca about itself. Exported since adapter/agentwsserver (the
// direct-websocket inbound server) and adapter/sshrelay (relay-ssh's
// deploy+launch provisioner) both construct one after their own handshake
// exchange and pass it to Client.AttachInboundSession /
// Client.SshProvisioner.Provision's return value respectively.
type HandshakeInfo struct {
	Platform     string   `json:"platform"`
	Arch         string   `json:"arch"`
	NodeVersion  string   `json:"nodeVersion"`
	AgentVersion string   `json:"agentVersion"`
	SessionID    string   `json:"sessionId"`
	Capabilities []string `json:"capabilities"`
	// SockPath is relay-ssh mode's Unix socket path for the detached agent
	// process (see adapter/sshrelay's launch/reattach — SOL-SSH-03), cached
	// on *session so relaySSHReconnect can call reattach() again without
	// re-resolving the SshTarget or re-deploying. Empty for
	// relay-websocket/direct-websocket, which have no detached process.
	SockPath string `json:"-"`
}

// pendingCall is one in-flight JSON-RPC request awaiting its response.
type pendingCall struct {
	resultCh chan JSONRPCResponse
}

// session is one persistent connection to a single dev server's agent —
// dial (or accept, or provision), handshake, read loop, keepalive, and
// reconnect live here, mirroring DevServerRelayBridge.connectRelayWebSocket
// + SshChannelMultiplexer's combined responsibilities. Used for all three
// connection modes via the Transport abstraction (transport.go):
// relay-websocket dials out over a wsTransport, direct-websocket accepts an
// inbound wsTransport, relay-ssh gets a Transport from adapter/sshrelay's
// SSH-exec-channel deploy+launch pipeline — everything past
// attachTransport is identical across all three; only how the Transport was
// obtained differs, tracked by managedMode.
type managedMode int

const (
	managedModeNone             managedMode = iota // relay-websocket: backgroundReconnect dials as before
	managedModeInboundOnly                          // direct-websocket: agent re-dials on its own
	managedModeRelaySSHReattach                     // relay-ssh: relaySSHReconnect (reattach, not redeploy)
)

type session struct {
	cfg    Config
	host   string
	logger *slog.Logger

	mu             sync.Mutex
	transport      Transport
	handshaked     bool
	handshakeInfo  HandshakeInfo
	nextFrameID    uint32
	nextRequestID  uint32
	highestPeerSeq uint32
	pending        map[uint32]*pendingCall
	closed         bool

	// managedMode marks a session this package doesn't reconnect the plain
	// relay-websocket way — direct-websocket's inbound accept (the agent
	// must dial in again) and relay-ssh's reattach-to-a-detached-process
	// loop (relaySSHReconnect), as opposed to relay-websocket's plain
	// backgroundReconnect redial.
	managedMode managedMode
	// relaySSHDevServer/reattacher are set only for managedModeRelaySSHReattach
	// sessions (by Client.getOrProvisionSession) — relaySSHReconnect's inputs.
	relaySSHDevServer domain.DevServer
	reattacher        SshReattacher

	// ptyMu/ptySubs implement the notification demux TASK-183 adds: routing
	// pty.data/pty.exit/pty.replay notifications (see routeNotification) to
	// whichever StreamPty caller subscribed for a given pty id. Kept as its
	// own mutex, not s.mu, so routing a notification never contends with
	// call()/readLoop's request-response bookkeeping.
	ptyMu   sync.Mutex
	ptySubs map[string][]chan rawPtyNotification

	// hookMu/hookSubs implement TASK-AG-03-03's agent.hook fan-out — unkeyed
	// (there is no ptyId/session correlation key on the wire yet, see
	// TASK-AG-03-07), so every subscriber on this session gets every
	// agent.hook notification, unlike ptySubs's per-pty-id keying.
	hookMu   sync.Mutex
	hookSubs []chan rawAgentHookNotification

	reconnectAttempt int

	closeCh chan struct{}
	closeMu sync.Mutex

	// tokenSource resolves this session's current relay-websocket bearer
	// token, re-invoked on every (re)connect so a token rotated between
	// attempts is picked up with no process restart — set by Client at
	// newSession time for relay-websocket only; nil for
	// direct-websocket/relay-ssh, which never call connect(). See
	// TASK-AWS-01-03/SOL-AWS-01.
	tokenSource func(ctx context.Context) (string, error)
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

// connect dials the agent and runs the initiator handshake using token
// (resolved per-dial by the caller — see Client.AgentTokenSource). Safe to
// call again after a disconnect (reconnect path). relay-websocket only —
// direct-websocket/relay-ssh sessions never call this, see
// attachTransport's callers.
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

// attachTransport installs t as this session's live transport and starts
// the read/keepalive loops — the shared tail every connection-establishment
// path (outbound dial, inbound accept, SSH-exec provision) funnels through.
// Everything past this point — readLoop, keepAliveLoop, call,
// backgroundReconnect — is transport-agnostic; only how t/info was obtained
// differs.
func (s *session) attachTransport(t Transport, info HandshakeInfo) {
	s.mu.Lock()
	s.transport = t
	s.handshaked = true
	s.handshakeInfo = info
	s.nextFrameID = 1
	s.nextRequestID = 1
	s.highestPeerSeq = 0
	s.mu.Unlock()

	go s.readLoop(t)
	go s.keepAliveLoop(t)
}

// runInitiatorHandshake sends agent.handshake (frame id=1, ack=0, exactly
// as runOrcaInitiatorHandshake does) and waits for the agent's response —
// a one-shot exchange run before the persistent read loop starts, matching
// the TS side's detach-after-settle discipline (see ws-handshake.ts's
// BUG-FE-PTY-001 fix comment: this function owns its own single read here,
// it never leaves a handshake-only listener attached once done). Operates
// on the raw *websocket.Conn directly (not yet wrapped as a Transport) —
// relay-websocket only, the one mode where Orca is the handshake initiator.
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
// their pending caller by ID. Runs until the transport errors/closes.
func (s *session) readLoop(t Transport) {
	ctx := context.Background()
	for {
		decoded, err := t.ReadFrame(ctx)
		if err != nil {
			s.handleDisconnect(t, err)
			return
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
		if err == nil && ok {
			s.mu.Lock()
			call := s.pending[resp.ID]
			delete(s.pending, resp.ID)
			s.mu.Unlock()
			if call != nil {
				call.resultCh <- resp
			}
			continue
		}

		// Not a response — route pty.data/pty.exit/pty.replay notifications
		// (TASK-183's notification demux, see routeNotification). Anything
		// else (malformed payload, a notification method this client
		// doesn't demux) is dropped — this client issues no general-purpose
		// onRequest/onNotification handlers yet (see README "Known gaps").
		if notif, isNotif, nerr := ParseJSONRPCNotification(decoded.Payload); nerr == nil && isNotif {
			s.routeNotification(notif)
		}
	}
}

// rawPtyNotification is session.go's internal decoding of one
// pty.data/pty.exit/pty.replay notification — StreamPty (client.go) wraps
// this into the exported usecase.PtyEvent shape.
type rawPtyNotification struct {
	PtyID    string
	Data     []byte
	Exited   bool
	ExitCode int32
}

// ptyNotificationParams is this adapter's best-effort decoding of
// pty.data/pty.exit/pty.replay notification params.
//
// FLAGGED (TASK-183): the exact field names were not confirmed against
// pty-daemon-server.ts's/pty-handler.ts's actual notify() call sites for
// these three methods specifically — out of this pass's scope/budget (the
// TASK-183 spec file itself was also not present in this worktree, see this
// package's README/the implementing PR's description). "id" (the pty id)
// mirrors pty.write/pty.resize/pty.destroy's confirmed {id, ...} params
// shape; "data" and "exitCode" are the most likely field names given
// relay-protocol.ts's general JSON-RPC conventions elsewhere in this
// adapter, but are UNCONFIRMED. Treat as a best-effort default, not a
// verified contract, until checked against a real agent build.
type ptyNotificationParams struct {
	ID       string `json:"id"`
	Data     string `json:"data"`
	ExitCode int32  `json:"exitCode"`
}

// routeNotification demuxes an incoming notification by method — pty.*
// notifications route by pty id (routePtyNotification), agent.hook fans out
// to every subscriber on this session (routeAgentHookNotification, TASK-AG-03-03,
// unkeyed — see hookSubs's doc comment).
func (s *session) routeNotification(n JSONRPCNotification) {
	switch n.Method {
	case "pty.data", "pty.exit", "pty.replay":
		s.routePtyNotification(n)
	case "agent.hook":
		s.routeAgentHookNotification(n)
	default:
		return // not a notification this client demuxes, see package doc comment's "Two RPC surfaces" note
	}
}

// routePtyNotification decodes and fans out one pty.* notification to every
// subscriber currently registered for its pty id (subscribePty/StreamPty).
// A slow subscriber never blocks the read loop — see the non-blocking send
// below, matching this adapter's "the read loop must never block on a
// consumer" discipline (see keepAliveLoop's write-timeout, handleDisconnect's
// unblocking of pending calls).
func (s *session) routePtyNotification(n JSONRPCNotification) {
	var p ptyNotificationParams
	if len(n.Params) > 0 {
		_ = json.Unmarshal(n.Params, &p) // best-effort, see ptyNotificationParams's FLAGGED doc comment
	}
	if p.ID == "" {
		return
	}

	raw := rawPtyNotification{PtyID: p.ID}
	if n.Method == "pty.exit" {
		raw.Exited = true
		raw.ExitCode = p.ExitCode
	} else {
		raw.Data = []byte(p.Data)
	}

	s.ptyMu.Lock()
	subs := append([]chan rawPtyNotification(nil), s.ptySubs[p.ID]...)
	s.ptyMu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- raw:
		default: // slow/gone consumer — drop rather than block the read loop
		}
	}
}

// subscribePty registers a new listener for ptyID's notifications —
// StreamPty's implementation. The returned channel is buffered so a burst of
// output doesn't immediately trip routeNotification's drop-on-full path.
func (s *session) subscribePty(ptyID string) chan rawPtyNotification {
	ch := make(chan rawPtyNotification, 64)
	s.ptyMu.Lock()
	if s.ptySubs == nil {
		s.ptySubs = make(map[string][]chan rawPtyNotification)
	}
	s.ptySubs[ptyID] = append(s.ptySubs[ptyID], ch)
	s.ptyMu.Unlock()
	return ch
}

// unsubscribePty removes and closes ch — MUST be called exactly once by
// whoever called subscribePty (see StreamPty's returned unsubscribe func).
func (s *session) unsubscribePty(ptyID string, ch chan rawPtyNotification) {
	s.ptyMu.Lock()
	subs := s.ptySubs[ptyID]
	for i, c := range subs {
		if c == ch {
			s.ptySubs[ptyID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(s.ptySubs[ptyID]) == 0 {
		delete(s.ptySubs, ptyID)
	}
	s.ptyMu.Unlock()
	close(ch)
}

// rawAgentHookNotification is session.go's internal decoding of one
// agent.hook notification — RecordAgentHookProviderSession (usecase layer)
// wraps this into AgentHookEvent. providerSession fields are empty when the
// hook event carried none (not every hook fires one).
type rawAgentHookNotification struct {
	WorktreeID         string
	PtyID              string
	ProviderSessionKey string
	ProviderSessionID  string
}

// agentHookNotificationParams mirrors agent-hook-server.ts's
// AgentHookRelayEnvelope's fields this client cares about — worktreeId for
// the correlation fallback (TASK-AG-03-05), ptyId for the exact-match
// correlation TASK-AG-03-07 added (empty on older agent builds mid-rollout
// — RecordAgentHookProviderSession.Handle falls back to worktreeId when so),
// providerSession.{key,id} for what to persist.
type agentHookNotificationParams struct {
	WorktreeID      string `json:"worktreeId"`
	PtyID           string `json:"ptyId"`
	ProviderSession *struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	} `json:"providerSession"`
}

// routeAgentHookNotification decodes and fans out one agent.hook
// notification to every subscriber on this session — unkeyed, see
// hookSubs's doc comment.
func (s *session) routeAgentHookNotification(n JSONRPCNotification) {
	var p agentHookNotificationParams
	if len(n.Params) > 0 {
		_ = json.Unmarshal(n.Params, &p)
	}
	raw := rawAgentHookNotification{WorktreeID: p.WorktreeID, PtyID: p.PtyID}
	if p.ProviderSession != nil {
		raw.ProviderSessionKey = p.ProviderSession.Key
		raw.ProviderSessionID = p.ProviderSession.ID
	}

	s.hookMu.Lock()
	subs := append([]chan rawAgentHookNotification(nil), s.hookSubs...)
	s.hookMu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- raw:
		default: // slow/gone consumer — drop rather than block the read loop
		}
	}
}

// subscribeAgentHooks registers a new listener for every agent.hook
// notification on this session (unkeyed — see routeAgentHookNotification's
// doc comment). Exactly one long-lived subscriber per devServer connection
// is expected (usecase.RecordAgentHookProviderSession), not one per
// AgentSession.
func (s *session) subscribeAgentHooks() chan rawAgentHookNotification {
	ch := make(chan rawAgentHookNotification, 64)
	s.hookMu.Lock()
	s.hookSubs = append(s.hookSubs, ch)
	s.hookMu.Unlock()
	return ch
}

// unsubscribeAgentHooks removes and closes ch — MUST be called exactly once
// by whoever called subscribeAgentHooks.
func (s *session) unsubscribeAgentHooks(ch chan rawAgentHookNotification) {
	s.hookMu.Lock()
	for i, c := range s.hookSubs {
		if c == ch {
			s.hookSubs = append(s.hookSubs[:i], s.hookSubs[i+1:]...)
			break
		}
	}
	s.hookMu.Unlock()
	close(ch)
}

// keepAliveLoop sends a KeepAlive frame every cfg.KeepAliveInterval —
// mirrors SshChannelMultiplexer.startKeepalive(). Exits once the session is
// marked closed or superseded by a newer transport.
func (s *session) keepAliveLoop(t Transport) {
	ticker := time.NewTicker(s.cfg.KeepAliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			if s.transport != t || s.closed {
				s.mu.Unlock()
				return
			}
			id := s.nextFrameID
			s.nextFrameID++
			ack := s.highestPeerSeq
			s.mu.Unlock()

			writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := t.WriteFrame(writeCtx, EncodeKeepAliveFrame(id, ack))
			cancel()
			if err != nil {
				return // readLoop's next ReadFrame() will observe the same failure and drive reconnect
			}
		case <-s.closeCh:
			return
		}
	}
}

// handleDisconnect fails every pending call on this transport, clears
// session state so the next Call()/Health() triggers a fresh
// connect()/provision, and spawns backgroundReconnect so a dropped session
// recovers on its own instead of staying dead until a caller happens to
// retry it.
func (s *session) handleDisconnect(t Transport, cause error) {
	s.mu.Lock()
	if s.transport != t {
		s.mu.Unlock()
		return // already superseded by a newer transport
	}
	s.transport = nil
	s.handshaked = false
	pending := s.pending
	s.pending = make(map[uint32]*pendingCall)
	s.mu.Unlock()

	for _, call := range pending {
		call.resultCh <- JSONRPCResponse{Error: &JSONRPCError{Code: -32000, Message: fmt.Sprintf("devserveragent: connection lost: %v", cause)}}
	}

	go s.backgroundReconnect()
}

// backgroundReconnect dispatches by managedMode: relay-websocket retries
// connect() itself (below); direct-websocket is a true no-op (the agent
// re-dials on its own — specs/agent/api/connection-modes.md §7's
// RECONNECT_DELAYS_MS — and re-attaches via Client.AttachInboundSession);
// relay-ssh runs relaySSHReconnect, a real reconnect loop now that the
// detached process (SOL-SSH-03) survives an SSH drop.
func (s *session) backgroundReconnect() {
	s.mu.Lock()
	mode := s.managedMode
	s.mu.Unlock()
	switch mode {
	case managedModeInboundOnly:
		return // agent must dial in again — see AttachInboundSession
	case managedModeRelaySSHReattach:
		s.relaySSHReconnect()
		return
	}
	s.backgroundReconnectRelayWebSocket()
}

// backgroundReconnectRelayWebSocket retries connect() with
// backoffDelay-paced attempts (mirroring DevServerRelayBridge's
// exponential-backoff loop) until it succeeds or the session is closed.
// getOrCreateSession's existing lazy redial remains the fallback for a call
// that arrives mid-backoff — this loop just means a dropped session doesn't
// sit dead until one does. relay-websocket only.
func (s *session) backgroundReconnectRelayWebSocket() {
	for {
		s.mu.Lock()
		alreadyLive := s.handshaked && s.transport != nil
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
		if s.closed || (s.handshaked && s.transport != nil) {
			s.mu.Unlock()
			return // superseded by a lazy reconnect (getOrCreateSession) while we waited
		}
		s.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.DialTimeout+s.cfg.HandshakeTimeout)
		token := ""
		var err error
		if s.tokenSource != nil {
			token, err = s.tokenSource(ctx)
		}
		if err == nil {
			err = s.connect(ctx, token)
		}
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

// relaySSHReconnect mirrors backgroundReconnectRelayWebSocket's loop
// structure exactly (same backoffDelay call, same closed/superseded checks)
// but calls reattacher.Reattach instead of connect() — the detached agent
// process's in-memory state (its AgentSession, its pty-daemon children)
// survived the SSH drop untouched, only the bridge died, so no fresh
// agent.handshake is needed: this just confirms the bridge is live and
// reuses the HandshakeInfo captured at first Provision (cached via
// s.handshakeInfo).
func (s *session) relaySSHReconnect() {
	for {
		s.mu.Lock()
		alreadyLive := s.handshaked && s.transport != nil
		closed := s.closed
		attempt := s.reconnectAttempt
		sockPath := s.handshakeInfo.SockPath
		devServer := s.relaySSHDevServer
		reattacher := s.reattacher
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
		if s.closed || (s.handshaked && s.transport != nil) {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.DialTimeout+s.cfg.HandshakeTimeout)
		conn, err := reattacher.Reattach(ctx, devServer, sockPath)
		cancel()

		if errors.Is(err, ErrRelayDetachedProcessGone) {
			// Detached process itself is gone — reattach can never succeed
			// again; the next Exec/Health call's getOrProvisionSession will
			// do a full re-Provision (redeploy+relaunch) once this loop exits.
			return
		}

		s.mu.Lock()
		if err != nil {
			s.reconnectAttempt++
		} else {
			s.reconnectAttempt = 0
		}
		s.mu.Unlock()

		if err == nil {
			s.attachTransport(conn, s.handshakeInfo) // reuse cached info, only transport/liveness changed
			return
		}
		if s.logger != nil {
			s.logger.Warn("devserveragent: relay-ssh reattach attempt failed", slog.String("host", s.host), slog.Int("attempt", attempt), slog.Any("error", err))
		}
	}
}

// cancelReconnect unblocks a waiting backgroundReconnectRelayWebSocket/
// relaySSHReconnect loop without closing the session outright —
// TeardownConnection's cancel path (BR-SSH-13). Safe to call when no
// reconnect loop is running. Reuses closeMu the same way close() does to
// guard against re-closing an already-closed channel.
func (s *session) cancelReconnect() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	select {
	case <-s.closeCh:
		return // already closed/cancelled
	default:
		close(s.closeCh)
		s.mu.Lock()
		s.closeCh = make(chan struct{}) // replace so a FUTURE reconnect loop (next drop) gets a fresh channel
		s.mu.Unlock()
	}
}

// call sends a JSON-RPC request over the live transport and waits for its
// response, honoring ctx and cfg.RequestTimeout (whichever is shorter).
func (s *session) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	s.mu.Lock()
	if s.transport == nil || !s.handshaked {
		s.mu.Unlock()
		return nil, fmt.Errorf("devserveragent: not connected")
	}
	t := s.transport
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

	if err := t.WriteFrame(callCtx, frame); err != nil {
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
	return s.handshaked && s.transport != nil
}

// handshakeInfoSnapshot returns the HandshakeInfo captured at the most
// recent attachTransport, if this session has completed one and is still
// live — shared by Client.LastHandshakeInfo (SOL-FLEET-04) and
// Client.HandshakeInfoFor (TASK-INT-03-02), both narrow wrappers over this
// same session-local read.
func (s *session) handshakeInfoSnapshot() (HandshakeInfo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.handshaked || s.transport == nil {
		return HandshakeInfo{}, false
	}
	return s.handshakeInfo, true
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
	t := s.transport
	s.mu.Unlock()
	close(s.closeCh)
	if t != nil {
		_ = t.Close("session closed")
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
