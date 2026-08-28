package sshconn_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// startEchoServer starts a minimal local TCP echo listener — the "remote"
// service the tunnel forwards to (dialed via the fake SSH server's
// direct-tcpip channel support, itself a real proxy — see connector_test.go's
// handleDirectTCPIP).
func startEchoServer(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if n > 0 {
						if _, werr := c.Write(buf[:n]); werr != nil {
							return
						}
					}
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()
	return mustPort(t, ln.Addr().String())
}

func mustPort(t *testing.T, addr string) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("splitting addr %q: %v", addr, err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parsing port from %q: %v", portStr, err)
	}
	return port
}

// freeLocalPort finds an OS-assigned free port by briefly binding then
// releasing it — Tunnel's local listener rebinds this exact port a moment
// later. Small TOCTOU window, acceptable for tests (same pattern
// portalloc.Allocator's own probe-then-bind uses in production).
func freeLocalPort(t *testing.T) (int, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding a free local port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return mustPort(t, addr), addr
}

func dialFakeSSHTarget(t *testing.T, ca *fakeCA, server *fakeSSHServer, targetID string) *sshconn.Connection {
	t.Helper()
	target, err := domain.NewSshTarget(targetID, "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	issuer := &fakeIssuer{ca: ca, principal: "deploy"}
	connector := sshconn.NewConnector(issuer, nil, sshconn.Config{DialTimeout: 5 * time.Second}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := connector.Connect(ctx, target)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return conn
}

func TestTunnel_ForwardsBytesRoundTrip(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy")
	echoPort := startEchoServer(t)

	conn := dialFakeSSHTarget(t, ca, server, "target-tunnel-1")
	defer func() { _ = conn.Close() }()

	localPort, localAddr := freeLocalPort(t)
	tunnel, err := conn.Forward(localPort, echoPort)
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	defer func() { _ = tunnel.Close() }()

	client, err := net.Dial("tcp", localAddr)
	if err != nil {
		t.Fatalf("dialing tunnel's local port: %v", err)
	}
	defer func() { _ = client.Close() }()

	if _, err := client.Write([]byte("hello through the tunnel")); err != nil {
		t.Fatalf("writing to tunnel: %v", err)
	}
	buf := make([]byte, 64)
	_ = client.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("reading echoed bytes: %v", err)
	}
	if got := string(buf[:n]); got != "hello through the tunnel" {
		t.Errorf("echoed %q, want %q", got, "hello through the tunnel")
	}
}

func TestTunnel_CloseStopsAcceptingNewConnections(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy")
	echoPort := startEchoServer(t)

	conn := dialFakeSSHTarget(t, ca, server, "target-tunnel-2")
	defer func() { _ = conn.Close() }()

	localPort, localAddr := freeLocalPort(t)
	tunnel, err := conn.Forward(localPort, echoPort)
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}

	if err := tunnel.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// A second Close must be a safe no-op.
	if err := tunnel.Close(); err != nil {
		t.Errorf("second Close returned an error: %v", err)
	}

	if _, err := net.DialTimeout("tcp", localAddr, time.Second); err == nil {
		t.Error("expected the local listener to be closed and refuse new connections")
	}
}

func TestTunnel_ClosesInFlightCopiesWithoutLeaking(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy")
	echoPort := startEchoServer(t)

	conn := dialFakeSSHTarget(t, ca, server, "target-tunnel-3")
	defer func() { _ = conn.Close() }()

	localPort, localAddr := freeLocalPort(t)
	tunnel, err := conn.Forward(localPort, echoPort)
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}

	client, err := net.Dial("tcp", localAddr)
	if err != nil {
		t.Fatalf("dialing tunnel's local port: %v", err)
	}
	defer func() { _ = client.Close() }()

	// Prove the forward is actually live before closing.
	if _, err := client.Write([]byte("ping")); err != nil {
		t.Fatalf("writing: %v", err)
	}
	buf := make([]byte, 16)
	_ = client.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := client.Read(buf); err != nil {
		t.Fatalf("reading: %v", err)
	}

	if err := tunnel.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The in-flight client connection must observe the forward going away
	// (either a read error or EOF) shortly after Close — proves the
	// listener-side socket teardown propagates to serveOne's io.Copy calls.
	_ = client.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = client.Read(buf)
	if err == nil {
		t.Error("expected the in-flight connection to observe the tunnel closing")
	}
}
