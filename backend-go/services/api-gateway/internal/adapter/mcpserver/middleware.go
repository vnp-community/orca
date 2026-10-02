package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
)

// orcaSessionCookie is the browser session cookie of the REST/WS edge. It is
// never an acceptable credential on /mcp (confused deputy / CSRF).
const orcaSessionCookie = "orca_session"

// jsonResponseWriteDeadline bounds a JSON reply; the http.Server itself has no
// WriteTimeout because that would also cut SSE streams.
const jsonResponseWriteDeadline = 60 * time.Second

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeKillSwitch is the 403 for a kill-switched tenant/client/grant/session.
func writeKillSwitch(w http.ResponseWriter) {
	writeRPCError(w, http.StatusForbidden, -32000, "MCP_KILL_SWITCH_ACTIVE: MCP access is suspended by an administrator")
}

func writeRPCError(w http.ResponseWriter, status, code int, msg string) {
	writeJSON(w, status, map[string]any{"jsonrpc": "2.0", "id": nil, "error": map[string]any{"code": code, "message": msg}})
}

// limitBody caps the request body (413 + JSON-RPC -32600).
func (h *Handler) limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > h.cfg.MaxBodyBytes {
			writeRPCError(w, http.StatusRequestEntityTooLarge, -32600, "request body too large")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// guardOrigin blocks DNS-rebinding / cross-site browser access. No Origin
// (CLI, desktop, server-side clients) passes; any Origin must be allow-listed,
// and with no allow-list configured every Origin is refused.
func (h *Handler) guardOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" {
			p := h.cfg.AllowedOrigins
			if !p.Enforced() || !p.Allow(r) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden_origin"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) challenge(w http.ResponseWriter, status int, errCode string) {
	v := fmt.Sprintf("Bearer resource_metadata=%q", h.cfg.metadataURL())
	if errCode != "" {
		v += fmt.Sprintf(", error=%q", errCode)
	}
	w.Header().Set("WWW-Authenticate", v)
	body := map[string]string{"error": "unauthorized"}
	if status == http.StatusForbidden {
		body["error"] = "forbidden"
	}
	writeJSON(w, status, body)
}

// rejectCookieOnly refuses session-cookie authentication and strips Cookie so
// nothing deeper can accidentally honor it.
func (h *Handler) rejectCookieOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie(orcaSessionCookie); err == nil && BearerToken(r) == "" {
			h.rec.AuthFailure("cookie_only")
			h.challenge(w, http.StatusUnauthorized, "")
			return
		}
		r.Header.Del("Cookie")
		next.ServeHTTP(w, r)
	})
}

// authenticate runs the TokenVerifier (fail closed) and stores the Principal.
func (h *Handler) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := h.verifier.Verify(r.Context(), r)
		switch {
		case err == nil && p.TenantID != "" && p.UserID != "":
			next.ServeHTTP(w, r.WithContext(ContextWithPrincipal(r.Context(), p)))
		case errors.Is(err, ErrNoCredentials):
			h.rec.AuthFailure("no_credentials")
			h.challenge(w, http.StatusUnauthorized, "")
		case errors.Is(err, ErrVerifierUnavailable):
			h.rec.AuthFailure("verifier_unavailable")
			w.Header().Set("Retry-After", "2")
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		case errors.Is(err, ErrKillSwitchActive):
			// An administrator stopped MCP access: 403, not a 401 challenge
			// (re-authenticating cannot help and would make clients loop).
			h.rec.AuthFailure("kill_switch")
			writeKillSwitch(w)
		case errors.Is(err, ErrInsufficientScope):
			h.rec.AuthFailure("insufficient_scope")
			h.challenge(w, http.StatusForbidden, "insufficient_scope")
		default: // includes a "successful" verify with no tenant/user: fail closed
			h.rec.AuthFailure("invalid_token")
			h.challenge(w, http.StatusUnauthorized, "invalid_token")
		}
	})
}

