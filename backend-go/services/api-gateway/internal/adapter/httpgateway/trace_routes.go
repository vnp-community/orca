package httpgateway

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// mountTraceRoutes serves GET /api/trace-stream — the SSE endpoint
// frontend/src/shared/trace/browser.ts's startSseClient() connects an
// EventSource to at app boot (main-web-bootstrap.tsx's initBrowserTrace()),
// powering TracePanel (Ctrl+Shift+T). Matches
// backend/src/server/trace-sse-routes.ts's wire behavior (SSE headers,
// an immediate ": connected" comment, a 15s heartbeat) closely enough that
// EventSource — which silently auto-reconnects on any error/close, per
// browser.ts's onerror comment — never sees a hard failure.
//
// Deliberately unauthenticated, matching the old backend's own
// "intentionally low-security: trace data is diagnostic, not sensitive"
// stance (trace-sse-routes.ts's isAuthorized() comment) — mounted outside
// authMiddleware in router.go, same group as /auth/local and /ws.
//
// Known gap: this only keeps the connection alive (heartbeats) — it does
// NOT forward any real trace/debug events yet, since backend-go has no
// equivalent to the old backend's global registerTraceSink() fan-out.
// TracePanel will show a live-but-empty stream rather than the 404 that
// was breaking EventSource's connection state before this existed. Wiring
// real event forwarding (e.g. from common/eventbus) is tracked as a
// follow-up in docs/execution-plan.md, not attempted here.
func mountTraceRoutes(mux chi.Router) {
	mux.Get("/api/trace-stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "GET only")
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSONError(w, http.StatusInternalServerError, "SSE_UNSUPPORTED", "streaming not supported")
			return
		}

		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("X-Accel-Buffering", "no") // tell nginx not to buffer SSE
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(": connected\n\n"))
		flusher.Flush()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := w.Write([]byte(": heartbeat\n\n")); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})
}
