// Package mcpserver is the edge protocol adapter for the MCP Streamable HTTP
// endpoint (/mcp). It translates protocol <-> ports only: authentication is a
// TokenVerifier port, tools are a ToolCatalog/ToolExecutor port, and every
// policy/audit/approval decision lives in mcp-service (T1/T2). See
// specs/backend-go/crs/v5 BE-MCP-SOL-002/003 and ADR-MCP-001.
package mcpserver

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// Deps is everything the adapter needs. nil optional fields mean "feature not
// plugged in yet by its owning solution".
type Deps struct {
	Logger *slog.Logger
	Config Config
	// Verifier validates bearer tokens (BE-MCP-SOL-005/006). nil -> deny all.
	Verifier TokenVerifier
	// Catalog lists tools (BE-MCP-SOL-007). nil -> empty list.
	Catalog ToolCatalog
	// Executor runs tools/call (BE-MCP-SOL-007/008). nil -> every call is an
	// unknown-tool protocol error.
	Executor ToolExecutor
	// CursorKeys sign pagination cursors: [current, previous...]. Empty ->
	// an ephemeral per-process key (single replica only; a WARN is logged).
	CursorKeys [][]byte
	// RateLimiter is the gateway's per-tenant limiter (nil = unlimited).
	RateLimiter *usecase.RateLimiter
	// Recorder receives security/metric events (nil -> noop).
	Recorder Recorder
	// EventStore buffers SSE events for resumability (BE-MCP-SOL-004). A
	// ResumableEventStore (JetStream, or MemoryResumableStore) also resumes on
	// ANOTHER replica; nil -> a bounded in-process store (this replica only).
	EventStore mcp.EventStore
	// Sessions is the durable session registry shared by all replicas
	// (identity binding, idle TTL, adoption). nil -> in-process MemorySessionStore.
	Sessions SessionStore
	// Signals carries cancel/close hints between replicas (core NATS). nil ->
	// they only reach this replica.
	Signals SignalBus
	// Now overrides the clock (tests).
	Now func() time.Time
	// KillCheck reports whether an administrator stopped this session
	// (kill-switch scope "session", keyed by the non-secret row id). nil = never.
	KillCheck func(ctx context.Context, p Principal, sessionRowID string) (blocked bool, err error)
	// RequestContext enriches the context of every tool call with the verified
	// session/depth/client facts the policy gate reads (mcppolicy.With*).
	RequestContext func(ctx context.Context, p Principal, info RequestInfo) context.Context
	// GetSessionID overrides session id generation (BE-MCP-SOL-004).
	GetSessionID func() string
	// ProtectedResourceMetadata overrides the built-in RFC 9728 document.
	ProtectedResourceMetadata http.Handler
	// Resources serves resources/* (BE-MCP-SOL-010). nil = capability off.
	Resources ResourceProvider
	// Prompts serves prompts/* (BE-MCP-SOL-011). nil = capability off.
	Prompts PromptProvider
	// AuthServerMetadata, when set, is mounted at
	// /.well-known/oauth-authorization-server (BE-MCP-SOL-005).
	AuthServerMetadata http.Handler
	// OnSessionClosed is called with the non-secret session row id when a
	// session ends for good on this replica (client DELETE, user/admin close,
	// store expiry, or a close signal from another replica). Tool processes the
	// session started (terminals, agents) are stopped from it (BE-MCP-SOL-009).
	// Must not block; nil = ignored.
	OnSessionClosed func(sessionRowID, reason string)
}

// Handler serves /mcp and the OAuth discovery documents.
type Handler struct {
	d        Deps
	cfg      Config
	log      *slog.Logger
	verifier TokenVerifier
	rec      Recorder
	sdk      http.Handler
	streams  *streamLimiter
	engine   *engine
	host     *sessionHost

	toolsDebounce *tenantDebouncer
}

