package devserveragent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// The fakes below mirror sshconn/connector_test.go's fakeCA/fakeIssuer/
// fakeSSHServer (unexported there, so duplicated here rather than shared) —
// same "real, minimal, working counterpart, not a mock" philosophy as
// devserveragent/client_test.go's fakeAgent, applied to relay-ssh's Health/
// Exec instead of the WS modes'.

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

type fakeIssuer struct {
	ca        *fakeCA
	principal string
}

func (f *fakeIssuer) SSHSignPublicKey(_ context.Context, _ string, publicKeyOpenSSH string) (string, error) {
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
// server-side API) that accepts a single "session" channel per connection
// and echoes back the exec'd command so tests can assert on the exact
// command string Client sent (env exports + script), with exit status 0.
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
		go handleFakeSSHSessionRequests(channel, requests)
	}
}

func handleFakeSSHSessionRequests(channel ssh.Channel, requests <-chan *ssh.Request) {
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
			_, _ = channel.Write([]byte("ran: " + execMsg.Command))
			_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
			return
		default:
			_ = req.Reply(false, nil)
		}
	}
}

// fakeSshTargetResolver is an in-memory usecase.SshTargetResolver.
type fakeSshTargetResolver struct {
	byID map[string]domain.SshTarget
}

func (f *fakeSshTargetResolver) Get(_ context.Context, _, id string) (domain.SshTarget, error) {
	target, ok := f.byID[id]
	if !ok {
		return domain.SshTarget{}, fmt.Errorf("fakeSshTargetResolver: no ssh target %q", id)
	}
	return target, nil
}

func newRelaySSHTestClient(t *testing.T, connector *sshconn.Connector, resolver *fakeSshTargetResolver) *Client {
	t.Helper()
	client := New(testConfig(0, ""), slog.Default(), WithRelaySSH(connector, resolver))
	t.Cleanup(client.Close)
	return client
}

func TestClientHealth_RelaySSH_SucceedsAgainstFakeServer(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy")

	target, err := domain.NewSshTarget("ssht-1", "tenant-1", "127.0.0.1", "deploy", "role-1")
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	resolver := &fakeSshTargetResolver{byID: map[string]domain.SshTarget{"ssht-1": target}}
	connector := sshconn.NewConnector(&fakeIssuer{ca: ca, principal: "deploy"}, sshconn.Config{DialTimeout: 5 * time.Second, Port: server.port(t)})
	client := newRelaySSHTestClient(t, connector, resolver)

	devServer, err := domain.NewDevServer("ds-relay-ssh-1", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-1")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	healthy, err := client.Health(context.Background(), devServer)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !healthy {
		t.Error("Health = false, want true against a live fake SSH server")
	}
}

func TestClientHealth_RelaySSH_FalseWhenConnectionFails(t *testing.T) {
	ca := newFakeCA(t)
	// No server listening on this port — Connect must fail to dial.
	target, err := domain.NewSshTarget("ssht-2", "tenant-1", "127.0.0.1", "deploy", "role-1")
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	resolver := &fakeSshTargetResolver{byID: map[string]domain.SshTarget{"ssht-2": target}}
	connector := sshconn.NewConnector(&fakeIssuer{ca: ca, principal: "deploy"}, sshconn.Config{DialTimeout: 1 * time.Second, Port: 1})
	client := newRelaySSHTestClient(t, connector, resolver)

	devServer, err := domain.NewDevServer("ds-relay-ssh-2", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-2")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	healthy, err := client.Health(context.Background(), devServer)
	if err != nil {
		t.Fatalf("Health: %v, want (false, nil) for an unreachable relay-ssh target", err)
	}
	if healthy {
		t.Error("Health = true, want false when the SSH connection fails")
	}
}

func TestClientHealth_RelaySSH_FalseWhenNotEnabled(t *testing.T) {
	client := New(testConfig(0, ""), slog.Default()) // no WithRelaySSH
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-relay-ssh-3", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-3")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	healthy, err := client.Health(context.Background(), devServer)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if healthy {
		t.Error("Health = true, want false when relay-ssh support was never enabled via WithRelaySSH")
	}
}

func TestClientExec_RelaySSH_ShellExecSucceedsAgainstFakeServer(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy")

	target, err := domain.NewSshTarget("ssht-4", "tenant-1", "127.0.0.1", "deploy", "role-1")
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	resolver := &fakeSshTargetResolver{byID: map[string]domain.SshTarget{"ssht-4": target}}
	connector := sshconn.NewConnector(&fakeIssuer{ca: ca, principal: "deploy"}, sshconn.Config{DialTimeout: 5 * time.Second, Port: server.port(t)})
	client := newRelaySSHTestClient(t, connector, resolver)

	devServer, err := domain.NewDevServer("ds-relay-ssh-4", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-4")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	result, err := client.Exec(context.Background(), devServer, "shell.exec", map[string]any{
		"script": "echo hi",
		"env":    map[string]any{"FOO": "bar"},
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if result["exitCode"] != 0 {
		t.Errorf("exitCode = %v, want 0", result["exitCode"])
	}
	stdout, _ := result["stdout"].(string)
	if !strings.Contains(stdout, "echo hi") {
		t.Errorf("stdout = %q, want it to contain the script", stdout)
	}
	if !strings.Contains(stdout, "export FOO='bar'") {
		t.Errorf("stdout = %q, want it to contain the env export", stdout)
	}
}

func TestClientExec_RelaySSH_UnsupportedMethodReturnsTypedError(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy")

	target, err := domain.NewSshTarget("ssht-5", "tenant-1", "127.0.0.1", "deploy", "role-1")
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	resolver := &fakeSshTargetResolver{byID: map[string]domain.SshTarget{"ssht-5": target}}
	connector := sshconn.NewConnector(&fakeIssuer{ca: ca, principal: "deploy"}, sshconn.Config{DialTimeout: 5 * time.Second, Port: server.port(t)})
	client := newRelaySSHTestClient(t, connector, resolver)

	devServer, err := domain.NewDevServer("ds-relay-ssh-5", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-5")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	_, err = client.Exec(context.Background(), devServer, "ports.scan", nil)
	if err == nil {
		t.Fatal("expected an error for a relay-ssh method other than shell.exec")
	}
	if !errors.Is(err, ErrRelaySSHMethodNotSupported) {
		t.Errorf("expected ErrRelaySSHMethodNotSupported, got %v", err)
	}
}

func TestClientExec_RelaySSH_NotEnabledReturnsTypedError(t *testing.T) {
	client := New(testConfig(0, ""), slog.Default()) // no WithRelaySSH
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-relay-ssh-6", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-6")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	_, err = client.Exec(context.Background(), devServer, "shell.exec", map[string]any{"script": "echo hi"})
	if err == nil {
		t.Fatal("expected an error when relay-ssh support was never enabled via WithRelaySSH")
	}
	if !errors.Is(err, ErrConnectionModeNotImplemented) {
		t.Errorf("expected ErrConnectionModeNotImplemented, got %v", err)
	}
}
