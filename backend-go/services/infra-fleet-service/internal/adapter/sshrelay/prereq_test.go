package sshrelay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestParseAndCompareVersion(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		min        version
		wantOK     bool
		wantParsed string
	}{
		{"node ok", "v22.3.0\n", minNodeVersion, true, "22.3.0"},
		{"node too old", "v18.0.0\n", minNodeVersion, false, "18.0.0"},
		{"node exactly min", "v22.0.0\n", minNodeVersion, true, "22.0.0"},
		{"git ok", "git version 2.39.2\n", minGitVersion, true, "2.39.2"},
		{"git too old", "git version 2.20.1\n", minGitVersion, false, "2.20.1"},
		{"git exactly min", "git version 2.25.0\n", minGitVersion, true, "2.25.0"},
		{"unparseable", "command not found\n", minNodeVersion, false, "command not found"},
		{"empty", "", minNodeVersion, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, ok := parseAndCompareVersion(tt.output, tt.min)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if parsed != tt.wantParsed {
				t.Errorf("parsed = %q, want %q", parsed, tt.wantParsed)
			}
		})
	}
}

func TestParseDiskKB(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantOK     bool
		wantFreeGB float64
	}{
		{"sufficient", "10485760\n", true, 10}, // 10 GiB in 1024-byte blocks
		{"exactly at minimum", fmt.Sprintf("%d\n", int64(minFreeDiskGB*1024*1024)), true, minFreeDiskGB},
		{"insufficient", "1048576\n", false, 1}, // 1 GiB
		{"unparseable", "not-a-number\n", false, 0},
		{"empty", "", false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			freeGB, ok := parseDiskKB(tt.output)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if freeGB != tt.wantFreeGB {
				t.Errorf("freeGB = %v, want %v", freeGB, tt.wantFreeGB)
			}
		})
	}
}

// --- fake SSH server for checkPrerequisites' end-to-end coverage ---
//
// Mirrors sshconn/connector_test.go's fakeCA/fakeIssuer/fakeSSHServer
// pattern (unexported there, so duplicated+minimized here rather than
// shared — same "real, minimal, working counterpart, not a mock"
// philosophy already established by provisioner_test.go's own copy) —
// dispatches exec commands by content, only the three commands
// checkPrerequisites issues.

type prereqFakeCA struct{ signer ssh.Signer }

func newPrereqFakeCA(t *testing.T) *prereqFakeCA {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating fake CA keypair: %v", err)
	}
	signer, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		t.Fatalf("wrapping fake CA signer: %v", err)
	}
	return &prereqFakeCA{signer: signer}
}

type prereqFakeIssuer struct {
	ca        *prereqFakeCA
	principal string
}

func (f *prereqFakeIssuer) SSHSignPublicKey(_ context.Context, _ string, publicKeyOpenSSH string) (string, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKeyOpenSSH))
	if err != nil {
		return "", fmt.Errorf("prereqFakeIssuer: parsing public key to sign: %w", err)
	}
	cert := &ssh.Certificate{
		Key: pubKey, Serial: 1, CertType: ssh.UserCert,
		ValidPrincipals: []string{f.principal}, ValidAfter: 0, ValidBefore: ssh.CertTimeInfinity,
	}
	if err := cert.SignCert(rand.Reader, f.ca.signer); err != nil {
		return "", fmt.Errorf("prereqFakeIssuer: signing certificate: %w", err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(cert))), nil
}

// prereqFakeSSHServer replies to exec requests with canned output keyed by
// command content — responses is a map from an exact command string to the
// stdout it should produce; a command with no entry gets empty output
// (exercises the unparseable-output path).
type prereqFakeSSHServer struct {
	listener  net.Listener
	responses map[string]string
}

func startPrereqFakeSSHServer(t *testing.T, trustedCAPub ssh.PublicKey, expectPrincipal string, responses map[string]string) *prereqFakeSSHServer {
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
		if conn.User() != expectPrincipal {
			return nil, fmt.Errorf("unexpected user %q", conn.User())
		}
		return checker.Authenticate(conn, key)
	}}
	cfg.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	srv := &prereqFakeSSHServer{listener: listener, responses: responses}
	t.Cleanup(func() { _ = listener.Close() })

	go srv.serve(cfg)
	return srv
}

