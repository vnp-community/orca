package mcpserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// This file replaces mcp.NewStreamableHTTPHandler (ADR-MCP-001 gap (a)): the
// SDK keeps sessions in process memory with no store hook, so the gateway owns
// the session table. Each replica holds live SDK connections ("local
// sessions") for the sessions it has served; a session known only to the
// durable SessionStore is ADOPTED on first use by rebuilding the SDK session
// from the stored initialize state. The Mcp-Session-Id is a bearer secret:
// only its SHA-256 reaches the store, NATS subjects use a derived key.

const (
	sessionCacheTTL = 5 * time.Second
	touchInterval   = 30 * time.Second
	storeCallLimit  = 3 * time.Second
	headerSessionID = "Mcp-Session-Id"
	headerProtocol  = "MCP-Protocol-Version"
)

type localSession struct {
	secret           string
	tenantID, userID string
	ss               *mcp.ServerSession
	conn             *obsConn
	tr               *mcp.StreamableServerTransport
	done             chan struct{}
	closeOnce        sync.Once

	mu         sync.Mutex
	inflight   map[uint64]context.CancelFunc // contexts of running requests
	nextReq    uint64
	closing    bool
	rowID      string
	ready      bool
	cachedAt   time.Time
	lastActive time.Time
	lastTouch  time.Time
	calls      int64
}

// track returns a context cancelled when the session is closed.
func (l *localSession) track(ctx context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(ctx)
	l.mu.Lock()
	if l.closing {
		l.mu.Unlock()
		cancel()
		return ctx, func() {}
	}
	if l.inflight == nil {
		l.inflight = map[uint64]context.CancelFunc{}
	}
	l.nextReq++
	id := l.nextReq
	l.inflight[id] = cancel
	l.mu.Unlock()
	return ctx, func() {
		l.mu.Lock()
		delete(l.inflight, id)
		l.mu.Unlock()
		cancel()
	}
}

func (l *localSession) cancelInflight() {
	l.mu.Lock()
	l.closing = true
	fns := make([]context.CancelFunc, 0, len(l.inflight))
	for _, f := range l.inflight {
		fns = append(fns, f)
	}
	l.mu.Unlock()
	for _, f := range fns {
		f()
	}
}

func (l *localSession) row() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rowID
}

func (l *localSession) principal() Principal {
	return Principal{TenantID: l.tenantID, UserID: l.userID}
}

type sessionHost struct {
	d         Deps
	cfg       Config
	log       *slog.Logger
	rec       Recorder
	srec      SessionRecorder
	store     SessionStore
	registry  StreamRegistry
	resum     ResumableEventStore
	sdkEvents mcp.EventStore
	bus       SignalBus
	replicaID string
	now       func() time.Time
	server    *mcp.Server
	ready     *readySessions
	getID     func() string

	mu       sync.Mutex
	bySecret map[string]*localSession
	byRow    map[string]*localSession
	adoptMu  sync.Mutex

	wg      sync.WaitGroup
	stop    chan struct{}
	unsubs  []func()
	stopped sync.Once
}

// keepOnClose makes the SDK's per-connection SessionClosed a no-op: one
// replica closing ITS connection (idle eviction) must not wipe the shared
// resume buffer of a session that is still alive elsewhere.
type keepOnClose struct{ ResumableEventStore }

func (keepOnClose) SessionClosed(context.Context, string) error { return nil }

func newSessionHost(h *Handler) *sessionHost {
	d := h.d
	s := &sessionHost{d: d, cfg: h.cfg, log: h.log, rec: h.rec, store: d.Sessions, bus: d.Signals,
		now: d.Now, getID: d.GetSessionID, bySecret: map[string]*localSession{}, byRow: map[string]*localSession{}, stop: make(chan struct{})}
	if s.now == nil {
		s.now = time.Now
	}
	if s.getID == nil {
		s.getID = rand.Text
	}
	if s.store == nil {
		s.store = NewMemorySessionStore(h.cfg.SessionIdleTTL, s.now)
	}
	s.registry, _ = s.store.(StreamRegistry)
	s.srec, _ = h.rec.(SessionRecorder)
	var b [6]byte
	_, _ = rand.Read(b[:])
	s.replicaID = hex.EncodeToString(b[:])

	events := d.EventStore
	if events == nil {
		events = NewMemoryResumableStore(0, 0)
	}
	if rs, ok := events.(ResumableEventStore); ok {
		s.resum, s.sdkEvents = rs, keepOnClose{rs}
	} else {
		s.sdkEvents = events // SDK-native replay on this replica only
	}
	return s
}

