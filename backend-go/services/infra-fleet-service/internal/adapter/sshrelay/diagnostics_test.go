package sshrelay_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshrelay"
)

func TestDiagnosticStderr_CapsToTail(t *testing.T) {
	buf := sshrelay.NewDiagnosticStderrForTest(10)

	if _, err := buf.Write([]byte("0123456789")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := buf.String(); got != "0123456789" {
		t.Fatalf("String() = %q, want %q", got, "0123456789")
	}

	if _, err := buf.Write([]byte("ABCDE")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Buffer capped at 10 bytes, keeping the tail: last 10 of "0123456789ABCDE".
	want := "56789ABCDE"
	if got := buf.String(); got != want {
		t.Errorf("String() = %q, want %q (tail-truncated to cap)", got, want)
	}
}

func TestDiagnosticStderr_ConcurrentWritesAreSafe(t *testing.T) {
	buf := sshrelay.NewDiagnosticStderrForTest(1024)
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 50; j++ {
				_, _ = buf.Write([]byte("x"))
			}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	if got := len(buf.String()); got != 500 {
		t.Errorf("expected 500 bytes written total, got %d", got)
	}
}

func TestCollectDiagnostics_EmbedsProbeOutputAndStderrTail(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy", false)
	conn := dialFakeServer(t, ca, server, "target-diag-1")
	defer func() { _ = conn.Close() }()

	stderrBuf := sshrelay.NewDiagnosticStderrForTest(1024)
	if _, err := stderrBuf.Write([]byte("relay process crashed: boom")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	diag := sshrelay.CollectDiagnosticsForTest(ctx, conn, stderrBuf)

	for _, want := range []string{"os=", "arch=", "node=", "user=", "stderr_tail="} {
		if !strings.Contains(diag, want) {
			t.Errorf("diagnostics %q missing expected marker %q", diag, want)
		}
	}
	if !strings.Contains(diag, "relay process crashed: boom") {
		t.Errorf("diagnostics %q missing the captured stderr tail", diag)
	}
}
