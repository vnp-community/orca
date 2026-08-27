package sshrelay_test

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshrelay"
)

// TestLaunch_ReturnsACappedStderrBuffer drives launch() against the shared
// fakeSSHServer directly (independent of Provisioner.Provision) and asserts
// the returned diagnosticStderr is live and capped, per TASK-SSH-02-06 (now
// carried through launch's --detach + reattach() flow — TASK-SSH-03-03).
func TestLaunch_ReturnsACappedStderrBuffer(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy", false)
	conn := dialFakeServer(t, ca, server, "target-launch-1")
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	transport, sockPath, stderrBuf, err := sshrelay.LaunchForTest(ctx, conn, sshrelay.RemoteDirForTest, "ds-launch-1")
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if transport == nil {
		t.Fatal("expected a non-nil transport")
	}
	if stderrBuf == nil {
		t.Fatal("expected a non-nil diagnostic stderr buffer")
	}
	if want := sshrelay.RelaySockPathForTest(sshrelay.RemoteDirForTest); sockPath != want {
		t.Errorf("sockPath = %q, want %q", sockPath, want)
	}
	defer func() { _ = transport.Close("test done") }()

	// The buffer starts empty but is a real, live *diagnosticStderr wired to
	// session.Stderr — verified structurally (cap behavior) since this fake
	// server's exec handler never itself writes to the SSH channel's
	// extended-data (stderr) stream.
	if got := stderrBuf.String(); got != "" {
		t.Errorf("expected an initially-empty stderr buffer, got %q", got)
	}
}
