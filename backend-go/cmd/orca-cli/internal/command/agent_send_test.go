package command

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func TestAgentSend_SuccessRoundTrip_ReturnsExitCode0(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/worktrees/wt-1/agent/send" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	factory := func() (*apiclient.Client, error) { return apiclient.New(srv.URL, "test-token"), nil }
	exitCode := ExecuteAgentSend(context.Background(), factory, "wt-1", "hello agent", false)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
	if gotBody == "" {
		t.Fatal("expected a non-empty request body")
	}
}

func TestAgentSend_MissingText_Returns2WithoutHTTPCall(t *testing.T) {
	factory := func() (*apiclient.Client, error) {
		t.Fatal("clientFactory must not be called when --text/stdin resolves empty")
		return nil, nil
	}

	exitCode := ExecuteAgentSend(context.Background(), factory, "wt-1", "", false)

	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
}
