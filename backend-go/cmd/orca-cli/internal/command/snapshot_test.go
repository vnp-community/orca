package command

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

// TestExecuteSnapshot_OutputFlag_WritesExactBodyByteForByte proves
// --output writes the raw scrollback text/plain response body verbatim —
// no JSON envelope, no re-encoding — even when the payload contains
// characters that would need escaping in a JSON string.
func TestExecuteSnapshot_OutputFlag_WritesExactBodyByteForByte(t *testing.T) {
	const wantBody = "line one\nline \"two\" with \\backslash\\\nline three\t(tab)\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/worktrees/wt-1/agent/snapshot" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(wantBody))
	}))
	defer srv.Close()

	factory := func() (*apiclient.Client, error) { return apiclient.New(srv.URL, "test-token"), nil }

	outputPath := filepath.Join(t.TempDir(), "result.txt")
	exitCode := ExecuteSnapshot(context.Background(), factory, "wt-1", outputPath, false)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if string(got) != wantBody {
		t.Fatalf("file content = %q, want %q", got, wantBody)
	}
}

// TestExecuteSnapshot_OutputFlag_JSONModeStillWritesRawBody proves --json
// does not change --output's file contents — the snapshot payload is
// always a flat text file (BR-CLI-06), never JSON-wrapped.
func TestExecuteSnapshot_OutputFlag_JSONModeStillWritesRawBody(t *testing.T) {
	const wantBody = "raw scrollback text"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(wantBody))
	}))
	defer srv.Close()

	factory := func() (*apiclient.Client, error) { return apiclient.New(srv.URL, "test-token"), nil }

	outputPath := filepath.Join(t.TempDir(), "result.txt")
	exitCode := ExecuteSnapshot(context.Background(), factory, "wt-1", outputPath, true)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if string(got) != wantBody {
		t.Fatalf("file content = %q, want %q (no JSON envelope)", got, wantBody)
	}
}

// TestExecuteSnapshot_MissingWorktree_Returns2WithoutHTTPCall proves flag
// validation happens before clientFactory is ever invoked.
func TestExecuteSnapshot_MissingWorktree_Returns2WithoutHTTPCall(t *testing.T) {
	factory := func() (*apiclient.Client, error) {
		t.Fatal("clientFactory must not be called when --worktree is missing")
		return nil, nil
	}

	exitCode := ExecuteSnapshot(context.Background(), factory, "", "", false)

	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
}
