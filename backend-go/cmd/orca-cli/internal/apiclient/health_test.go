package apiclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newFakeHealthServer(t *testing.T, healthzStatus, readyzStatus int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(healthzStatus)
		case "/readyz":
			w.WriteHeader(readyzStatus)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
}

func TestGetHealth_BothOK_ReturnsHealthyAndReady(t *testing.T) {
	srv := newFakeHealthServer(t, http.StatusOK, http.StatusOK)
	defer srv.Close()

	c := New(srv.URL, "")
	result, err := c.GetHealth(context.Background())
	if err != nil {
		t.Fatalf("GetHealth() error = %v", err)
	}
	if !result.Healthy || !result.Ready {
		t.Fatalf("result = %+v, want {Healthy:true Ready:true}", result)
	}
}

// TestGetHealth_ReadyzFails_ReturnsHealthyButNotReady proves a failing
// readyz checker (503) is a distinct outcome from Healthy=false or a
// network error — the gateway process is up and answering, just not ready
// (e.g. DB unreachable).
func TestGetHealth_ReadyzFails_ReturnsHealthyButNotReady(t *testing.T) {
	srv := newFakeHealthServer(t, http.StatusOK, http.StatusServiceUnavailable)
	defer srv.Close()

	c := New(srv.URL, "")
	result, err := c.GetHealth(context.Background())
	if err != nil {
		t.Fatalf("GetHealth() error = %v", err)
	}
	if !result.Healthy {
		t.Fatal("result.Healthy = false, want true")
	}
	if result.Ready {
		t.Fatal("result.Ready = true, want false")
	}
}

// TestGetHealth_GatewayUnreachable_ReturnsError proves a connection
// failure (gateway fully down) returns a non-nil error, not a
// HealthResult{Healthy:false} — a distinct failure mode from "reachable
// but reporting unhealthy".
func TestGetHealth_GatewayUnreachable_ReturnsError(t *testing.T) {
	srv := newFakeHealthServer(t, http.StatusOK, http.StatusOK)
	srv.Close() // close immediately: baseURL now points at nothing listening

	c := New(srv.URL, "")
	_, err := c.GetHealth(context.Background())
	if err == nil {
		t.Fatal("GetHealth() error = nil, want non-nil for an unreachable gateway")
	}
}
