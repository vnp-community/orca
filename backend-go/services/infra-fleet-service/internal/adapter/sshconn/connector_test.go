package sshconn_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeCA stands in for Vault's SSH secrets engine CA — it never talks to a
// real Vault, but exercises the exact same signed-certificate wire shape
// (ssh.Certificate, marshaled/parsed via ssh.MarshalAuthorizedKey /
// ssh.ParseAuthorizedKey) that a real Vault ssh/sign/<role> response would
// produce, so Connector.Connect's cert handling is exercised for real, not
// stubbed.
type fakeCA struct {
	signer ssh.Signer
}

func newFakeCA(t *testing.T) *fakeCA {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating fake CA keypair: %v", err)
	}
	signer, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		t.Fatalf("wrapping fake CA signer: %v", err)
	}
	return &fakeCA{signer: signer}
}

// fakeIssuer implements sshconn.SSHCertIssuer by actually signing the
// caller's ephemeral public key with the fake CA — a real (if fake-rooted)
// certificate round trip, not a canned return value.
type fakeIssuer struct {
	ca        *fakeCA
	principal string
	failWith  error
}

func (f *fakeIssuer) SSHSignPublicKey(_ context.Context, _ string, publicKeyOpenSSH string) (string, error) {
	if f.failWith != nil {
		return "", f.failWith
	}
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKeyOpenSSH))
	if err != nil {
		return "", fmt.Errorf("fakeIssuer: parsing public key to sign: %w", err)
	}
	cert := &ssh.Certificate{
		Key:             pubKey,
		Serial:          1,
		CertType:        ssh.UserCert,
		ValidPrincipals: []string{f.principal},
		ValidAfter:      0,
		ValidBefore:     ssh.CertTimeInfinity,
	}
	if err := cert.SignCert(rand.Reader, f.ca.signer); err != nil {
		return "", fmt.Errorf("fakeIssuer: signing certificate: %w", err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(cert))), nil
}

// fakeSSHServer is a minimal, real SSH server (golang.org/x/crypto/ssh's own
// server-side API, not a mock) — accepts connections, authenticates via
// ssh.CertChecker against a trusted CA public key, and on a "session"
// channel handles a single "exec" request by writing back a fixed string
// plus a zero exit status. In the spirit of
// devserveragent/client_test.go's fakeAgent: a real, minimal, working
// counterpart, not a mock.
type fakeSSHServer struct {
	listener net.Listener
}

func startFakeSSHServer(t *testing.T, trustedCAPub ssh.PublicKey, expectPrincipal string) *fakeSSHServer {
	t.Helper()

	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating fake host keypair: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromSigner(hostPriv)
	if err != nil {
		t.Fatalf("wrapping fake host signer: %v", err)
	}

	checker := &ssh.CertChecker{
		IsUserAuthority: func(auth ssh.PublicKey) bool {
			return string(auth.Marshal()) == string(trustedCAPub.Marshal())
		},
	}

	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if conn.User() != expectPrincipal {
				return nil, fmt.Errorf("unexpected user %q", conn.User())
			}
			return checker.Authenticate(conn, key)
		},
	}
	cfg.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	srv := &fakeSSHServer{listener: listener}
	t.Cleanup(func() { _ = listener.Close() })

	go srv.serve(cfg)
	return srv
}

// port returns the OS-assigned port the fake server is listening on, for
// tests to feed into sshconn.Config{Port: ...} — Connect otherwise defaults
// to the real port 22 (see connector.go's documented gap).
func (s *fakeSSHServer) port(t *testing.T) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(s.listener.Addr().String())
	if err != nil {
		t.Fatalf("splitting fake server addr: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parsing fake server port: %v", err)
	}
	return port
}

func (s *fakeSSHServer) serve(cfg *ssh.ServerConfig) {
	for {
		rawConn, err := s.listener.Accept()
		if err != nil {
			return // listener closed at test cleanup
		}
		go s.handleConn(rawConn, cfg)
	}
}

func (s *fakeSSHServer) handleConn(rawConn net.Conn, cfg *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(rawConn, cfg)
	if err != nil {
		_ = rawConn.Close()
		return // auth failure or handshake failure — expected in the rejection test
	}
	defer func() { _ = sshConn.Close() }()
	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		switch newChannel.ChannelType() {
		case "session":
			channel, requests, err := newChannel.Accept()
			if err != nil {
				continue
			}
			go handleSessionRequests(channel, requests)
		case "direct-tcpip":
			// Minimal port-forwarding support so this fake server can act as
			// a jump host/bastion for TestConnect_DialsThroughJumpHost — a
			// real proxy, not a mock: it actually dials the requested
			// address and pipes bytes both ways.
			go handleDirectTCPIP(newChannel)
		default:
			_ = newChannel.Reject(ssh.UnknownChannelType, "only session/direct-tcpip channels supported")
		}
	}
}

