package sshrelay_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/devserveragent"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshrelay"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// The fakes below mirror sshconn/connector_test.go's fakeCA/fakeIssuer/
// fakeSSHServer (unexported there, so duplicated+extended here rather than
// shared) — same "real, minimal, working counterpart, not a mock"
// philosophy, extended to also serve the SFTP subsystem (deploy) and a
// scripted exec handler that behaves like a real launched agent.js --stdio
// process would (checksum command, and a real frame-protocol handshake on
// the launch command) — genuine end-to-end coverage of
// deploy+launch+handshake, not three separately-mocked steps.

type fakeCA struct{ signer ssh.Signer }

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
		Key: pubKey, Serial: 1, CertType: ssh.UserCert,
		ValidPrincipals: []string{f.principal}, ValidAfter: 0, ValidBefore: ssh.CertTimeInfinity,
	}
	if err := cert.SignCert(rand.Reader, f.ca.signer); err != nil {
		return "", fmt.Errorf("fakeIssuer: signing certificate: %w", err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(cert))), nil
}

// fakeSSHServer serves one connection: any number of "exec" channels
// (mkdir/checksum/launch, each dispatched by command content) and
// "subsystem" sftp channels, all rooted at deployDir — the same directory
// deploy()'s uploaded agent.js and this server's checksum command both see,
// so the checksum step genuinely verifies what was actually uploaded.
type fakeSSHServer struct {
	listener  net.Listener
	deployDir string
	// badChecksum, if true, makes the checksum exec reply with a wrong hash
	// — exercises deploy()'s mismatch-detection path for real.
	badChecksum bool
	// reportedVersion, if non-empty, makes the exec handler answer
	// remoteVersionAndPresence's `require(...).AGENT_VERSION` probe with
	// this string instead of falling through to the default (empty-output)
	// branch — lets TestProvision_* exercise BR-SSH-07's version-gate
	// against a "server already has this exact version deployed" scenario.
	reportedVersion string
	// mkdirCalls counts real mkdir execs — TestProvision_VersionMatches_
	// SkipsDeploy asserts this stays 0 when the version gate skips deploy.
	mkdirCalls atomic.Int32
	// skipHandshake, if true, makes the "--connect"/"--stdio" exec handler
	// write a fake crash message to the SSH channel's stderr (extended data)
	// and then simply never send agent.handshake — simulates a launched
	// process that starts but never completes the handshake, driving
	// TestProvision_HandshakeTimeout_IncludesDiagnostics.
	skipHandshake bool
	// detachedStarted tracks whether a "--detach --sock-path" exec has run
	// — the fake server's stand-in for the real detached process actually
	// creating its Unix socket, so a later `test -S <path>` (reattach's
	// liveness probe) reports "alive" only after a real detach happened.
	detachedStarted atomic.Bool
}

func startFakeSSHServer(t *testing.T, trustedCAPub ssh.PublicKey, expectPrincipal string, badChecksum bool) *fakeSSHServer {
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
	srv := &fakeSSHServer{listener: listener, deployDir: t.TempDir(), badChecksum: badChecksum}
	t.Cleanup(func() { _ = listener.Close() })

	go srv.serve(t, cfg)
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

func (s *fakeSSHServer) serve(t *testing.T, cfg *ssh.ServerConfig) {
	for {
		rawConn, err := s.listener.Accept()
		if err != nil {
			return // listener closed at test cleanup
		}
		go s.handleConn(t, rawConn, cfg)
	}
}

func (s *fakeSSHServer) handleConn(t *testing.T, rawConn net.Conn, cfg *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(rawConn, cfg)
	if err != nil {
		_ = rawConn.Close()
		return
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
			go s.handleSessionRequests(t, channel, requests)
		default:
			_ = newChannel.Reject(ssh.UnknownChannelType, "only session channels supported")
		}
	}
}

