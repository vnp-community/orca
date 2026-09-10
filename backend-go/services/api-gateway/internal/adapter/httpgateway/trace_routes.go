package httpgateway

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// traceStreamHeartbeatInterval is a var (not a literal in mountTraceRoutes)
// solely so trace_routes_test.go can shrink it for
// TestMountTraceRoutes_HeartbeatStillFiresWithNoEvents without waiting a
// real 15s — production always runs at the 15s default.
var traceStreamHeartbeatInterval = 15 * time.Second

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
<<<<<<< HEAD
// broadcast forwards real backend spans (TASK-BE-FFT-009/010) — every
// currently-connected client subscribes to the same *TraceBroadcast hub
// cmd/server/main.go feeds from the TRACE JetStream stream. frontend's
// EventSource.onmessage (browser.ts) parses each `data:` line as JSON
// directly, no `event:` field needed (default "message" event).
func mountTraceRoutes(mux chi.Router, broadcast *TraceBroadcast) {
	if broadcast == nil {
		broadcast = NewTraceBroadcast()
	}
=======
// Forwards real F40 TraceEvent JSON delivered via broadcast — fed by this
// replica's own NATS SubscribeEphemeral loop (main.go, CR-FFT-002/003):
// each api-gateway replica independently fans NATS trace-span events out
// to its own locally-connected SSE clients, the same per-replica fan-out
// shape as notification-service's cross-replica broadcaster.
func mountTraceRoutes(mux chi.Router, broadcast *TraceBroadcast) {
>>>>>>> feat/team-rbac-implementation
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

<<<<<<< HEAD
		events, unsubscribe := broadcast.Subscribe()
		defer unsubscribe()

		ticker := time.NewTicker(15 * time.Second)
=======
		ch, unsubscribe := broadcast.Subscribe()
		defer unsubscribe()

		ticker := time.NewTicker(traceStreamHeartbeatInterval)
>>>>>>> feat/team-rbac-implementation
		defer ticker.Stop()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
<<<<<<< HEAD
			case payload := <-events:
				if _, err := w.Write([]byte("data: ")); err != nil {
					return
				}
				if _, err := w.Write(payload); err != nil {
					return
				}
				if _, err := w.Write([]byte("\n\n")); err != nil {
=======
			case raw := <-ch:
				if _, err := w.Write(append(append([]byte("data: "), raw...), '\n', '\n')); err != nil {
>>>>>>> feat/team-rbac-implementation
					return
				}
				flusher.Flush()
			case <-ticker.C:
				if _, err := w.Write([]byte(": heartbeat\n\n")); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})
}
