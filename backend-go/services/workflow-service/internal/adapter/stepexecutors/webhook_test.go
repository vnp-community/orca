package stepexecutors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func mustHostname(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parsing test server URL: %v", err)
	}
	return u.Hostname()
}

func TestWebhookExecutor_SuccessfulCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	// httptest.NewServer listens on a loopback address — allowlist it
	// explicitly so this test doesn't trip the always-on SSRF loopback
	// block, which is exactly the behavior under test elsewhere.
	exec := NewWebhookExecutor([]string{mustHostname(t, srv.URL)}, srv.Client())

	cfg, _ := json.Marshal(webhookStepConfig{URL: srv.URL, Method: "POST"})
	result, err := exec.Execute(context.Background(), string(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ResultStatusCompleted {
		t.Errorf("expected completed status, got %v: %s", result.Status, result.OutputJSON)
	}
}

func TestWebhookExecutor_NonSuccessStatusIsFailedNotError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	exec := NewWebhookExecutor([]string{mustHostname(t, srv.URL)}, srv.Client())

	cfg, _ := json.Marshal(webhookStepConfig{URL: srv.URL})
	result, err := exec.Execute(context.Background(), string(cfg))
	if err != nil {
		t.Fatalf("unexpected error (non-2xx should be a failed StepResult, not an error): %v", err)
	}
	if result.Status != domain.ResultStatusFailed {
		t.Errorf("expected failed status for a 500 response, got %v", result.Status)
	}
}

func TestWebhookExecutor_BlocksLoopbackWithoutAllowlist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// No allowlist configured — SSRF's private/loopback/link-local block
	// still applies unconditionally.
	exec := NewWebhookExecutor(nil, srv.Client())

	cfg, _ := json.Marshal(webhookStepConfig{URL: srv.URL})
	_, err := exec.Execute(context.Background(), string(cfg))
	if err == nil {
		t.Fatal("expected loopback target to be rejected by the SSRF check")
	}
}

func TestWebhookExecutor_RejectsUnsupportedScheme(t *testing.T) {
	exec := NewWebhookExecutor(nil, nil)
	cfg, _ := json.Marshal(webhookStepConfig{URL: "file:///etc/passwd"})
	_, err := exec.Execute(context.Background(), string(cfg))
	if err == nil {
		t.Fatal("expected an unsupported-scheme error")
	}
}

func TestWebhookExecutor_RejectsInvalidConfigJSON(t *testing.T) {
	exec := NewWebhookExecutor(nil, nil)
	_, err := exec.Execute(context.Background(), "{not json")
	if err == nil {
		t.Fatal("expected an error for invalid config JSON")
	}
}
