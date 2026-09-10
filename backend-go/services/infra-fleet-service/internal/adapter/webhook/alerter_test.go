package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestAlerter_NotifyStatusChange_ReceivesExactJSONShape(t *testing.T) {
	var mu sync.Mutex
	var received map[string]any
	var contentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		contentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&received)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := NewAlerter(server.URL, nil)
	ds := domain.DevServer{ID: "ds1", TenantID: "t1", Host: "dev1.example.com"}
	sample := domain.DevServerHealth{DevServerID: "ds1", CPUPercent: 81.5, RAMPercent: 40.2}
	a.NotifyStatusChange(context.Background(), ds, domain.HealthStatusHealthy, domain.HealthStatusDegraded, sample)

	mu.Lock()
	defer mu.Unlock()
	if received == nil {
		t.Fatal("expected the webhook server to receive a request")
	}
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}
	if received["event"] != "fleet.server.status_change" {
		t.Errorf("expected event=fleet.server.status_change, got %v", received["event"])
	}
	if received["server"] != "dev1.example.com" {
		t.Errorf("expected server=dev1.example.com, got %v", received["server"])
	}
	if received["from"] != string(domain.HealthStatusHealthy) || received["to"] != string(domain.HealthStatusDegraded) {
		t.Errorf("unexpected from/to: %+v", received)
	}
	if _, ok := received["timestamp"].(string); !ok {
		t.Errorf("expected a string timestamp field, got %+v", received["timestamp"])
	}
	metrics, ok := received["metrics"].(map[string]any)
	if !ok {
		t.Fatalf("expected a metrics object, got %+v", received["metrics"])
	}
	if metrics["cpu"] != 81.5 || metrics["ram"] != 40.2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}
}

func TestAlerter_EmptyURLSendsNothing(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	a := NewAlerter("", nil) // FLEET_WEBHOOK_URL="" — disabled
	ds := domain.DevServer{ID: "ds1", TenantID: "t1", Host: "dev1.example.com"}
	a.NotifyStatusChange(context.Background(), ds, domain.HealthStatusHealthy, domain.HealthStatusDegraded, domain.DevServerHealth{})

	if called {
		t.Error("expected no request to be sent when the alerter's url is empty")
	}
}

func TestAlerter_ServerReturning500DoesNotPropagateAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := NewAlerter(server.URL, nil)
	ds := domain.DevServer{ID: "ds1", TenantID: "t1", Host: "dev1.example.com"}
	// NotifyStatusChange returns nothing (fire-and-forget) — this test's
	// real assertion is simply that this call completes without panicking.
	a.NotifyStatusChange(context.Background(), ds, domain.HealthStatusHealthy, domain.HealthStatusDegraded, domain.DevServerHealth{})
}

func TestAlerter_UnreachableServerDoesNotPropagateAnError(t *testing.T) {
	// Port 0 on a closed httptest server — a connection that will fail.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close() // now unreachable

	a := NewAlerter(url, nil)
	ds := domain.DevServer{ID: "ds1", TenantID: "t1", Host: "dev1.example.com"}
	a.NotifyStatusChange(context.Background(), ds, domain.HealthStatusHealthy, domain.HealthStatusUnreachable, domain.DevServerHealth{})
}