// start wires the signal subscriptions and the local janitor; the SDK server
// must be set first.
func (s *sessionHost) start() {
	if s.bus != nil {
		if u, err := s.bus.Subscribe(signalSessionPrefix+">", s.onSessionSignal); err == nil {
			s.unsubs = append(s.unsubs, u)
		} else {
			s.log.Warn("mcp session signals unavailable: cancel/close will only reach this replica", slog.Any("error", err))
		}
		if u, err := s.bus.Subscribe(signalTenantPrefix+">", s.onTenantSignal); err == nil {
			s.unsubs = append(s.unsubs, u)
		}
	}
	s.wg.Add(1)
	go s.janitor()
}

// Close stops background work and closes every local session; it cancels
// their in-flight tool calls. Safe to call twice.
func (s *sessionHost) Close() {
	s.stopped.Do(func() {
		close(s.stop)
		for _, u := range s.unsubs {
			u()
		}
		s.mu.Lock()
		all := make([]*localSession, 0, len(s.bySecret))
		for _, ls := range s.bySecret {
			all = append(all, ls)
		}
		s.mu.Unlock()
		for _, ls := range all {
			s.closeLocal(ls, "shutdown", false)
		}
		s.wg.Wait()
	})
}

func (s *sessionHost) janitor() {
	defer s.wg.Done()
	every := s.cfg.SessionIdleTTL / 4
	if every > time.Minute {
		every = time.Minute
	}
	if every < time.Second {
		every = time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.evictIdle()
		}
	}
}

// evictIdle drops idle LOCAL connections to bound memory. The session itself
// stays valid in the store and is adopted again on its next request; only the
// store's idle TTL ends a session.
func (s *sessionHost) evictIdle() {
	s.mu.Lock()
	var idle []*localSession
	for _, ls := range s.bySecret {
		ls.mu.Lock()
		if s.now().Sub(ls.lastActive) > s.cfg.SessionIdleTTL && ls.conn.Inflight() == 0 {
			idle = append(idle, ls)
		}
		ls.mu.Unlock()
	}
	s.mu.Unlock()
	for _, ls := range idle {
		s.closeLocal(ls, CloseIdle, false)
	}
}

func (s *sessionHost) local(secret string) *localSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bySecret[secret]
}

func (s *sessionHost) localByRow(id string) *localSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byRow[id]
}

// ActiveLocalSessions is the number of live SDK connections on this replica.
func (s *sessionHost) ActiveLocalSessions() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.bySecret)
}

func (s *sessionHost) unregister(ls *localSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bySecret[ls.secret] == ls {
		delete(s.bySecret, ls.secret)
	}
	if id := ls.row(); id != "" && s.byRow[id] == ls {
		delete(s.byRow, id)
	}
}

func (s *sessionHost) setRow(ls *localSession, id string) {
	ls.mu.Lock()
	ls.rowID = id
	ls.mu.Unlock()
	s.mu.Lock()
	s.byRow[id] = ls
	s.mu.Unlock()
}

// closeLocal tears down this replica's connection: the SDK cancels the
// contexts of in-flight requests, so running tools stop. purge also drops the
// shared resume buffer and must only be used for authoritative closes.
func (s *sessionHost) closeLocal(ls *localSession, reason string, purge bool) {
	ls.closeOnce.Do(func() {
		close(ls.done)
		ls.cancelInflight()
		s.unregister(ls)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			_ = ls.ss.Close()
			if purge && s.resum != nil {
				ctx, cancel := context.WithTimeout(context.Background(), storeCallLimit)
				defer cancel()
				_ = s.resum.Purge(ctx, ls.secret)
			}
		}()
		if s.srec != nil {
			s.srec.SessionClosed(reason)
		}
	})
}