// directTCPIPPayload mirrors RFC 4254 §7.2's "direct-tcpip" channel-open
// extra data: the address ssh.Client.Dial asked this server to forward to.
type directTCPIPPayload struct {
	HostToConnect  string
	PortToConnect  uint32
	OriginatorAddr string
	OriginatorPort uint32
}

func handleDirectTCPIP(newChannel ssh.NewChannel) {
	var payload directTCPIPPayload
	if err := ssh.Unmarshal(newChannel.ExtraData(), &payload); err != nil {
		_ = newChannel.Reject(ssh.ConnectionFailed, "malformed direct-tcpip payload")
		return
	}
	target := net.JoinHostPort(payload.HostToConnect, fmt.Sprintf("%d", payload.PortToConnect))
	targetConn, err := net.Dial("tcp", target)
	if err != nil {
		_ = newChannel.Reject(ssh.ConnectionFailed, "dial failed: "+err.Error())
		return
	}
	channel, requests, err := newChannel.Accept()
	if err != nil {
		_ = targetConn.Close()
		return
	}
	go ssh.DiscardRequests(requests)

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(targetConn, channel)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(channel, targetConn)
		done <- struct{}{}
	}()
	<-done
	_ = targetConn.Close()
	_ = channel.Close()
}

func handleSessionRequests(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer func() { _ = channel.Close() }()
	for req := range requests {
		switch req.Type {
		case "exec":
			var execMsg struct{ Command string }
			if err := ssh.Unmarshal(req.Payload, &execMsg); err != nil {
				_ = req.Reply(false, nil)
				continue
			}
			_ = req.Reply(true, nil)
			_, _ = channel.Write([]byte("fake-server-output for: " + execMsg.Command))
			_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
			return
		default:
			_ = req.Reply(false, nil)
		}
	}
}

