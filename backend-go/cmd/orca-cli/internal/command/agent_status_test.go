package command

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func TestAgentStatus_SuccessRoundTrip_ReturnsExitCode0(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/worktrees/wt-1/agent/status" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"agent_running":true,"agent_kind":"claude","ready_for_input":true}`))
	}))
	defer srv.Close()

	factory := func() (*apiclient.Client, error) { return apiclient.New(srv.URL, "test-token"), nil }
	exitCode := ExecuteAgentStatus(context.Background(), factory, "wt-1", false)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
}

func TestAgentStatus_MissingWorktree_Returns2WithoutHTTPCall(t *testing.T) {
	factory := func() (*apiclient.Client, error) {
		t.Fatal("clientFactory must not be called when --worktree is missing")
		return nil, nil
	}

	exitCode := ExecuteAgentStatus(context.Background(), factory, "", false)

	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
}