func (h *Handler) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := PrincipalFromContext(r.Context()); ok && h.d.RateLimiter != nil && !h.d.RateLimiter.Allow(p.TenantID) {
			h.rec.RateLimited()
			w.Header().Set("Retry-After", "1")
			writeRPCError(w, http.StatusTooManyRequests, -32000, "rate limit exceeded")
			return
		}
		if r.Method == http.MethodPost {
			_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(jsonResponseWriteDeadline))
		}
		next.ServeHTTP(w, r)
	})
}

// tokenInfoFromPrincipal adapts our verified Principal to the SDK's TokenInfo.
func (h *Handler) tokenInfoFromPrincipal(ctx context.Context, _ string, _ *http.Request) (*auth.TokenInfo, error) {
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		return nil, auth.ErrInvalidToken
	}
	exp := p.ExpiresAt
	if exp.IsZero() {
		exp = time.Now().Add(time.Hour)
	}
	return &auth.TokenInfo{
		Scopes:     p.Scopes,
		Expiration: exp,
		// Tenant-qualified so two tenants can never share a session binding.
		UserID: p.TenantID + "/" + p.UserID,
		Extra:  map[string]any{principalExtraKey: p},
	}, nil
}

// streamLimiter caps concurrent standalone SSE (GET) streams per user and
// tenant on this replica (cluster-exact limits are BE-MCP-SOL-004's).
type streamLimiter struct {
	mu                 sync.Mutex
	perUser, perTenant int
	users, tenants     map[string]int
}

func newStreamLimiter(perUser, perTenant int) *streamLimiter {
	return &streamLimiter{perUser: perUser, perTenant: perTenant, users: map[string]int{}, tenants: map[string]int{}}
}

func (l *streamLimiter) acquire(p Principal) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	uk := p.TenantID + "/" + p.UserID
	if (l.perUser > 0 && l.users[uk] >= l.perUser) || (l.perTenant > 0 && l.tenants[p.TenantID] >= l.perTenant) {
		return false
	}
	l.users[uk]++
	l.tenants[p.TenantID]++
	return true
}

func (l *streamLimiter) release(p Principal) {
	l.mu.Lock()
	defer l.mu.Unlock()
	uk := p.TenantID + "/" + p.UserID
	if l.users[uk]--; l.users[uk] <= 0 {
		delete(l.users, uk)
	}
	if l.tenants[p.TenantID]--; l.tenants[p.TenantID] <= 0 {
		delete(l.tenants, p.TenantID)
	}
}

func (h *Handler) limitStreams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		p, _ := PrincipalFromContext(r.Context())
		if !h.streams.acquire(p) {
			h.rec.RateLimited()
			w.Header().Set("Retry-After", "5")
			writeRPCError(w, http.StatusTooManyRequests, -32000, "too many open streams")
			return
		}
		defer h.streams.release(p)
		next.ServeHTTP(w, r)
	})
}

// requireInitializeFirst answers a session-less POST that is not `initialize`
// (or `ping`) with a proper JSON-RPC -32600; the SDK would answer such a call
// with error code 0 and spin up a throwaway session first.
func (h *Handler) requireInitializeFirst(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Mcp-Session-Id") != "" {
			next.ServeHTTP(w, r)
			return
		}
		body, err := io.ReadAll(r.Body) // already bounded by limitBody
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				writeRPCError(w, http.StatusRequestEntityTooLarge, -32600, "request body too large")
				return
			}
			writeRPCError(w, http.StatusBadRequest, -32700, "parse error")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		var msg struct {
			Method string `json:"method"`
		}
		if json.Unmarshal(body, &msg) == nil && msg.Method != "" && msg.Method != "initialize" && msg.Method != "ping" {
			writeRPCError(w, http.StatusBadRequest, -32600, "server not initialized: send initialize first")
			return
		}
		next.ServeHTTP(w, r)
	})
}
