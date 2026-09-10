package devserveragent

import (
	"context"
	"encoding/base64"
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
// Exactly one of resultCh/streamCh is set: resultCh for an ordinary call()
// (exactly one response frame, then removed from session.pending by
// readLoop itself); streamCh for a callStream() call (TASK-PW-03-08) —
// every response frame sharing this request id is forwarded there until a
// terminal frame is observed (isTerminalStreamResponse), at which point
// readLoop removes the pending entry AND closes streamCh. readLoop is the
// only goroutine that ever writes to or closes streamCh — see callStream's
// doc comment for why the unsubscribe side deliberately never closes it.
type pendingCall struct {
	resultCh chan JSONRPCResponse
	streamCh chan JSONRPCResponse
	// streaming marks a call whose response arrives as MULTIPLE frames
	// sharing this call's request id (vm.provision's stream.started/
	// stream.chunk/stream.end shape — see streamCall's doc comment). readLoop
	// must not auto-delete this pending entry after its first frame the way
	// it does for an ordinary call() — the caller (streamCall/its consumer)
	// owns calling the returned complete func exactly once, on the terminal
	// frame or early cancellation.
	streaming bool
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
	managedModeInboundOnly                         // direct-websocket: agent re-dials on its own
	managedModeRelaySSHReattach                    // relay-ssh: relaySSHReconnect (reattach, not redeploy)
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

	// screencastMu/screencastSubs is the same demux pattern as ptyMu/ptySubs,
	// for browser.screencastReady/Frame/Ended/Error notifications
	// (StreamScreencast) — keyed by worktree_id rather than a pty id, since
	// (unlike a pty, which already exists by the time StreamPty subscribes)
	// a screencast's subscription_id/browser_page_id are only assigned by
	// the agent's browser.screencastReady response, so worktree_id (known
	// up front, caller-supplied) is the only viable subscribe-before-call
	// correlation key. Own mutex for the same never-contend-with-call()
	// reason ptyMu has its own.
	screencastMu   sync.Mutex
	screencastSubs map[string][]chan rawScreencastNotification

	// fileWatchMu/fileWatchSubs is the same demux pattern again, for
	// fs.changed notifications (StreamFileChanges, BACKLOG-003) — keyed by
	// the watched absolute path, exactly what fs.watch/fs.unwatch/fs.changed
	// all key by agent-side (agent/src/relay/fs-agent-extensions.ts's
	// AGENT_WATCH_MAP), so no separate id-assignment step exists to race the
	// way screencastSubs's subscription_id does. Own mutex for the same
	// never-contend-with-call() reason ptyMu/screencastMu have their own.
	fileWatchMu   sync.Mutex
	fileWatchSubs map[string][]chan rawFileWatchNotification

	// execOutputMu/execOutputSubs is the same demux pattern as ptyMu/ptySubs
	// and screencastMu/screencastSubs — a third copy (TASK-AG-FLOWTASK-002),
	// for agent.execPrompt.output notifications (agent-print-mode-exec.ts's
	// handleAgentExecPrompt, wired to makeNotifier by TASK-AGENT-TASKV1-01/02
	// — NOT the "agent.execOutput" name SOL-AG-FLOWTASK-001 §2.1 sketched;
	// this demux matches the real agent/ method name, confirmed by reading
	// agent-print-mode-exec.ts directly rather than the design doc's guess).
	// Keyed by stepId (agent.execPrompt's caller-supplied correlation id),
	// not a pty/worktree id — own mutex, same never-contend-with-call()
	// reason ptyMu/screencastMu each have their own.
	execOutputMu   sync.Mutex
	execOutputSubs map[string][]chan rawExecOutputNotification

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
// their pending caller by ID. Runs until the transport errors/closes, or
// cfg.IdleTimeout elapses with no frame received at all (including
// keepalives) — see the fresh per-iteration deadline below.
//
// Why a per-iteration context.WithTimeout, not one long-lived context plus a
// separate watchdog goroutine comparing timestamps: Transport.ReadFrame's
// own doc comment guarantees it returns once ctx is cancelled, and
// wsTransport delegates straight to coder/websocket's conn.Read(ctx), which
// already does exactly this (cancel → Read errors, connection torn down) —
// no extra goroutine, no extra mutex-protected "last frame at" field, no
// second code path to keep in sync with handleDisconnect. Was previously a
// gap the agent side (Phase 8) had already fixed and documented as "must
// terminate(), never rely on close() eventually timing out" — this closes
// the mirror-image gap on the backend-go side of the same connection.
func (s *session) readLoop(t Transport) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.IdleTimeout)
		decoded, err := t.ReadFrame(ctx)
		cancel()
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
			if call != nil && call.streamCh != nil {
				terminal := isTerminalStreamResponse(resp)
				if terminal {
					delete(s.pending, resp.ID)
				}
				s.mu.Unlock()
				select {
				case call.streamCh <- resp:
				default: // slow/gone consumer — drop rather than block the read loop
				}
				if terminal {
					close(call.streamCh)
				}
				continue
			}
			// A streaming call's pending entry stays registered across
			// multiple response frames — only an ordinary (non-streaming)
			// call is auto-cleared on its one response. An error response is
			// always terminal even for a streaming call (e.g. vm.provision
			// dispatch-time "method not found": the dispatcher never even
			// sends 'stream.started', see streamCall's doc comment), so it
			// clears the entry regardless of streaming.
			if call != nil && (resp.Error != nil || !call.streaming) {
				delete(s.pending, resp.ID)
			}
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
// unkeyed — see hookSubs's doc comment), browser.screencast* routes by
// worktree id (routeScreencastNotification, StreamScreencast), and
// fs.changed routes by watched path (routeFileWatchNotification,
// StreamFileChanges).
func (s *session) routeNotification(n JSONRPCNotification) {
	switch n.Method {
	case "pty.data", "pty.exit", "pty.replay":
		s.routePtyNotification(n)
	case "agent.hook":
		s.routeAgentHookNotification(n)
	case "browser.screencastReady", "browser.screencastFrame", "browser.screencastEnded", "browser.screencastError":
		s.routeScreencastNotification(n)
	case "fs.changed":
		s.routeFileWatchNotification(n)
	case "agent.execPrompt.output":
		// TASK-AG-FLOWTASK-002 Open Question 3: exact match, not a
		// strings.HasPrefix("agent.execOutput", ...) like SOL-AG-FLOWTASK-001
		// §2.2 sketched — the real agent/ notification family also includes
		// "shell.exec.output" (fs-agent-extensions.ts's handleShellExec,
		// keyed by traceId, a different workflow-service consumer than this
		// stepId-keyed one), which a broad "agent.exec*" prefix would have
		// wrongly captured here. Only agent.execPrompt.output is routed by
		// this task's scope (task-service's SimpleExecutor/Engine 1) —
		// shell.exec.output has no consumer wired yet and is intentionally
		// left to the silent default below, not a false demux.
		s.routeExecOutputNotification(n)
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

// rawScreencastNotification is session.go's internal decoding of one
// browser.screencastReady/Frame/Ended/Error notification — StreamScreencast
// (client.go) wraps this into the exported usecase.ScreencastEvent shape.
// Exactly one of Ready/Frame/Ended/ErrorMsg is meaningfully set per value,
// matching rawPtyNotification's "one raw struct, caller narrows by which
// notification method produced it" convention.
type rawScreencastNotification struct {
	Ready          bool
	SubscriptionID string
	BrowserPageID  string
	Format         string
	Frame          []byte
	Ended          bool
	ErrorMsg       string
}

// screencastNotificationParams is this adapter's decoding of
// browser.screencastReady/Frame/Ended/Error notification params — unlike
// ptyNotificationParams, this shape is NOT a best-effort guess: both this
// adapter and agent/src/relay/browser-screencast-handler.ts (this same
// implementation pass) were written together, so the field names below are
// the actual, verified contract, not a FLAGGED placeholder.
type screencastNotificationParams struct {
	WorktreeID     string `json:"worktreeId"`
	SubscriptionID string `json:"subscriptionId"`
	BrowserPageID  string `json:"browserPageId"`
	Format         string `json:"format"`
	DataBase64     string `json:"dataBase64"`
	Message        string `json:"message"`
}

// routeScreencastNotification is routeNotification's screencast counterpart
// — same demux-by-correlation-key-then-non-blocking-fanout shape as
// routePtyNotification, keyed by worktree_id (see screencastSubs's doc
// comment for why).
func (s *session) routeScreencastNotification(n JSONRPCNotification) {
	var p screencastNotificationParams
	if len(n.Params) > 0 {
		_ = json.Unmarshal(n.Params, &p)
	}
	if p.WorktreeID == "" {
		return
	}

	raw := rawScreencastNotification{}
	switch n.Method {
	case "browser.screencastReady":
		raw.Ready = true
		raw.SubscriptionID = p.SubscriptionID
		raw.BrowserPageID = p.BrowserPageID
		raw.Format = p.Format
	case "browser.screencastFrame":
		decoded, err := base64.StdEncoding.DecodeString(p.DataBase64)
		if err != nil {
			return // malformed frame — drop rather than forward garbage bytes
		}
		raw.Frame = decoded
	case "browser.screencastEnded":
		raw.Ended = true
	case "browser.screencastError":
		raw.ErrorMsg = p.Message
	}

	s.screencastMu.Lock()
	subs := append([]chan rawScreencastNotification(nil), s.screencastSubs[p.WorktreeID]...)
	s.screencastMu.Unlock()

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

// subscribeScreencast registers a new listener for worktreeID's screencast
// notifications — StreamScreencast's implementation. MUST be called BEFORE
// issuing the browser.screencastStart call (see StreamScreencast) so a fast
// agent response can never arrive before the subscription exists.
func (s *session) subscribeScreencast(worktreeID string) chan rawScreencastNotification {
	ch := make(chan rawScreencastNotification, 64)
	s.screencastMu.Lock()
	if s.screencastSubs == nil {
		s.screencastSubs = make(map[string][]chan rawScreencastNotification)
	}
	s.screencastSubs[worktreeID] = append(s.screencastSubs[worktreeID], ch)
	s.screencastMu.Unlock()
	return ch
}

// unsubscribeScreencast removes and closes ch — MUST be called exactly once
// by whoever called subscribeScreencast (see StreamScreencast's returned
// unsubscribe func).
func (s *session) unsubscribeScreencast(worktreeID string, ch chan rawScreencastNotification) {
	s.screencastMu.Lock()
	subs := s.screencastSubs[worktreeID]
	for i, c := range subs {
		if c == ch {
			s.screencastSubs[worktreeID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(s.screencastSubs[worktreeID]) == 0 {
		delete(s.screencastSubs, worktreeID)
	}
	s.screencastMu.Unlock()
	close(ch)
}

// rawFileWatchNotification is session.go's internal decoding of one
// fs.changed notification — StreamFileChanges (client.go) wraps this into
// the exported usecase.FileChangeEvent shape. Path is the watched ROOT
// (what fs.watch was called with, and what fileWatchSubs is keyed by, not
// the individual changed file) — see fsChangedParams's doc comment.
type rawFileWatchNotification struct {
	Path     string
	Kind     string // "create" | "update" | "delete" | "rename" | "overflow"
	Filename string // root-relative, "" when the root itself is the changed entry
}

// fsChangedParams is fs.changed's real wire shape — confirmed against
// agent/src/relay/fs-agent-extensions.ts's handleFsWatch/
// handleLinuxWatchEvent notify() call sites (unlike ptyNotificationParams,
// this one IS verified, not a best-effort guess).
type fsChangedParams struct {
	Path      string `json:"path"`      // the watched root, same value fs.watch({path}) was called with
	EventType string `json:"eventType"` // "rename" | "change" | "error" — Node fs.watch's own two kinds, plus the agent's synthesized "error"
	Filename  string `json:"filename"`  // root-relative path of the changed entry, "" when unknown/root itself
	Error     string `json:"error"`     // set only when eventType == "error"
}

// routeFileWatchNotification decodes one fs.changed notification and fans
// it out to every subscriber registered for its watched root path — see
// routePtyNotification's doc comment for the same non-blocking-send
// discipline. Kind mapping mirrors the legacy Node backend's own
// battle-tested approximation (backend/src/main/providers/
// dev-server-filesystem-provider.ts's watchViaPush): Node's fs.watch only
// ever reports "rename" (create/delete/rename, indistinguishable) or
// "change" (content) — not the finer create/update/delete split
// FileChangeEvent.kind's shape otherwise suggests — so "rename" maps to
// "rename" and everything else maps to "update"; "error" maps to the
// dedicated "overflow" kind (matching the legacy mapping's own choice: no
// FsChangeEvent.kind value means "watch itself broke", "overflow" is the
// closest existing one and every caller already treats it as "give up on
// deltas, do a full re-read").
func (s *session) routeFileWatchNotification(n JSONRPCNotification) {
	var p fsChangedParams
	if len(n.Params) > 0 {
		_ = json.Unmarshal(n.Params, &p)
	}
	if p.Path == "" {
		return
	}

	raw := rawFileWatchNotification{Path: p.Path, Filename: p.Filename}
	switch {
	case p.EventType == "error":
		raw.Kind = "overflow"
	case p.EventType == "rename":
		raw.Kind = "rename"
	default:
		raw.Kind = "update"
	}

	s.fileWatchMu.Lock()
	subs := append([]chan rawFileWatchNotification(nil), s.fileWatchSubs[p.Path]...)
	s.fileWatchMu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- raw:
		default: // slow/gone consumer — drop rather than block the read loop
		}
	}
}

// rawExecOutputNotification is session.go's internal decoding of one
// agent.execPrompt.output notification — Client.StreamExecOutput (client.go)
// wraps this into the exported usecase.ExecOutputEvent shape. Unlike
// rawPtyNotification/rawScreencastNotification, every field is always
// meaningful (there is no exit/ended variant of this notification — the
// unary agent.execPrompt response itself carries exitCode/timedOut).
type rawExecOutputNotification struct {
	StepID string
	Stream string // "stdout" | "stderr"
	Data   string
}

// execOutputNotificationParams is this adapter's decoding of
// agent.execPrompt.output's params — field names match
// agent-print-mode-exec.ts's handleAgentExecPrompt's notify?.(
// 'agent.execPrompt.output', { stepId, stream, data }) call exactly (both
// sides read directly from the real TS source for this pass, not guessed
// the way ptyNotificationParams's FLAGGED comment documents for TASK-183).
type execOutputNotificationParams struct {
	StepID string `json:"stepId"`
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

// routeExecOutputNotification is routeNotification's agent.execPrompt.output
// counterpart — same demux-by-correlation-key-then-non-blocking-fanout
// shape as routePtyNotification/routeScreencastNotification, keyed by
// stepId.
func (s *session) routeExecOutputNotification(n JSONRPCNotification) {
	var p execOutputNotificationParams
	if len(n.Params) > 0 {
		_ = json.Unmarshal(n.Params, &p)
	}
	if p.StepID == "" {
		return // no correlation key to demux on — drop, same posture as routePtyNotification's empty-ID case
	}

	raw := rawExecOutputNotification{StepID: p.StepID, Stream: p.Stream, Data: p.Data}

	s.execOutputMu.Lock()
	subs := append([]chan rawExecOutputNotification(nil), s.execOutputSubs[p.StepID]...)
	s.execOutputMu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- raw:
		default: // slow/gone consumer — drop rather than block the read loop
		}
	}
}

// subscribeFileWatch registers a new listener for path's fs.changed
// notifications — StreamFileChanges's implementation. MUST be called BEFORE
// issuing the fs.watch call (same subscribe-before-call discipline
// StreamScreencast's doc comment explains) so a fast agent notification can
// never arrive before the subscription exists.
func (s *session) subscribeFileWatch(path string) chan rawFileWatchNotification {
	ch := make(chan rawFileWatchNotification, 64)
	s.fileWatchMu.Lock()
	if s.fileWatchSubs == nil {
		s.fileWatchSubs = make(map[string][]chan rawFileWatchNotification)
	}
	s.fileWatchSubs[path] = append(s.fileWatchSubs[path], ch)
	s.fileWatchMu.Unlock()
	return ch
}

// unsubscribeFileWatch removes and closes ch — MUST be called exactly once
// by whoever called subscribeFileWatch (see StreamFileChanges's returned
// unsubscribe func).
func (s *session) unsubscribeFileWatch(path string, ch chan rawFileWatchNotification) {
	s.fileWatchMu.Lock()
	subs := s.fileWatchSubs[path]
	for i, c := range subs {
		if c == ch {
			s.fileWatchSubs[path] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(s.fileWatchSubs[path]) == 0 {
		delete(s.fileWatchSubs, path)
	}
	s.fileWatchMu.Unlock()
	close(ch)
}

// subscribeExecOutput registers a new listener for stepID's
// agent.execPrompt.output notifications — Client.StreamExecOutput's
// implementation. The returned channel is buffered so a burst of output
// doesn't immediately trip routeExecOutputNotification's drop-on-full path
// — same buffer size as subscribePty/subscribeScreencast, no evidence yet
// this RPC's chunk frequency needs a different bound (TASK-AG-FLOWTASK-002
// Open Question 1: revisit once real traffic data exists).
func (s *session) subscribeExecOutput(stepID string) chan rawExecOutputNotification {
	ch := make(chan rawExecOutputNotification, 64)
	s.execOutputMu.Lock()
	if s.execOutputSubs == nil {
		s.execOutputSubs = make(map[string][]chan rawExecOutputNotification)
	}
	s.execOutputSubs[stepID] = append(s.execOutputSubs[stepID], ch)
	s.execOutputMu.Unlock()
	return ch
}

// unsubscribeExecOutput removes and closes ch — MUST be called exactly once
// by whoever called subscribeExecOutput (see Client.StreamExecOutput's
// returned unsubscribe func).
func (s *session) unsubscribeExecOutput(stepID string, ch chan rawExecOutputNotification) {
	s.execOutputMu.Lock()
	subs := s.execOutputSubs[stepID]
	for i, c := range subs {
		if c == ch {
			s.execOutputSubs[stepID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(s.execOutputSubs[stepID]) == 0 {
		delete(s.execOutputSubs, stepID)
	}
	s.execOutputMu.Unlock()
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

	lostErr := JSONRPCResponse{Error: &JSONRPCError{Code: -32000, Message: fmt.Sprintf("devserveragent: connection lost: %v", cause)}}
	for _, call := range pending {
		if call.streamCh != nil {
			// callStream callers select on ctx.Done() alongside reading this
			// channel (see devserveragent.Client.ExecStream) — a non-blocking
			// send-then-close here mirrors routeNotification's "never block
			// the read loop on a consumer" discipline, and this is the
			// terminal frame for this stream: readLoop won't observe another
			// one on a torn-down transport.
			select {
			case call.streamCh <- lostErr:
			default:
			}
			close(call.streamCh)
			continue
		}
		call.resultCh <- lostErr
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

// callStream sends one JSON-RPC request the same way call() does, but
// registers a streamCh instead of a resultCh so every response frame
// sharing this request id — not just the first — is forwarded to the
// returned channel (TASK-PW-03-08's ExecStream/git.execStream support; see
// specs/agent/api/agent-rpc-catalog-git-fs.md's "git.execStream streaming
// shape" section: multiple frames replying to one request id, terminated
// by a stream.end-typed frame).
//
// Unlike call(), this has no per-call RequestTimeout — a git push/pull can
// run arbitrarily long — so the caller's ctx is the only cancellation path
// (only used to size the initial WriteFrame's deadline via the transport;
// the returned channel keeps delivering frames until readLoop observes the
// terminal frame or the session itself closes).
//
// The returned unsubscribe func only removes the pending-map entry (so a
// caller that loses interest early stops being routed future frames); it
// deliberately does NOT close the returned channel itself — only readLoop
// (session.go's single reader/writer of streamCh) ever closes it, avoiding
// the send-on-closed-channel race a consumer-side close would risk.
func (s *session) callStream(ctx context.Context, method string, params any) (<-chan JSONRPCResponse, func(), error) {
	s.mu.Lock()
	if s.transport == nil || !s.handshaked {
		s.mu.Unlock()
		return nil, nil, fmt.Errorf("devserveragent: not connected")
	}
	t := s.transport
	reqID := s.nextRequestID
	s.nextRequestID++
	frameID := s.nextFrameID
	s.nextFrameID++
	ack := s.highestPeerSeq
	call := &pendingCall{streamCh: make(chan JSONRPCResponse, 256)}
	s.pending[reqID] = call
	s.mu.Unlock()

	var paramsRaw json.RawMessage
	if params != nil {
		encoded, err := json.Marshal(params)
		if err != nil {
			s.dropPending(reqID)
			return nil, nil, err
		}
		paramsRaw = encoded
	}
	req := JSONRPCRequest{JSONRPC: "2.0", ID: reqID, Method: method, Params: paramsRaw}
	frame, err := EncodeJSONRPCFrame(req, frameID, ack)
	if err != nil {
		s.dropPending(reqID)
		return nil, nil, err
	}

	if err := t.WriteFrame(ctx, frame); err != nil {
		s.dropPending(reqID)
		return nil, nil, fmt.Errorf("devserveragent: sending %q: %w", method, err)
	}

	unsubscribe := func() { s.dropPending(reqID) }
	return call.streamCh, unsubscribe, nil
}

// isTerminalStreamResponse decides whether resp ends a callStream() call —
// either an agent-level JSON-RPC error, or a decoded result whose "type"
// field is "stream.end" (git.execStream's own terminator, per
// specs/agent/api/agent-rpc-catalog-git-fs.md). FLAGGED: this "type" probe
// is this adapter's best-effort reading of the doc's
// {type:'stream.chunk',...}/{type:'stream.end',...} shape, same
// unconfirmed-against-a-real-build caveat as ptyNotificationParams above.
func isTerminalStreamResponse(resp JSONRPCResponse) bool {
	if resp.Error != nil {
		return true
	}
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(resp.Result, &probe); err != nil {
		return false
	}
	return probe.Type == "stream.end"
}

// streamCall sends method and blocks for exactly the FIRST response frame —
// mirroring call()'s wait — then hands back a channel for every SUBSEQUENT
// frame sharing this request's id, until the caller invokes the returned
// complete func exactly once (on a terminal frame or early cancellation),
// matching StreamPty/StreamScreencast's subscribe/unsubscribe contract.
//
// This is vm.provision's real wire shape (agent-ephemeral-vm-handler.ts's
// handleVmProvision, mirroring agent-git-handler.ts's handleGitExecStream —
// the confirmed precedent): the RPC dispatcher answers the original request
// id immediately with {result:{type:'stream.started'}} BEFORE the handler
// even starts running, then the handler pushes zero-or-more
// {result:{type:'stream.chunk',...}} frames and exactly one terminal
// {result:{type:'stream.end',...}} frame — all sharing that same id. This is
// NOT the pty.data/browser.screencastReady notification demux shape (a
// separate method+params push with no id) that subscribePty/
// subscribeScreencast handle — it's multiple RESPONSE frames for one
// request, which ordinary call() cannot receive (its pendingCall is deleted
// after the first frame). See pendingCall.streaming's doc comment for the
// readLoop-side half of this.
func (s *session) streamCall(ctx context.Context, method string, params any) (firstResult json.RawMessage, rest <-chan JSONRPCResponse, complete func(), err error) {
	s.mu.Lock()
	if s.transport == nil || !s.handshaked {
		s.mu.Unlock()
		return nil, nil, nil, fmt.Errorf("devserveragent: not connected")
	}
	t := s.transport
	reqID := s.nextRequestID
	s.nextRequestID++
	frameID := s.nextFrameID
	s.nextFrameID++
	ack := s.highestPeerSeq
	call := &pendingCall{resultCh: make(chan JSONRPCResponse, 256), streaming: true}
	s.pending[reqID] = call
	s.mu.Unlock()

	complete = func() {
		s.mu.Lock()
		if s.pending[reqID] == call {
			delete(s.pending, reqID)
		}
		s.mu.Unlock()
	}

	var paramsRaw json.RawMessage
	if params != nil {
		encoded, encErr := json.Marshal(params)
		if encErr != nil {
			complete()
			return nil, nil, nil, encErr
		}
		paramsRaw = encoded
	}
	req := JSONRPCRequest{JSONRPC: "2.0", ID: reqID, Method: method, Params: paramsRaw}
	frame, encErr := EncodeJSONRPCFrame(req, frameID, ack)
	if encErr != nil {
		complete()
		return nil, nil, nil, encErr
	}

	callCtx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
	defer cancel()

	if writeErr := t.WriteFrame(callCtx, frame); writeErr != nil {
		complete()
		return nil, nil, nil, fmt.Errorf("devserveragent: sending %q: %w", method, writeErr)
	}

	select {
	case resp := <-call.resultCh:
		if resp.Error != nil {
			complete() // no-op if readLoop already cleared it (it does, for an error frame)
			return nil, nil, nil, resp.Error
		}
		return resp.Result, call.resultCh, complete, nil
	case <-callCtx.Done():
		complete()
		return nil, nil, nil, fmt.Errorf("devserveragent: request %q timed out: %w", method, callCtx.Err())
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