func (s *fakeSSHServer) handleSessionRequests(t *testing.T, channel ssh.Channel, requests <-chan *ssh.Request) {
	defer func() { _ = channel.Close() }()
	for req := range requests {
		switch req.Type {
		case "exec":
			var execMsg struct{ Command string }
			_ = ssh.Unmarshal(req.Payload, &execMsg)
			_ = req.Reply(true, nil)
			s.handleExec(t, channel, execMsg.Command)
			return
		case "subsystem":
			var subMsg struct{ Name string }
			_ = ssh.Unmarshal(req.Payload, &subMsg)
			if subMsg.Name != "sftp" {
				_ = req.Reply(false, nil)
				continue
			}
			_ = req.Reply(true, nil)
			s.handleSFTP(t, channel)
			return
		default:
			_ = req.Reply(false, nil)
		}
	}
}

func exitStatus(channel ssh.Channel, code uint32) {
	_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{code}))
}

// handleExec dispatches by command content — this fake server doesn't
// implement a real shell, it recognizes exactly the three commands
// deploy.go/launch.go issue.
func (s *fakeSSHServer) handleExec(t *testing.T, channel ssh.Channel, cmd string) {
	switch {
	case strings.HasPrefix(cmd, "mkdir -p"):
		// deploy.go's actual mkdir command, e.g. `mkdir -p '.orca-relay'` —
		// really create it under s.deployDir so the SFTP upload that
		// follows (Create doesn't make parent dirs) has somewhere to land,
		// same as a real remote shell would.
		s.mkdirCalls.Add(1)
		dir := strings.Trim(strings.TrimSpace(strings.TrimPrefix(cmd, "mkdir -p")), "'\"")
		if err := os.MkdirAll(filepath.Join(s.deployDir, dir), 0o755); err != nil {
			exitStatus(channel, 1)
			return
		}
		exitStatus(channel, 0)
	case strings.Contains(cmd, "AGENT_VERSION"):
		// version_check.go's remoteVersionAndPresence probe:
		// `test -f <path> && node -e "...AGENT_VERSION..." || true`.
		if s.reportedVersion != "" {
			_, _ = channel.Write([]byte(s.reportedVersion))
		}
		exitStatus(channel, 0)
	case strings.Contains(cmd, "createHash('sha256')"):
		// Extract the path embedded in fs.readFileSync('<path>') rather than
		// assuming deploy.go's exact directory/file constants — reads
		// whatever the real command actually asked for, same as a real
		// remote node process would.
		const marker = "readFileSync('"
		start := strings.Index(cmd, marker)
		if start < 0 {
			exitStatus(channel, 1)
			return
		}
		start += len(marker)
		end := strings.Index(cmd[start:], "'")
		if end < 0 {
			exitStatus(channel, 1)
			return
		}
		remotePath := cmd[start : start+end]
		data, err := os.ReadFile(filepath.Join(s.deployDir, remotePath))
		if err != nil {
			exitStatus(channel, 1)
			return
		}
		sum := sha256.Sum256(data)
		hexSum := hex.EncodeToString(sum[:])
		if s.badChecksum {
			hexSum = "0000000000000000000000000000000000000000000000000000000000000000"
		}
		_, _ = channel.Write([]byte(hexSum))
		exitStatus(channel, 0)
	case strings.Contains(cmd, "--detach"):
		// launch.go's detach-start command: the parent blocks until the
		// (simulated) detached child reports listening, then exits 0 — the
		// fake server stands in for that by just flipping a flag other execs
		// on this same fake connection can observe.
		s.detachedStarted.Store(true)
		exitStatus(channel, 0)
	case strings.HasPrefix(cmd, "test -S"):
		// launch.go/reattach's liveness probe: `test -S <sockPath> && echo alive`.
		if s.detachedStarted.Load() {
			_, _ = channel.Write([]byte("alive"))
		}
		exitStatus(channel, 0)
	case strings.Contains(cmd, "--connect"), strings.Contains(cmd, "--stdio"):
		if s.skipHandshake {
			// Write to the channel's extended-data (stderr) stream — this is
			// what session.Stderr (launch.go's diagnosticStderr) receives on
			// the real Provisioner side.
			_, _ = channel.Stderr().Write([]byte("fatal: relay process crashed before handshake\n"))
			// Never send agent.handshake and never exit — the channel just
			// stays open with no response, forcing receiveHandshake's own
			// timeout to fire.
			return
		}
		s.runFakeAgentHandshake(t, channel)
		// Deliberately no exit-status here — a real launched process stays
		// running; the session/channel simply stays open as the live
		// transport, exactly like launch.go's caller expects.
	default:
		exitStatus(channel, 0)
	}
}