func TestConnectAndRunCommand_SucceedsAgainstFakeServer(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy")

	target, err := domain.NewSshTarget("target-1", "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
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
	defer func() { _ = conn.Close() }()

	stdout, _, err := conn.RunCommand(ctx, "echo hi")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if !strings.Contains(stdout, "echo hi") {
		t.Errorf("stdout = %q, want it to contain the executed command", stdout)
	}
}

func TestConnect_FailsWhenIssuerErrors(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy")

	target, err := domain.NewSshTarget("target-2", "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}

	issuer := &fakeIssuer{ca: ca, principal: "deploy", failWith: errors.New("vault: role not found")}
	connector := sshconn.NewConnector(issuer, nil, sshconn.Config{DialTimeout: 5 * time.Second}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := connector.Connect(ctx, target)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected Connect to fail when the issuer errors (Vault unreachable/role-not-found)")
	}
	if !strings.Contains(err.Error(), "role not found") {
		t.Errorf("expected the wrapped error to mention the underlying cause, got: %v", err)
	}
}

func TestConnect_FailsWhenServerRejectsCert(t *testing.T) {
	realCA := newFakeCA(t)
	wrongCA := newFakeCA(t)
	server := startFakeSSHServer(t, realCA.signer.PublicKey(), "deploy") // server trusts realCA only

	target, err := domain.NewSshTarget("target-3", "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}

	issuer := &fakeIssuer{ca: wrongCA, principal: "deploy"} // signs with the WRONG CA
	connector := sshconn.NewConnector(issuer, nil, sshconn.Config{DialTimeout: 5 * time.Second}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := connector.Connect(ctx, target)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected Connect to fail when the fake server rejects a cert signed by an untrusted CA")
	}
}

// fakeSshTargetResolver satisfies sshconn.SshTargetResolver against an
// in-memory map — no postgres involved.
type fakeSshTargetResolver struct {
	byID map[string]domain.SshTarget
}

func (f *fakeSshTargetResolver) Get(_ context.Context, _, id string) (domain.SshTarget, error) {
	target, ok := f.byID[id]
	if !ok {
		return domain.SshTarget{}, fmt.Errorf("fakeSshTargetResolver: no target %q", id)
	}
	return target, nil
}

func TestConnect_DialsThroughJumpHost(t *testing.T) {
	ca := newFakeCA(t)
	bastion := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy")
	realTarget := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy")

	bastionTarget, err := domain.NewSshTarget("bastion-1", "tenant-1", "127.0.0.1", bastion.port(t), "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget(bastion): %v", err)
	}
	finalTarget, err := domain.NewSshTarget("target-jump", "tenant-1", "127.0.0.1", realTarget.port(t), "deploy", "role-1", "", "bastion-1", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget(final): %v", err)
	}

	resolver := &fakeSshTargetResolver{byID: map[string]domain.SshTarget{"bastion-1": bastionTarget}}
	issuer := &fakeIssuer{ca: ca, principal: "deploy"}
	connector := sshconn.NewConnector(issuer, resolver, sshconn.Config{DialTimeout: 5 * time.Second}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := connector.Connect(ctx, finalTarget)
	if err != nil {
		t.Fatalf("Connect through jump host: %v", err)
	}
	defer func() { _ = conn.Close() }()

	stdout, _, err := conn.RunCommand(ctx, "echo through-bastion")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if !strings.Contains(stdout, "echo through-bastion") {
		t.Errorf("stdout = %q, want it to contain the executed command (proves traffic reached the real target through the bastion)", stdout)
	}
}

func TestConnect_KnownHostsMismatchRejectsDial(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy")

	target, err := domain.NewSshTarget("target-mismatch", "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "SHA256:not-the-real-fingerprint", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}

	issuer := &fakeIssuer{ca: ca, principal: "deploy"}
	connector := sshconn.NewConnector(issuer, nil, sshconn.Config{DialTimeout: 5 * time.Second}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := connector.Connect(ctx, target)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected Connect to fail on a known-hosts fingerprint mismatch")
	}
	// The handshake failure is wrapped in ErrUnreachableHost (same as a dial
	// failure); the underlying cause is the fingerprint-mismatch error.
	var unreachable *sshconn.ErrUnreachableHost
	if !errors.As(err, &unreachable) {
		t.Fatalf("expected an *sshconn.ErrUnreachableHost, got %T: %v", err, err)
	}
	if !strings.Contains(unreachable.Cause.Error(), "fingerprint mismatch") {
		t.Errorf("expected the wrapped cause to mention a fingerprint mismatch, got: %v", unreachable.Cause)
	}
}

func TestErrUnreachableHost_ErrorMessages(t *testing.T) {
	refused := &sshconn.ErrUnreachableHost{Host: "10.0.0.1", Port: 22, Cause: errors.New("connect: connection refused")}
	if got, want := refused.Error(), "sshconn: connection refused: 10.0.0.1:22"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	timedOut := &sshconn.ErrUnreachableHost{Host: "10.0.0.1", Port: 22, Cause: context.DeadlineExceeded}
	if got, want := timedOut.Error(), "sshconn: connection to 10.0.0.1:22 timed out"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestConnect_UnreachableHostReturnsTypedError(t *testing.T) {
	// Nothing is listening on this OS-assigned-then-closed port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	_, portStr, _ := net.SplitHostPort(addr)
	var port int
	_, _ = fmt.Sscanf(portStr, "%d", &port)

	target, err := domain.NewSshTarget("target-unreachable", "tenant-1", "127.0.0.1", port, "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	issuer := &fakeIssuer{ca: newFakeCA(t), principal: "deploy"}
	connector := sshconn.NewConnector(issuer, nil, sshconn.Config{DialTimeout: 2 * time.Second}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = connector.Connect(ctx, target)
	if err == nil {
		t.Fatal("expected Connect to fail against an unreachable host")
	}
	var unreachable *sshconn.ErrUnreachableHost
	if !errors.As(err, &unreachable) {
		t.Fatalf("expected an *sshconn.ErrUnreachableHost, got %T: %v", err, err)
	}
}

func TestKeepAlive_SendsRequestsAtInterval(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy")

	target, err := domain.NewSshTarget("target-keepalive", "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
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

	keepAliveCtx, keepAliveCancel := context.WithCancel(context.Background())
	conn.StartKeepAlive(keepAliveCtx, 20*time.Millisecond)

	// Let a few intervals elapse — the fake server's ssh.DiscardRequests
	// replies to every global request, so SendRequest returning without
	// error at each tick proves the loop is actually running.
	time.Sleep(100 * time.Millisecond)

	// Close should stop the loop without leaking a goroutine — closing conn
	// twice (via Close and via keepAliveCancel) must both be safe.
	keepAliveCancel()
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