func (s *sessionHost) startLocal(ctx context.Context, secret string, p Principal, state *mcp.ServerSessionState) (*localSession, error) {
	inner := &mcp.StreamableServerTransport{SessionID: secret, EventStore: s.sdkEvents}
	ot := &obsTransport{inner: inner, rec: s.srec, now: s.now}
	ls := &localSession{secret: secret, tenantID: p.TenantID, userID: p.UserID, tr: inner, done: make(chan struct{}),
		lastActive: s.now(), cachedAt: s.now()}
	// Register BEFORE connecting: the first request is handled as soon as the
	// transport is served, and the receiving middleware looks the session up.
	s.mu.Lock()
	s.bySecret[secret] = ls
	s.mu.Unlock()
	ss, err := s.server.Connect(context.WithoutCancel(ctx), ot, &mcp.ServerSessionOptions{State: state})
	if err != nil {
		s.unregister(ls)
		return nil, err
	}
	ls.ss, ls.conn = ss, ot.conn
	s.wg.Add(1)
	go func() { // the SDK ends the connection on client/transport close
		defer s.wg.Done()
		_ = ss.Wait()
		s.closeLocal(ls, "connection_closed", false)
	}()
	return ls, nil
}

func (s *sessionHost) adopt(ctx context.Context, p Principal, secret string, rec SessionRecord) (*localSession, error) {
	s.adoptMu.Lock()
	defer s.adoptMu.Unlock()
	if ls := s.local(secret); ls != nil {
		return ls, nil
	}
	var caps mcp.ClientCapabilities
	if len(rec.Capabilities) > 0 {
		_ = json.Unmarshal(rec.Capabilities, &caps)
	}
	level := mcp.LoggingLevel(rec.LogLevel)
	if level == "" {
		level = "warning"
	}
	state := &mcp.ServerSessionState{
		InitializeParams: &mcp.InitializeParams{ProtocolVersion: rec.ProtocolVersion, Capabilities: &caps,
			ClientInfo: &mcp.Implementation{Name: rec.ClientName, Version: rec.ClientVersion}},
		NegotiatedProtocolVersion: rec.ProtocolVersion, LogLevel: level,
	}
	if rec.State == SessionReady {
		state.InitializedParams = &mcp.InitializedParams{}
	}
	ls, err := s.startLocal(ctx, secret, p, state)
	if err != nil {
		return nil, err
	}
	ls.ready = rec.State == SessionReady
	if ls.ready {
		s.ready.mark(secret) // the handshake already happened on another replica
	}
	if s.srec != nil {
		s.srec.SessionOpened()
	}
	return ls, nil
}

// storeCtx bounds a store call and keeps it alive past request cancellation.
func storeCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), storeCallLimit)
}

// resolve finds the live session for a request, enforcing identity binding:
// a session belongs to ONE (tenant, user); anyone else gets 404, exactly like
// an unknown id.
func (s *sessionHost) resolve(ctx context.Context, p Principal, secret string) (*localSession, int) {
	ls := s.local(secret)
	if ls != nil && (ls.tenantID != p.TenantID || ls.userID != p.UserID) {
		s.rec.IdentityMismatch()
		return nil, http.StatusNotFound
	}
	if ls != nil {
		ls.mu.Lock()
		fresh := s.now().Sub(ls.cachedAt) < sessionCacheTTL
		ls.mu.Unlock()
		if fresh {
			return ls, 0
		}
	}
	sctx, cancel := storeCtx(ctx)
	defer cancel()
	rec, err := s.store.Lookup(sctx, p, SecretHash(secret))
	switch {
	case errors.Is(err, ErrSessionNotFound):
		if ls != nil {
			s.sessionEnded(ls, CloseIdle)
			s.closeLocal(ls, CloseIdle, false)
		}
		return nil, http.StatusNotFound
	case err != nil:
		s.log.WarnContext(ctx, "mcp session lookup failed", slog.Any("error", err))
		return nil, http.StatusServiceUnavailable
	case rec.TenantID != p.TenantID || rec.UserID != p.UserID:
		s.rec.IdentityMismatch()
		return nil, http.StatusNotFound
	case rec.State == SessionClosed:
		if ls != nil {
			s.sessionEnded(ls, CloseIdle)
			s.closeLocal(ls, CloseIdle, false)
		}
		return nil, http.StatusNotFound
	}
	if ls == nil {
		if ls, err = s.adopt(ctx, p, secret, rec); err != nil {
			s.log.WarnContext(ctx, "mcp session adoption failed", slog.Any("error", err))
			return nil, http.StatusServiceUnavailable
		}
	}
	s.setRow(ls, rec.ID)
	ls.mu.Lock()
	ls.cachedAt = s.now()
	ls.mu.Unlock()
	return ls, 0
}