// runFakeAgentHandshake plays the AGENT's role in the handshake — the fake
// server proactively sends agent.handshake (matching agent-session.ts's
// real, unconditional "send handshake the instant the transport is open"
// behavior) and reads back Provisioner's {ok:true,...} reply, proving the
// receiver-side handshake in provisioner.go actually completes end to end.
func (s *fakeSSHServer) runFakeAgentHandshake(t *testing.T, channel ssh.Channel) {
	params, _ := json.Marshal(map[string]any{"devServerId": "ds-1", "platform": "linux", "arch": "x64", "agentVersion": "2.1.0"})
	req := devserveragent.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "agent.handshake", Params: params}
	frame, err := devserveragent.EncodeJSONRPCFrame(req, 1, 0)
	if err != nil {
		t.Errorf("fake agent: encoding handshake request: %v", err)
		return
	}
	if _, err := channel.Write(frame); err != nil {
		return // Provision may have already failed/closed — not a test failure by itself
	}

	buf := make([]byte, 4096)
	n, err := channel.Read(buf)
	if err != nil {
		return
	}
	decoded, err := devserveragent.DecodeFrame(buf[:n])
	if err != nil {
		t.Errorf("fake agent: decoding handshake response frame: %v", err)
		return
	}
	var resp devserveragent.JSONRPCResponse
	if err := json.Unmarshal(decoded.Payload, &resp); err != nil {
		t.Errorf("fake agent: unmarshaling handshake response: %v", err)
		return
	}
	if resp.Error != nil {
		t.Errorf("fake agent: handshake rejected: %+v", resp.Error)
	}
}

func (s *fakeSSHServer) handleSFTP(t *testing.T, channel ssh.Channel) {
	server, err := sftp.NewServer(channel, sftp.WithServerWorkingDirectory(s.deployDir))
	if err != nil {
		t.Errorf("starting fake sftp server: %v", err)
		return
	}
	_ = server.Serve() // returns when the client closes the sftp session — expected, not an error to report
}

// fakeSshTargetResolver is an in-memory sshrelay.SshTargetResolver.
type fakeSshTargetResolver struct {
	byID map[string]domain.SshTarget
	err  error
}

func (f *fakeSshTargetResolver) Get(_ context.Context, _, id string) (domain.SshTarget, error) {
	if f.err != nil {
		return domain.SshTarget{}, f.err
	}
	target, ok := f.byID[id]
	if !ok {
		return domain.SshTarget{}, fmt.Errorf("fakeSshTargetResolver: no ssh target %q", id)
	}
	return target, nil
}

func writeLocalBundle(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.js")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing local fake bundle: %v", err)
	}
	return path
}

