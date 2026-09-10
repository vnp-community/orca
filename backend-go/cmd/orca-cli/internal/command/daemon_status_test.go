package command

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/localdaemon"
)

// TestDaemonStatus_LocalMode_NeverCallsClient proves ModeLocal never
// touches cli — cli is passed nil (as the real --local caller does per
// daemon_status.go's doc comment), so if the ModeLocal branch mistakenly
// dereferenced it, this test would panic with a nil-pointer dereference
// instead of returning a result.
func TestDaemonStatus_LocalMode_NeverCallsClient(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	if err := os.WriteFile(pidFile, []byte("999999999"), 0600); err != nil {
		t.Fatalf("writing pidfile: %v", err)
	}
	sup := &localdaemon.ComposeSupervisor{PidFile: pidFile}

	result, err := RunDaemonStatus(context.Background(), ModeLocal, nil, sup)
	if err != nil {
		t.Fatalf("RunDaemonStatus() error = %v", err)
	}
	if result.Mode != "local" {
		t.Fatalf("Mode = %q, want %q", result.Mode, "local")
	}
	if result.Status != "stopped" { // pid 999999999 is not a live process
		t.Fatalf("Status = %q, want %q", result.Status, "stopped")
	}
}

// TestDaemonStatus_RemoteMode_NeverCallsSupervisor proves ModeRemote never
// touches sup — sup is passed nil, so a mistaken sup.Status() call in the
// ModeRemote branch would panic with a nil-pointer dereference.
func TestDaemonStatus_RemoteMode_NeverCallsSupervisor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cli := apiclient.New(srv.URL, "")
	result, err := RunDaemonStatus(context.Background(), ModeRemote, cli, nil)
	if err != nil {
		t.Fatalf("RunDaemonStatus() error = %v", err)
	}
	if result.Mode != "remote" {
		t.Fatalf("Mode = %q, want %q", result.Mode, "remote")
	}
	if result.Status != "healthy" {
		t.Fatalf("Status = %q, want %q", result.Status, "healthy")
	}
}
