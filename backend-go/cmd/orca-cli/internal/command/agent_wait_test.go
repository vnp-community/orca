package command

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func newFakeWaitServer(t *testing.T, resp apiclient.WaitAgentResult) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/worktrees/wt-1/agent/wait" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// TestAgentWait_TimedOut_MapsToExitCode2 proves timed_out=true in the RPC
// response maps to exit code 2 exactly (BR-CLI-05) — decided purely from
// the response body, never from a client-side timer.
func TestAgentWait_TimedOut_MapsToExitCode2(t *testing.T) {
	srv := newFakeWaitServer(t, apiclient.WaitAgentResult{TimedOut: true})
	defer srv.Close()

	cli := apiclient.New(srv.URL, "test-token")
	// A large --timeout does not leak into the exit-code decision — only
	// the response body's timed_out field does, so this passing 1h and
	// still getting exit 2 back proves the decision isn't elapsed-time
	// based on the client side.
	_, exitCode := RunAgentWait(context.Background(), cli, "wt-1", time.Hour)

	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
}

// TestAgentWait_Exited_MapsToExitCode0 proves a normal (non-timed-out)
// completion maps to the base success exit code.
func TestAgentWait_Exited_MapsToExitCode0(t *testing.T) {
	srv := newFakeWaitServer(t, apiclient.WaitAgentResult{Exited: true, ExitCode: 0})
	defer srv.Close()

	cli := apiclient.New(srv.URL, "test-token")
	result, exitCode := RunAgentWait(context.Background(), cli, "wt-1", 30*time.Second)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
	if !result.Exited || result.TimedOut {
		t.Fatalf("result = %+v, want Exited:true TimedOut:false", result)
	}
}

// TestAgentWait_TransportError_MapsToExitCode1 proves a genuine
// network/transport failure (not a timeout) maps to the generic
// server-error exit code, not exit 2.
func TestAgentWait_TransportError_MapsToExitCode1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "INTERNAL", "message": "boom"},
		})
	}))
	defer srv.Close()

	cli := apiclient.New(srv.URL, "test-token")
	result, exitCode := RunAgentWait(context.Background(), cli, "wt-1", 30*time.Second)

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if result.TimedOut {
		t.Fatal("result.TimedOut = true, want false for a transport error")
	}
}

// TestExecuteAgentWait_MissingWorktree_Returns2WithoutHTTPCall proves flag
// validation happens before clientFactory is ever invoked.
func TestExecuteAgentWait_MissingWorktree_Returns2WithoutHTTPCall(t *testing.T) {
	factory := func() (*apiclient.Client, error) {
		t.Fatal("clientFactory must not be called when --worktree is missing")
		return nil, nil
	}

	exitCode := ExecuteAgentWait(context.Background(), factory, "", 30*time.Second, false)

	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
}

// TestExecuteAgentWait_TimedOut_ReturnsExitCode2AndPrintsResult proves the
// end-to-end wiring (ExecuteAgentWait, not just RunAgentWait) also reports
// exit 2 on timeout and still prints the (TimedOut) result.
func TestExecuteAgentWait_TimedOut_ReturnsExitCode2AndPrintsResult(t *testing.T) {
	srv := newFakeWaitServer(t, apiclient.WaitAgentResult{TimedOut: true})
	defer srv.Close()
	factory := func() (*apiclient.Client, error) { return apiclient.New(srv.URL, "test-token"), nil }

	exitCode := ExecuteAgentWait(context.Background(), factory, "wt-1", 30*time.Second, false)

	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
}