func TestProvision_SucceedsAgainstFakeServer(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy", false)
	bundlePath := writeLocalBundle(t, "// fake agent bundle content\n")

	target, err := domain.NewSshTarget("ssht-1", "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	resolver := &fakeSshTargetResolver{byID: map[string]domain.SshTarget{"ssht-1": target}}
	connector := sshconn.NewConnector(&fakeIssuer{ca: ca, principal: "deploy"}, nil, sshconn.Config{DialTimeout: 5 * time.Second}, nil)
	provisioner := sshrelay.NewProvisioner(connector, resolver, sshrelay.Config{
		BundlePath: bundlePath, HandshakeTimeout: 5 * time.Second, OrcaVersion: "test",
	})

	devServer, err := domain.NewDevServer("ds-1", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-1")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	transport, info, err := provisioner.Provision(ctx, devServer)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	t.Cleanup(func() { _ = transport.Close("test done") })

	if info.Platform != "linux" || info.AgentVersion != "2.1.0" {
		t.Errorf("HandshakeInfo = %+v, want platform=linux agentVersion=2.1.0", info)
	}
	if info.SessionID == "" {
		t.Error("expected a non-empty SessionID")
	}
}

func TestProvision_FailsOnChecksumMismatch(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy", true) // badChecksum
	bundlePath := writeLocalBundle(t, "// fake agent bundle content\n")

	target, err := domain.NewSshTarget("ssht-2", "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	resolver := &fakeSshTargetResolver{byID: map[string]domain.SshTarget{"ssht-2": target}}
	connector := sshconn.NewConnector(&fakeIssuer{ca: ca, principal: "deploy"}, nil, sshconn.Config{DialTimeout: 5 * time.Second}, nil)
	provisioner := sshrelay.NewProvisioner(connector, resolver, sshrelay.Config{
		BundlePath: bundlePath, HandshakeTimeout: 5 * time.Second, OrcaVersion: "test",
	})

	devServer, err := domain.NewDevServer("ds-2", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-2")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _, err = provisioner.Provision(ctx, devServer)
	if err == nil {
		t.Fatal("expected Provision to fail on a checksum mismatch")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected a checksum-mismatch error, got: %v", err)
	}
}

func TestProvision_FailsWhenBundlePathNotConfigured(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy", false)

	target, err := domain.NewSshTarget("ssht-3", "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	resolver := &fakeSshTargetResolver{byID: map[string]domain.SshTarget{"ssht-3": target}}
	connector := sshconn.NewConnector(&fakeIssuer{ca: ca, principal: "deploy"}, nil, sshconn.Config{DialTimeout: 5 * time.Second}, nil)
	provisioner := sshrelay.NewProvisioner(connector, resolver, sshrelay.Config{HandshakeTimeout: 5 * time.Second}) // no BundlePath

	devServer, err := domain.NewDevServer("ds-3", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-3")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err = provisioner.Provision(ctx, devServer)
	if err == nil {
		t.Fatal("expected Provision to fail when BundlePath is not configured")
	}
}

func TestProvision_FailsWhenSshTargetUnresolvable(t *testing.T) {
	resolver := &fakeSshTargetResolver{err: fmt.Errorf("postgres: ssh target not found")}
	connector := sshconn.NewConnector(&fakeIssuer{}, nil, sshconn.Config{}, nil)
	provisioner := sshrelay.NewProvisioner(connector, resolver, sshrelay.Config{BundlePath: "/nonexistent", HandshakeTimeout: time.Second})

	devServer, err := domain.NewDevServer("ds-4", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-missing")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	_, _, err = provisioner.Provision(context.Background(), devServer)
	if err == nil {
		t.Fatal("expected Provision to fail when the ssh target can't be resolved")
	}
}

// TestProvision_VersionMatches_SkipsDeploy is TASK-SSH-02-04's regression:
// BR-SSH-07's version gate must skip the SFTP upload entirely when the
// remote's already-deployed AGENT_VERSION matches OrcaVersion.
func TestProvision_VersionMatches_SkipsDeploy(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy", false)
	server.reportedVersion = "2.1.0"
	bundlePath := writeLocalBundle(t, "// fake agent bundle content\n")

	target, err := domain.NewSshTarget("ssht-version-match", "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	resolver := &fakeSshTargetResolver{byID: map[string]domain.SshTarget{"ssht-version-match": target}}
	connector := sshconn.NewConnector(&fakeIssuer{ca: ca, principal: "deploy"}, nil, sshconn.Config{DialTimeout: 5 * time.Second}, nil)
	provisioner := sshrelay.NewProvisioner(connector, resolver, sshrelay.Config{
		BundlePath: bundlePath, HandshakeTimeout: 5 * time.Second, OrcaVersion: "2.1.0",
	})

	devServer, err := domain.NewDevServer("ds-version-match", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-version-match")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	transport, info, err := provisioner.Provision(ctx, devServer)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	t.Cleanup(func() { _ = transport.Close("test done") })

	if info.AgentVersion != "2.1.0" {
		t.Errorf("expected handshake to still complete, got info=%+v", info)
	}
	if got := server.mkdirCalls.Load(); got != 0 {
		t.Errorf("expected deploy() to be skipped entirely (0 mkdir calls) when the version already matches, got %d calls", got)
	}
}

// TestProvision_VersionMismatch_StillDeploys is the converse: an absent or
// mismatched remote version must fall through to deployWithRetry.
func TestProvision_VersionMismatch_StillDeploys(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy", false)
	server.reportedVersion = "1.0.0-old" // mismatches OrcaVersion below
	bundlePath := writeLocalBundle(t, "// fake agent bundle content\n")

	target, err := domain.NewSshTarget("ssht-version-mismatch", "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	resolver := &fakeSshTargetResolver{byID: map[string]domain.SshTarget{"ssht-version-mismatch": target}}
	connector := sshconn.NewConnector(&fakeIssuer{ca: ca, principal: "deploy"}, nil, sshconn.Config{DialTimeout: 5 * time.Second}, nil)
	provisioner := sshrelay.NewProvisioner(connector, resolver, sshrelay.Config{
		BundlePath: bundlePath, HandshakeTimeout: 5 * time.Second, OrcaVersion: "2.1.0",
	})

	devServer, err := domain.NewDevServer("ds-version-mismatch", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-version-mismatch")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	transport, _, err := provisioner.Provision(ctx, devServer)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	t.Cleanup(func() { _ = transport.Close("test done") })

	if got := server.mkdirCalls.Load(); got != 1 {
		t.Errorf("expected deploy() to run once on a version mismatch, got %d mkdir calls", got)
	}
}

// TestProvision_HandshakeTimeout_IncludesDiagnostics is TASK-SSH-02-07's
// regression: a handshake timeout must surface collectDiagnostics' output,
// not a bare timeout error.
func TestProvision_HandshakeTimeout_IncludesDiagnostics(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy", false)
	server.skipHandshake = true
	bundlePath := writeLocalBundle(t, "// fake agent bundle content\n")

	target, err := domain.NewSshTarget("ssht-handshake-timeout", "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	resolver := &fakeSshTargetResolver{byID: map[string]domain.SshTarget{"ssht-handshake-timeout": target}}
	connector := sshconn.NewConnector(&fakeIssuer{ca: ca, principal: "deploy"}, nil, sshconn.Config{DialTimeout: 5 * time.Second}, nil)
	provisioner := sshrelay.NewProvisioner(connector, resolver, sshrelay.Config{
		BundlePath: bundlePath, HandshakeTimeout: 500 * time.Millisecond, OrcaVersion: "test",
	})

	devServer, err := domain.NewDevServer("ds-handshake-timeout", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-handshake-timeout")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _, err = provisioner.Provision(ctx, devServer)
	if err == nil {
		t.Fatal("expected Provision to fail when the handshake never arrives")
	}
	for _, want := range []string{"os=", "arch=", "node=", "user=", "stderr_tail="} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected the error to include diagnostics marker %q, got: %v", want, err)
		}
	}
	if !strings.Contains(err.Error(), "relay process crashed before handshake") {
		t.Errorf("expected the error to include the captured stderr, got: %v", err)
	}
}
