// Package health implements the /healthz (liveness) and /readyz (readiness)
// HTTP endpoints every service exposes, per
// specs/backend-go/architecture/09-observability-reliability.md. Kubernetes
// wires these to liveness/readiness probes.
package health

import (
	"encoding/json"
	"net/http"
	"sync"
)

// Checker reports whether a dependency (DB pool, Vault lease, NATS
// connection) is currently healthy. Services register one Checker per
// dependency their readiness should reflect.
type Checker func() error

// Server serves /healthz and /readyz.
type Server struct {
	mu       sync.RWMutex
	checkers map[string]Checker
}

// New returns a Server with no registered checkers — /healthz always
// succeeds (the process is up); /readyz succeeds trivially until checkers
// are registered.
func New() *Server {
	return &Server{checkers: make(map[string]Checker)}
}

// Register adds a named readiness check (e.g. "postgres", "vault", "nats").
// A failing checker pulls this pod out of the Service's endpoint list
// without restarting it — see the observability doc's readyz rationale.
func (s *Server) Register(name string, c Checker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkers[name] = c
}

// Handler returns an http.Handler serving /healthz and /readyz, ready to
// mount on a service's HTTP mux alongside /metrics.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", s.handleReady)
	return mux
}

func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make(map[string]string, len(s.checkers))
	allHealthy := true
	for name, check := range s.checkers {
		if err := check(); err != nil {
			results[name] = err.Error()
			allHealthy = false
		} else {
			results[name] = "ok"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if !allHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(results)
}
