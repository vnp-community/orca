package command

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

// TestIdempotencyKey_StableAcrossCalls proves IdempotencyKey() derives a
// deterministic sha256 hex digest from (project_id, repo_id, branch) per
// BR-CLI-01 — same three inputs must always produce the same key so a
// retried `orca worktree create` dedupes against the first attempt.
func TestIdempotencyKey_StableAcrossCalls(t *testing.T) {
	opts := WorktreeCreateOptions{ProjectID: "proj-1", RepoID: "repo-1", Name: "feature-x"}

	first := opts.IdempotencyKey()
	second := opts.IdempotencyKey()

	if first != second {
		t.Fatalf("IdempotencyKey() not stable: %q vs %q", first, second)
	}
	if len(first) != 64 { // hex-encoded sha256 is always 64 chars
		t.Fatalf("IdempotencyKey() = %q, want a 64-char hex sha256 digest", first)
	}
}

// TestIdempotencyKey_OverrideWins proves an explicit --idempotency-key
// value is used verbatim instead of being derived.
func TestIdempotencyKey_OverrideWins(t *testing.T) {
	opts := WorktreeCreateOptions{
		ProjectID: "proj-1", RepoID: "repo-1", Name: "feature-x",
		IdempotencyKeyOverride: "my-custom-key",
	}
	if got := opts.IdempotencyKey(); got != "my-custom-key" {
		t.Fatalf("IdempotencyKey() = %q, want %q", got, "my-custom-key")
	}
}

// newFakeWorktreesServer returns an httptest.Server that responds to POST
// /v1/worktrees with a fixed WorktreeResult — SpawnAgent never makes an
// HTTP call (apiclient.SpawnAgent is a local stub, see worktree.go), so
// this server only needs the one route.
func newFakeWorktreesServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/worktrees" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(apiclient.WorktreeResult{
			WorktreeID: "wt-1", Path: "/worktrees/wt-1", HeadSHA: "abc123",
		})
	}))
}

// TestExecuteWorktreeCreate_AgentSpawnFailure_DegradesToWarningExit0 proves
// SpawnAgent's AGENT_SPAWN_NOT_SUPPORTED error degrades to a warning, exit
// 0 — never exit 1 — per BUG-AG-01's scope note in worktree_create.go.
func TestExecuteWorktreeCreate_AgentSpawnFailure_DegradesToWarningExit0(t *testing.T) {
	srv := newFakeWorktreesServer(t)
	defer srv.Close()

	opts := WorktreeCreateOptions{ProjectID: "proj-1", RepoID: "repo-1", Name: "feature-x", Agent: "claude"}
	factory := func() (*apiclient.Client, error) { return apiclient.New(srv.URL, "test-token"), nil }

	exitCode := ExecuteWorktreeCreate(context.Background(), factory, opts, false)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
}

// TestExecuteWorktreeCreate_MissingRequiredFlags_Returns2WithoutHTTPCall
// proves flag validation happens before clientFactory is ever invoked — a
// missing --project-id/--repo-id/--name must never open an HTTP
// connection, let alone issue a request.
func TestExecuteWorktreeCreate_MissingRequiredFlags_Returns2WithoutHTTPCall(t *testing.T) {
	factoryCalled := false
	factory := func() (*apiclient.Client, error) {
		factoryCalled = true
		t.Fatal("clientFactory must not be called when required flags are missing")
		return nil, nil
	}

	opts := WorktreeCreateOptions{ProjectID: "", RepoID: "repo-1", Name: "feature-x"}
	exitCode := ExecuteWorktreeCreate(context.Background(), factory, opts, false)

	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
	if factoryCalled {
		t.Fatal("clientFactory was called despite missing required flags")
	}
}

// TestExecuteWorktreeCreate_JSONOutput_MatchesResultShape proves --json
// mode emits exactly Result's JSON shape for a fixed fake response, byte
// content decoded field-for-field.
func TestExecuteWorktreeCreate_JSONOutput_MatchesResultShape(t *testing.T) {
	srv := newFakeWorktreesServer(t)
	defer srv.Close()

	opts := WorktreeCreateOptions{ProjectID: "proj-1", RepoID: "repo-1", Name: "feature-x"}
	factory := func() (*apiclient.Client, error) { return apiclient.New(srv.URL, "test-token"), nil }

	stdout := captureStdout(t, func() {
		exitCode := ExecuteWorktreeCreate(context.Background(), factory, opts, true)
		if exitCode != 0 {
			t.Fatalf("exit code = %d, want 0", exitCode)
		}
	})

	var got Result
	if err := json.Unmarshal(stdout, &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v; stdout=%s", err, stdout)
	}
	if got.WorktreeID != "wt-1" || got.Path != "/worktrees/wt-1" || got.HeadSHA != "abc123" || got.PtyID != "" || len(got.Warnings) != 0 {
		t.Fatalf("decoded Result = %+v, want {WorktreeID:wt-1 Path:/worktrees/wt-1 HeadSHA:abc123 PtyID: Warnings:[]}", got)
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it — used to assert on output.Report's --json
// encoding without touching os.Stdout globally for other tests.
func captureStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe writer: %v", err)
	}
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return buf
}