func (s *prereqFakeSSHServer) port(t *testing.T) int {
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

func (s *prereqFakeSSHServer) serve(cfg *ssh.ServerConfig) {
	for {
		rawConn, err := s.listener.Accept()
		if err != nil {
			return // listener closed at test cleanup
		}
		go s.handleConn(rawConn, cfg)
	}
}

func (s *prereqFakeSSHServer) handleConn(rawConn net.Conn, cfg *ssh.ServerConfig) {
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
		go s.handleSessionRequests(channel, requests)
	}
}

func (s *prereqFakeSSHServer) handleSessionRequests(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer func() { _ = channel.Close() }()
	for req := range requests {
		if req.Type != "exec" {
			_ = req.Reply(false, nil)
			continue
		}
		var execMsg struct{ Command string }
		_ = ssh.Unmarshal(req.Payload, &execMsg)
		_ = req.Reply(true, nil)
		_, _ = channel.Write([]byte(s.responses[execMsg.Command]))
		_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
		return
	}
}

func connectPrereqFakeServer(t *testing.T, responses map[string]string) *sshconn.Connection {
	t.Helper()
	ca := newPrereqFakeCA(t)
	server := startPrereqFakeSSHServer(t, ca.signer.PublicKey(), "deploy", responses)

	target, err := domain.NewSshTarget("target-1", "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	issuer := &prereqFakeIssuer{ca: ca, principal: "deploy"}
	connector := sshconn.NewConnector(issuer, nil, sshconn.Config{DialTimeout: 5 * time.Second}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := connector.Connect(ctx, target)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

const diskCmd = "df -P ~ | tail -1 | awk '{print $4}'"

func TestCheckPrerequisites_AllOK(t *testing.T) {
	conn := connectPrereqFakeServer(t, map[string]string{
		"node --version": "v22.3.0\n",
		"git --version":  "git version 2.39.2\n",
		diskCmd:          "10485760\n", // 10 GiB
	})

	result, err := checkPrerequisites(context.Background(), conn)
	if err != nil {
		t.Fatalf("checkPrerequisites: %v", err)
	}
	if !result.NodeOK || !result.GitOK || !result.DiskOK {
		t.Errorf("expected all checks OK, got %+v", result)
	}
	if !result.Met() {
		t.Error("expected Met() to be true")
	}
	if result.NodeVersion != "22.3.0" || result.GitVersion != "2.39.2" {
		t.Errorf("expected parsed versions to round-trip, got %+v", result)
	}
}

func TestCheckPrerequisites_OldNodeVersion(t *testing.T) {
	conn := connectPrereqFakeServer(t, map[string]string{
		"node --version": "v18.0.0\n",
		"git --version":  "git version 2.39.2\n",
		diskCmd:          "10485760\n",
	})

	result, err := checkPrerequisites(context.Background(), conn)
	if err != nil {
		t.Fatalf("checkPrerequisites: %v", err)
	}
	if result.NodeOK {
		t.Error("expected NodeOK=false for v18.0.0")
	}
	if !result.GitOK || !result.DiskOK {
		t.Errorf("expected git/disk still OK, got %+v", result)
	}
	if result.Met() {
		t.Error("expected Met() to be false when any check fails")
	}
}

func TestCheckPrerequisites_UnparseableOutputIsNotOKNotACrash(t *testing.T) {
	conn := connectPrereqFakeServer(t, map[string]string{
		"node --version": "garbage output\n",
		"git --version":  "git version 2.39.2\n",
		diskCmd:          "not-a-number\n",
	})

	result, err := checkPrerequisites(context.Background(), conn)
	if err != nil {
		t.Fatalf("checkPrerequisites: %v", err)
	}
	if result.NodeOK {
		t.Error("expected NodeOK=false for unparseable node output")
	}
	if result.DiskOK {
		t.Error("expected DiskOK=false for unparseable disk output")
	}
	if !result.GitOK {
		t.Error("expected GitOK=true (git output was well-formed)")
	}
}