// touch records activity (at most every touchInterval, or when forced).
// closed=true means the store ended the session.
func (s *sessionHost) touch(ctx context.Context, p Principal, ls *localSession, force bool) (closed bool) {
	ls.mu.Lock()
	ls.lastActive = s.now()
	id := ls.rowID
	every := touchInterval
	if q := s.cfg.SessionIdleTTL / 4; q < every {
		every = q
	}
	if id == "" || (!force && s.now().Sub(ls.lastTouch) < every) {
		ls.mu.Unlock()
		return false
	}
	ls.lastTouch = s.now()
	delta, ready := ls.calls, ls.ready
	ls.calls = 0
	ls.mu.Unlock()
	sctx, cancel := storeCtx(ctx)
	defer cancel()
	st, err := s.store.Touch(sctx, p, id, SessionTouch{Ready: ready, ToolCallsDelta: delta})
	if errors.Is(err, ErrSessionNotFound) || st == SessionClosed {
		s.sessionEnded(ls, CloseIdle)
		s.closeLocal(ls, CloseIdle, false)
		return true
	}
	if err != nil {
		s.log.WarnContext(ctx, "mcp session touch failed", slog.Any("error", err))
	}
	return false
}

func (s *sessionHost) newSecret() string { return s.getID() }

func supportedProtocol(v string) bool { return slices.Contains(SupportedProtocolVersions, v) }

func baseMedia(v string) string {
	m, _, err := mime.ParseMediaType(v)
	if err != nil {
		return ""
	}
	return m
}

func accepts(values []string) (jsonOK, streamOK bool) {
	for _, v := range values {
		for _, raw := range strings.Split(v, ",") {
			base, _, _ := strings.Cut(strings.TrimSpace(raw), ";")
			switch strings.ToLower(strings.TrimSpace(base)) {
			case "application/json", "application/*":
				jsonOK = true
			case "text/event-stream", "text/*":
				streamOK = true
			case "*/*":
				jsonOK, streamOK = true, true
			}
		}
	}
	return
}