// NewHandler builds the adapter and its SDK server.
func NewHandler(d Deps) *Handler {
	cfg := d.Config.withDefaults()
	log := d.Logger
	if log == nil {
		log = slog.Default()
	}
	h := &Handler{d: d, cfg: cfg, log: log, verifier: d.Verifier, rec: d.Recorder,
		streams: newStreamLimiter(cfg.MaxStreamsPerUser, cfg.MaxStreamsPerTenant)}
	if h.verifier == nil {
		h.verifier = denyAllVerifier{}
	}
	if h.rec == nil {
		h.rec = noopRecorder{}
	}
	catalog := d.Catalog
	if catalog == nil {
		catalog = emptyCatalog{}
	}
	codec := mustCursorCodec(d.CursorKeys, log)
	srvOpts := &engine{catalog: catalog, executor: d.Executor, cursors: codec, pageSize: cfg.PageSize, log: log, ready: newReadySessions(cfg.SessionIdleTTL),
		resources: d.Resources, prompts: d.Prompts, toolsListChanged: cfg.ToolsListChanged}
	h.toolsDebounce = newTenantDebouncer(cfg.ToolsListChangedDebounce)
	server := newSDKServer(srvOpts, cfg, log, d.GetSessionID)
	h.engine = srvOpts
	srvOpts.reqRec, _ = h.rec.(RequestRecorder)
	h.host = newSessionHost(h)
	h.host.server, h.host.ready = server, srvOpts.ready
	srvOpts.host = h.host
	h.host.start()
	if d.Resources != nil {
		d.Resources.Bind(srvOpts)
	}
	// RequireBearerToken populates auth.TokenInfo (RequestExtra.TokenInfo, read
	// by the engine) from the principal our own authenticate middleware already
	// verified. Session ownership is enforced by the session host, not the SDK.
	bind := auth.RequireBearerToken(h.tokenInfoFromPrincipal, &auth.RequireBearerTokenOptions{ResourceMetadataURL: cfg.metadataURL()})
	h.sdk = bind(h.host)
	return h
}

// Close ends every session held by this replica (cancelling in-flight tools)
// and stops background work. Call it on shutdown.
func (h *Handler) Close() {
	h.toolsDebounce.Stop()
	h.host.Close()
}

// CloseSession ends a session by its row id on every replica (WS channel
// mcp.session.close, admin actions).
func (h *Handler) CloseSession(rowID, reason string) { h.host.CloseSession(rowID, reason) }

func mustCursorCodec(keys [][]byte, log *slog.Logger) *CursorCodec {
	if c, err := NewCursorCodec(keys...); err == nil {
		return c
	}
	ephemeral := make([]byte, 32)
	_, _ = rand.Read(ephemeral)
	log.Warn("MCP cursor key not configured: using an ephemeral key; pagination cursors will not survive restarts or work across replicas")
	c, _ := NewCursorCodec(ephemeral)
	return c
}

// Mount registers the routes. Call it OUTSIDE the cookie-authenticated group:
// /mcp authenticates with bearer tokens only.
func (h *Handler) Mount(r chi.Router) {
	var chain http.Handler = h.sdk
	for _, mw := range []func(http.Handler) http.Handler{
		h.limitStreams, h.requireInitializeFirst, h.rateLimit, h.authenticate, h.rejectCookieOnly, h.guardOrigin, h.limitBody, h.observeHTTP,
	} {
		chain = mw(chain)
	}
	// Both spellings: a redirect would drop the POST body.
	r.Handle("/mcp", chain)
	r.Handle("/mcp/", chain)

	meta := h.d.ProtectedResourceMetadata
	if meta == nil {
		meta = protectedResourceMetadata(h.cfg)
	}
	r.Handle("/.well-known/oauth-protected-resource", meta)
	r.Handle("/.well-known/oauth-protected-resource/mcp", meta) // RFC 9728 path-suffixed form
	if h.d.AuthServerMetadata != nil {
		r.Handle("/.well-known/oauth-authorization-server", h.d.AuthServerMetadata)
	}
}

// NotifyPromptsChanged tells every session on this replica that the prompt
// list changed (notifications/prompts/list_changed, debounced by the SDK). The
// SDK cannot target one tenant, so other tenants may see a spurious
// list_changed; it carries no data and their next prompts/list is unchanged.
func (h *Handler) NotifyPromptsChanged() {
	if h == nil || h.engine == nil {
		return
	}
	h.engine.promptsChanged()
}
