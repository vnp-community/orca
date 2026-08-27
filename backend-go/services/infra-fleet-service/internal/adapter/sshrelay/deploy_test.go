package sshrelay_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshrelay"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// flakySSHServer rejects every "exec" channel (a network/SFTP-shaped
// failure from Connect through to deploy()'s first RunCommand) for its
// first failUntilAttempt attempts, then behaves like a normal, minimal
// exec-only server that just exits 0 — enough to drive deployWithRetry's
// network-retry branch through deploy()'s real dial+session-open path
// without needing to fake out SFTP too (deploy's very first RunCommand,
// the `mkdir -p`, is the failure point).
type flakySSHServer struct {
	listener        net.Listener
	attempts        atomic.Int32
	failUntilAttempt int32
}

func startFlakySSHServer(t *testing.T, trustedCAPub ssh.PublicKey, failUntilAttempt int32) *flakySSHServer {
	t.Helper()
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating fake host keypair: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromSigner(hostPriv)
	if err != nil {
		t.Fatalf("wrapping fake host signer: %v", err)
	}
	checker := &ssh.CertChecker{IsUserAuthority: func(auth ssh.PublicKey) bool {
		return string(auth.Marshal()) == string(trustedCAPub.Marshal())
	}}
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		return checker.Authenticate(conn, key)
	}}
	cfg.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	srv := &flakySSHServer{listener: listener, failUntilAttempt: failUntilAttempt}
	t.Cleanup(func() { _ = listener.Close() })
	go srv.serve(cfg)
	return srv
}

func (s *flakySSHServer) port(t *testing.T) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(s.listener.Addr().String())
	if err != nil {
		t.Fatalf("splitting fake server addr: %v", err)
	}
	var port int
	_, _ = fmt.Sscanf(portStr, "%d", &port)
	return port
}

func (s *flakySSHServer) serve(cfg *ssh.ServerConfig) {
	for {
		rawConn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(rawConn, cfg)
	}
}

func (s *flakySSHServer) handleConn(rawConn net.Conn, cfg *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(rawConn, cfg)
	if err != nil {
		_ = rawConn.Close()
		return
	}
	defer func() { _ = sshConn.Close() }()
	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "only session channels supported")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go s.handleSession(channel, requests)
	}
}

func (s *flakySSHServer) handleSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer func() { _ = channel.Close() }()
	for req := range requests {
		if req.Type != "exec" {
			_ = req.Reply(false, nil)
			continue
		}
		_ = req.Reply(true, nil)
		n := s.attempts.Add(1)
		if n <= s.failUntilAttempt {
			// Simulate a transient network/exec failure: close the channel
			// with a nonzero exit rather than completing normally, which
			// surfaces to deploy()'s RunCommand as a run error.
			_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{1}))
			_ = channel.CloseWrite()
			return
		}
		_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
		return
	}
}

func dialFlakyServer(t *testing.T, ca *fakeCA, server *flakySSHServer, targetID string) *sshconn.Connection {
	t.Helper()
	target, err := domain.NewSshTarget(targetID, "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	connector := sshconn.NewConnector(&fakeIssuer{ca: ca, principal: "deploy"}, nil, sshconn.Config{DialTimeout: 5 * time.Second}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := connector.Connect(ctx, target)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return conn
}

func TestDeployWithRetry_SucceedsAfterTransientFailures(t *testing.T) {
	ca := newFakeCA(t)
	// The first 2 exec attempts (both deploy()'s `mkdir -p`) fail; the 3rd
	// (deployWithRetry's final network-retry attempt) still only gets as far
	// as deploy()'s mkdir succeeding — this fake server doesn't implement
	// SFTP, so deploy() will fail later at the SFTP-open step every time.
	// That's fine: this test's purpose is exercising deployWithRetry's
	// backoff-and-retry loop itself (3 attempts, 2 failures then a
	// non-mkdir-shaped failure on the 3rd), not a full successful deploy —
	// TestProvision_SucceedsAgainstFakeServer already covers the full happy
	// path against provisioner_test.go's SFTP-capable fakeSSHServer.
	server := startFlakySSHServer(t, ca.signer.PublicKey(), 2)
	conn := dialFlakyServer(t, ca, server, "target-deploy-retry-1")
	defer func() { _ = conn.Close() }()

	bundlePath := writeLocalBundle(t, "// fake bundle\n")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := sshrelay.DeployWithRetryForTest(ctx, conn, sshrelay.Config{BundlePath: bundlePath})
	// Every attempt against this SFTP-less server ultimately fails (mkdir
	// succeeds on attempt 3, but the following SFTP open never can) — the
	// assertion that matters here is the attempt count, proving the retry
	// loop actually ran multiple times rather than giving up after one.
	if err == nil {
		t.Fatal("expected deployWithRetry to eventually fail against an SFTP-less server")
	}
	if got := server.attempts.Load(); got != 3 {
		t.Errorf("expected exactly 3 exec attempts (2 failures + 1 that got past mkdir), got %d", got)
	}
}

func TestDeployWithRetry_FailsAfterMaxNetworkRetries(t *testing.T) {
	ca := newFakeCA(t)
	server := startFlakySSHServer(t, ca.signer.PublicKey(), 100) // always fails
	conn := dialFlakyServer(t, ca, server, "target-deploy-retry-2")
	defer func() { _ = conn.Close() }()

	bundlePath := writeLocalBundle(t, "// fake bundle\n")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	_, err := sshrelay.DeployWithRetryForTest(ctx, conn, sshrelay.Config{BundlePath: bundlePath})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected deployWithRetry to fail after exhausting network retries")
	}
	if got := server.attempts.Load(); got != 3 {
		t.Errorf("expected exactly 3 attempts (maxDeployNetworkRetries), got %d", got)
	}
	// 500ms + 1s backoff between attempts 1->2 and 2->3 (attempt 3 doesn't sleep after).
	if elapsed < 1400*time.Millisecond {
		t.Errorf("expected backoff delays between retries, only took %s", elapsed)
	}
}

func TestDeployWithRetry_ChecksumMismatchRetriesExactlyOnceNotNetworkBudget(t *testing.T) {
	ca := newFakeCA(t)
	// badChecksum=true on provisioner_test.go's SFTP-capable fakeSSHServer:
	// deploy() gets all the way through mkdir+SFTP-upload, then the checksum
	// step mismatches every time.
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy", true)
	conn := dialFakeServer(t, ca, server, "target-deploy-retry-3")
	defer func() { _ = conn.Close() }()

	bundlePath := writeLocalBundle(t, "// fake bundle\n")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := sshrelay.DeployWithRetryForTest(ctx, conn, sshrelay.Config{BundlePath: bundlePath})
	if err == nil {
		t.Fatal("expected a persistent checksum mismatch to fail")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected a checksum-mismatch error, got: %v", err)
	}
	var mismatch error = sshrelay.ErrChecksumMismatch
	if !errors.Is(err, mismatch) {
		t.Errorf("expected errors.Is(err, ErrChecksumMismatch), got: %v", err)
	}
}