func (s *sessionHost) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, ok := PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if v := r.Header.Get(headerProtocol); v != "" && !supportedProtocol(v) {
		http.Error(w, "Bad Request: Unsupported protocol version (supported versions: "+strings.Join(SupportedProtocolVersions, ",")+")", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodPost, http.MethodDelete:
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	secret := r.Header.Get(headerSessionID)
	if secret == "" {
		if r.Method != http.MethodPost {
			http.Error(w, "Bad Request: "+r.Method+" requires an Mcp-Session-Id header", http.StatusBadRequest)
			return
		}
		if !s.checkPOST(w, r) {
			return
		}
		s.createAndServe(w, r)
		return
	}
	ls, code := s.resolve(r.Context(), p, secret)
	if ls == nil {
		if code == http.StatusServiceUnavailable {
			w.Header().Set("Retry-After", "2")
			http.Error(w, "session store unavailable", code)
			return
		}
		http.Error(w, "session not found", code)
		return
	}
	if s.killed(w, r, p, ls) {
		return
	}
	if r.Method != http.MethodDelete && s.touch(r.Context(), p, ls, false) {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodPost:
		if !s.checkPOST(w, r) {
			return
		}
		ls.tr.ServeHTTP(w, r)
	case http.MethodGet:
		s.serveGET(w, r, p, ls)
	case http.MethodDelete:
		s.serveDelete(w, r, p, ls)
	}
}

// killed answers 403 MCP_KILL_SWITCH_ACTIVE when an administrator stopped
// this session (the tenant/client/grant scopes are checked by the verifier).
func (s *sessionHost) killed(w http.ResponseWriter, r *http.Request, p Principal, ls *localSession) bool {
	if s.d.KillCheck == nil || ls.row() == "" {
		return false
	}
	blocked, err := s.d.KillCheck(r.Context(), p, ls.row())
	switch {
	case err != nil:
		s.rec.AuthFailure("verifier_unavailable")
		w.Header().Set("Retry-After", "2")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return true
	case blocked:
		s.rec.AuthFailure("kill_switch")
		writeKillSwitch(w)
		return true
	}
	return false
}

// checkPOST mirrors the SDK's transport validation and additionally refuses
// JSON-RPC batches (removed in 2025-06-18; the SDK would accept them because
// we cannot hand it the negotiated version).
func (s *sessionHost) checkPOST(w http.ResponseWriter, r *http.Request) bool {
	if baseMedia(r.Header.Get("Content-Type")) != "application/json" {
		http.Error(w, "Content-Type must be 'application/json'", http.StatusUnsupportedMediaType)
		return false
	}
	jsonOK, streamOK := accepts(r.Header.Values("Accept"))
	if !jsonOK || !streamOK {
		http.Error(w, "Accept must contain both 'application/json' and 'text/event-stream'", http.StatusBadRequest)
		return false
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeRPCError(w, http.StatusRequestEntityTooLarge, -32600, "request body too large")
			return false
		}
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return false
	}
	if t := bytes.TrimLeft(body, " \t\r\n"); len(t) > 0 && t[0] == '[' {
		writeRPCError(w, http.StatusBadRequest, -32600, "JSON-RPC batching is not supported")
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return true
}

func (s *sessionHost) createAndServe(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFromContext(r.Context())
	ls, err := s.startLocal(r.Context(), s.newSecret(), p, nil)
	if err != nil {
		s.log.ErrorContext(r.Context(), "mcp session connect failed", slog.Any("error", err))
		http.Error(w, "failed connection", http.StatusInternalServerError)
		return
	}
	defer func() { // a session that never completed `initialize` is not a session
		if ls.ss.InitializeParams() == nil {
			s.closeLocal(ls, "init_failed", false)
		}
	}()
	ls.tr.ServeHTTP(w, r)
}

func (s *sessionHost) serveDelete(w http.ResponseWriter, r *http.Request, p Principal, ls *localSession) {
	sctx, cancel := storeCtx(r.Context())
	defer cancel()
	if id := ls.row(); id != "" {
		if err := s.store.Close(sctx, p, id, CloseClientDelete); err != nil && !errors.Is(err, ErrSessionNotFound) {
			s.log.WarnContext(r.Context(), "mcp session close not persisted", slog.Any("error", err))
		}
		s.signal(id, sessionSignal{T: "closed", Reason: CloseClientDelete})
	}
	s.sessionEnded(ls, CloseClientDelete)
	s.closeLocal(ls, CloseClientDelete, true)
	w.WriteHeader(http.StatusNoContent)
}

// CloseSession ends a session from the WS channels or an admin action on any
// replica: the store is already updated by mcp-service; this fans the signal
// out so every replica cancels in-flight tools and drops the resume buffer.
func (s *sessionHost) CloseSession(rowID, reason string) {
	s.signal(rowID, sessionSignal{T: "closed", Reason: reason})
	s.notifyEnded(rowID, reason)
	if ls := s.localByRow(rowID); ls != nil {
		s.closeLocal(ls, reason, true)
	}
}

// sessionEnded tells the OnSessionClosed hook that the session is over for good
// (client DELETE, admin/user close, store expiry). It is NOT called for local
// idle eviction, shutdown or a dropped connection: those leave the session
// alive and its tools' processes with it.
func (s *sessionHost) sessionEnded(ls *localSession, reason string) { s.notifyEnded(ls.row(), reason) }

func (s *sessionHost) notifyEnded(rowID, reason string) {
	if s.d.OnSessionClosed != nil && rowID != "" {
		s.d.OnSessionClosed(rowID, reason)
	}
}

// SessionRowID returns the non-secret id of a LOCAL session (kill-switch
// scope and audit key), "" if this replica does not hold it.
func (s *sessionHost) SessionRowID(secret string) string {
	if ls := s.local(secret); ls != nil {
		return ls.row()
	}
	return ""
}

func (s *sessionHost) serveGET(w http.ResponseWriter, r *http.Request, p Principal, ls *localSession) {
	if _, streamOK := accepts(r.Header.Values("Accept")); !streamOK {
		http.Error(w, "Accept must contain 'text/event-stream' for GET requests", http.StatusBadRequest)
		return
	}
	if s.registry != nil && ls.row() != "" {
		sctx, cancel := storeCtx(r.Context())
		sid, err := s.registry.OpenStream(sctx, p, ls.row(), s.cfg.MaxStreamsPerUser, s.cfg.MaxStreamsPerTenant)
		cancel()
		switch {
		case errors.Is(err, ErrStreamLimit):
			s.rec.RateLimited()
			w.Header().Set("Retry-After", "5")
			writeRPCError(w, http.StatusTooManyRequests, -32000, "too many open streams")
			return
		case errors.Is(err, ErrSessionNotFound):
			http.Error(w, "session not found", http.StatusNotFound)
			return
		case err != nil:
			s.log.WarnContext(r.Context(), "mcp stream registry unavailable; continuing with the per-replica cap", slog.Any("error", err))
		default:
			stop := s.heartbeat(r.Context(), p, sid)
			defer stop()
		}
	}
	if s.srec != nil {
		s.srec.StreamOpened()
		defer s.srec.StreamClosed()
	}
	if leid := r.Header.Get("Last-Event-ID"); leid != "" && s.resum != nil {
		s.serveResume(w, r, ls, leid)
		return
	}
	ls.tr.ServeHTTP(w, r)
}

// heartbeat keeps a stream row live until the returned stop is called, then
// removes it.
func (s *sessionHost) heartbeat(ctx context.Context, p Principal, streamID string) (stop func()) {
	done := make(chan struct{})
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-s.stop:
				return
			case <-t.C:
				hctx, cancel := storeCtx(ctx)
				_ = s.registry.HeartbeatStream(hctx, p, streamID)
				cancel()
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			cctx, cancel := storeCtx(ctx)
			defer cancel()
			_ = s.registry.CloseStream(cctx, p, streamID)
		})
	}
}

// ---- cross-replica signals ------------------------------------------------

type sessionSignal struct {
	T      string          `json:"t"` // cancel | closed
	From   string          `json:"from"`
	RID    json.RawMessage `json:"rid,omitempty"`
	Reason string          `json:"reason,omitempty"`
}

func (s *sessionHost) signal(rowID string, sig sessionSignal) {
	if s.bus == nil || rowID == "" {
		return
	}
	sig.From = s.replicaID
	b, _ := json.Marshal(sig)
	if err := s.bus.Publish(signalSessionPrefix+rowID, b); err != nil {
		s.log.Warn("mcp session signal not published", slog.String("type", sig.T), slog.Any("error", err))
	}
}

func (s *sessionHost) onSessionSignal(subject string, data []byte) {
	var sig sessionSignal
	if json.Unmarshal(data, &sig) != nil || sig.From == s.replicaID {
		return
	}
	ls := s.localByRow(strings.TrimPrefix(subject, signalSessionPrefix))
	if ls == nil {
		return
	}
	switch sig.T {
	case "closed":
		s.sessionEnded(ls, sig.Reason)
		s.closeLocal(ls, sig.Reason, false)
	case "cancel":
		s.injectCancel(ls, sig.RID)
	}
}

func (s *sessionHost) injectCancel(ls *localSession, rid json.RawMessage) {
	params, _ := json.Marshal(map[string]json.RawMessage{"requestId": rid})
	msg := &jsonrpc.Request{Method: "notifications/cancelled", Params: params,
		Extra: &mcp.RequestExtra{Header: http.Header{injectedHeader: {"1"}}}}
	if ls.conn.Inject(msg) && s.srec != nil {
		s.srec.RequestCancelled()
	}
}

// onTenantSignal tells this replica's sessions of a tenant that the
// tool list changed (core-NATS hint from mcp-service/BE-007/BE-012). Only
// sessions held by THIS replica are written; no buffer, no replication.
func (s *sessionHost) onTenantSignal(subject string, data []byte) {
	tenant := strings.TrimPrefix(subject, signalTenantPrefix)
	var sig struct {
		T string `json:"t"`
	}
	if json.Unmarshal(data, &sig) != nil || sig.T != "tools_list_changed" {
		return
	}
	s.notifyToolsListChanged(tenant)
}

// notifyToolsListChanged writes notifications/tools/list_changed to every
// session of the tenant held by THIS replica. Sessions elsewhere are told by
// their own replica (each one receives the triggering event).
func (s *sessionHost) notifyToolsListChanged(tenant string) {
	if !s.cfg.ToolsListChanged {
		return
	}
	s.mu.Lock()
	var targets []*localSession
	for _, ls := range s.bySecret {
		if ls.tenantID == tenant {
			targets = append(targets, ls)
		}
	}
	s.mu.Unlock()
	for _, ls := range targets {
		_ = ls.conn.Notify("notifications/tools/list_changed", nil)
	}
}
