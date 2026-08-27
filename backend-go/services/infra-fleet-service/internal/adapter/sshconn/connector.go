// Package sshconn is the connection-establishment half of relay-ssh mode —
// generating an ephemeral SSH keypair, exchanging it for a Vault-signed
// short-lived certificate, and dialing a real SSH connection to a
// domain.SshTarget, per the "Preferred: Vault's SSH secrets engine" model in
// specs/backend-go/services/infra-fleet-service.md §9.
//
// Connection.SFTPClient/NewSession are what adapter/sshrelay builds its
// deploy (SFTP-upload agent/out/agent.js) and launch (SSH exec channel,
// `node agent.js --stdio`) steps on, over this same already-authenticated
// connection — see that package's doc comment for the rest of relay-ssh
// mode, which this package only establishes the transport for.
//
// Connect resolves target.JumpHostTargetID into a full hop chain (jumphost.go)
// and dials through each bastion in turn; host-key verification uses
// target.KnownHostsFingerprint when set, falling back to
// ssh.InsecureIgnoreHostKey() (documented, opt-in degrade) when not — see
// hostKeyCallback. Per-target Port (domain.SshTarget.Port) is honored,
// defaulting to defaultSSHPort when zero.
package sshconn

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// defaultSSHPort is the fallback dial port when neither the target nor
// Config specifies one.
const defaultSSHPort = 22

// SSHCertIssuer is the narrow port sshconn needs from common/secrets —
// defined here (consumer-side), not in common/secrets, per this codebase's
// existing Dependency Inversion convention (see e.g.
// infra-fleet-service/internal/usecase/ports.go's own doc comment on why
// ports are defined where they're consumed, not where they're implemented).
type SSHCertIssuer interface {
	// SSHSignPublicKey signs publicKeyOpenSSH under the named Vault SSH
	// secrets engine role, returning a short-lived signed certificate in
	// OpenSSH authorized-key format. See common/secrets.Client's method of
	// the same name, which is what production wires in here.
	SSHSignPublicKey(ctx context.Context, role, publicKeyOpenSSH string) (string, error)
}

// Config tunes Connector's dial behavior — mirrors
// adapter/devserveragent.Config's Config/DefaultConfig()/LoadConfigFromEnv()
// convention (see config.go there) rather than hardcoding magic numbers.
type Config struct {
	// DialTimeout bounds TCP connect plus the SSH handshake (key exchange +
	// certificate auth) to each hop.
	DialTimeout time.Duration
	// Port is the fallback TCP port used when a target's own Port field is
	// zero. Defaults to 22 (defaultSSHPort) via DefaultConfig.
	Port int
}

// DefaultConfig returns Config with a conservative default dial timeout and
// the standard SSH port; callers override as needed.
func DefaultConfig() Config {
	return Config{
		DialTimeout: 10 * time.Second,
		Port:        defaultSSHPort,
	}
}

// LoadConfigFromEnv reads SSHCONN_DIAL_TIMEOUT_MS on top of DefaultConfig —
// same override-one-knob-via-env shape as devserveragent.LoadConfigFromEnv.
func LoadConfigFromEnv() Config {
	cfg := DefaultConfig()
	if v := os.Getenv("SSHCONN_DIAL_TIMEOUT_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			cfg.DialTimeout = time.Duration(ms) * time.Millisecond
		}
	}
	return cfg
}

// Connector establishes real SSH connections to domain.SshTarget hosts,
// authenticating with an ephemeral keypair + Vault-signed certificate —
// never a stored or reused private key. See package doc comment for scope.
type Connector struct {
	issuer   SSHCertIssuer
	resolver SshTargetResolver
	cfg      Config
	cap      *Cap // nil = no concurrent-connection cap (e.g. in tests)
}

// NewConnector builds a Connector. issuer is typically a *secrets.Client
// (common/secrets), narrowed to the SSHCertIssuer port here per this
// codebase's Dependency Inversion convention. resolver resolves jump-host
// chains (typically postgres.SshTargetStore); cap is the shared
// concurrent-connection cap (nil disables the cap, e.g. in tests). A
// zero-value cfg.Port defaults to defaultSSHPort (22), same as
// DefaultConfig().
func NewConnector(issuer SSHCertIssuer, resolver SshTargetResolver, cfg Config, cap *Cap) *Connector {
	if cfg.Port == 0 {
		cfg.Port = defaultSSHPort
	}
	return &Connector{issuer: issuer, resolver: resolver, cfg: cfg, cap: cap}
}

// Connect establishes a real SSH connection to target, walking through any
// jump-host chain (target.JumpHostTargetID) via c.resolver, dialing each hop
// through the previous one (bastion-first order — see resolveJumpChain).
// Each hop:
//  1. generates an ephemeral ed25519 keypair in-memory (never persisted,
//     never logged);
//  2. requests issuer.SSHSignPublicKey(ctx, hop.VaultSSHRole, <marshaled
//     pubkey>) to get a short-lived certificate;
//  3. builds an ssh.ClientConfig using ssh.NewCertSigner over the ephemeral
//     private key + signed cert, with host-key verification via
//     hop.KnownHostsFingerprint when set (InsecureIgnoreHostKey otherwise —
//     documented, opt-in degrade);
//  4. dials hop.Host:hop.Port (defaults to c.cfg.Port/22 when hop.Port is 0).
//
// If c.cap is set, Connect first acquires a (tenantID, host) slot — see
// pool.go — rejecting the 11th concurrent connection to the same host
// before ever dialing.
func (c *Connector) Connect(ctx context.Context, target domain.SshTarget) (*Connection, error) {
	if c.cap != nil {
		release, err := c.cap.Acquire(target.TenantID, target.Host)
		if err != nil {
			return nil, err
		}
		defer release()
	}

	hops, err := resolveJumpChain(ctx, c.resolver, target.TenantID, target)
	if err != nil {
		return nil, err
	}

	var current *ssh.Client
	for i, hop := range hops {
		clientConfig, err := c.buildClientConfig(ctx, hop)
		if err != nil {
			return nil, err
		}
		addr := targetAddr(hop)
		if current == nil {
			dialer := &net.Dialer{Timeout: c.cfg.DialTimeout}
			rawConn, dialErr := dialer.DialContext(ctx, "tcp", addr)
			if dialErr != nil {
				return nil, &ErrUnreachableHost{Host: hop.Host, Port: hop.Port, HopIndex: i, Cause: dialErr}
			}
			sshConn, chans, reqs, hsErr := ssh.NewClientConn(rawConn, addr, clientConfig)
			if hsErr != nil {
				_ = rawConn.Close()
				return nil, &ErrUnreachableHost{Host: hop.Host, Port: hop.Port, HopIndex: i, Cause: hsErr}
			}
			current = ssh.NewClient(sshConn, chans, reqs)
		} else {
			netConn, dialErr := current.Dial("tcp", addr)
			if dialErr != nil {
				return nil, &ErrUnreachableHost{Host: hop.Host, Port: hop.Port, HopIndex: i, Cause: dialErr}
			}
			sshConn, chans, reqs, hsErr := ssh.NewClientConn(netConn, addr, clientConfig)
			if hsErr != nil {
				_ = netConn.Close()
				return nil, &ErrUnreachableHost{Host: hop.Host, Port: hop.Port, HopIndex: i, Cause: hsErr}
			}
			current = ssh.NewClient(sshConn, chans, reqs)
		}
	}
	return &Connection{client: current, closeCh: make(chan struct{})}, nil
}

// buildClientConfig runs the ephemeral-keypair + Vault-cert issuance steps
// for one hop, then sets HostKeyCallback via hostKeyCallback(hop).
func (c *Connector) buildClientConfig(ctx context.Context, hop domain.SshTarget) (*ssh.ClientConfig, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("sshconn: generating ephemeral keypair: %w", err)
	}

	signer, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		return nil, fmt.Errorf("sshconn: wrapping ephemeral private key: %w", err)
	}
	pubKeySSH, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("sshconn: marshaling ephemeral public key: %w", err)
	}
	authorizedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pubKeySSH)))

	signedCert, err := c.issuer.SSHSignPublicKey(ctx, hop.VaultSSHRole, authorizedKey)
	if err != nil {
		return nil, fmt.Errorf("sshconn: requesting Vault SSH cert for role %s: %w", hop.VaultSSHRole, err)
	}
	if strings.TrimSpace(signedCert) == "" {
		return nil, fmt.Errorf("sshconn: Vault returned an empty signed certificate for role %s", hop.VaultSSHRole)
	}

	certPubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(signedCert))
	if err != nil {
		return nil, fmt.Errorf("sshconn: parsing Vault-signed certificate: %w", err)
	}
	cert, ok := certPubKey.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("sshconn: Vault response for role %s was not an SSH certificate", hop.VaultSSHRole)
	}

	certSigner, err := ssh.NewCertSigner(cert, signer)
	if err != nil {
		return nil, fmt.Errorf("sshconn: building certificate signer: %w", err)
	}

	return &ssh.ClientConfig{
		User:            hop.UserName,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(certSigner)},
		HostKeyCallback: hostKeyCallback(hop),
		Timeout:         c.cfg.DialTimeout,
	}, nil
}

// Connection wraps a live, authenticated SSH connection to one target.
type Connection struct {
	client   *ssh.Client
	closeCh  chan struct{}
	closeOne sync.Once
}

// RunCommand runs cmd in a fresh SSH session and returns its stdout/stderr —
// the "verify this connection is actually alive and can execute something"
// primitive a future relay-ssh deploy step (or a simple health check) would
// build on. NOT wired into GetFleetHealth or any other usecase this pass —
// see this package's doc comment for why.
func (conn *Connection) RunCommand(ctx context.Context, cmd string) (stdout, stderr string, err error) {
	session, err := conn.client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("sshconn: opening session: %w", err)
	}
	defer func() { _ = session.Close() }()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	done := make(chan error, 1)
	go func() { done <- session.Run(cmd) }()

	select {
	case <-ctx.Done():
		_ = session.Close()
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("sshconn: run command %q: %w", cmd, ctx.Err())
	case runErr := <-done:
		if runErr != nil {
			return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("sshconn: run command %q: %w", cmd, runErr)
		}
		return stdoutBuf.String(), stderrBuf.String(), nil
	}
}

// keepAlive sends an SSH keepalive@openssh.com global request every interval
// until ctx is cancelled or the connection is closed — matches the spec's
// ServerAliveInterval (30s). A missed write means the connection is dead;
// the caller (sshrelay.Provisioner, right after Connect succeeds) starting
// this loop is what feeds a drop into BUG-SSH-03's reconnect detection
// (SOL-SSH-03), same "who starts it" placement as
// devserveragent/session.go's keepAliveLoop, one layer lower.
func (conn *Connection) keepAlive(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, _, err := conn.client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				return // caller's next operation on this Connection will observe the same failure
			}
		case <-ctx.Done():
			return
		case <-conn.closeCh:
			return
		}
	}
}

// StartKeepAlive launches keepAlive in a goroutine — separated from
// Connect() itself so a caller that doesn't want the loop (e.g. a
// short-lived probe connection) can opt out.
func (conn *Connection) StartKeepAlive(ctx context.Context, interval time.Duration) {
	go conn.keepAlive(ctx, interval)
}

// Close closes the underlying SSH connection and stops any running
// keepalive loop.
func (conn *Connection) Close() error {
	conn.closeOne.Do(func() { close(conn.closeCh) })
	return conn.client.Close()
}

// NewSession opens a fresh SSH session over this connection — the
// lower-level primitive RunCommand itself uses, exposed so
// adapter/sshrelay can drive an exec channel directly (wiring its own
// Stdin/Stdout pipes for the agent.js --stdio process) instead of
// RunCommand's own buffer-and-wait shape, which doesn't fit a long-lived
// bidirectional process.
func (conn *Connection) NewSession() (*ssh.Session, error) {
	session, err := conn.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("sshconn: opening session: %w", err)
	}
	return session, nil
}

// SFTPClient opens an SFTP subsystem client over this connection —
// adapter/sshrelay's deploy step uses it to upload agent/out/agent.js.
// Callers must Close() the returned client when done.
func (conn *Connection) SFTPClient() (*sftp.Client, error) {
	client, err := sftp.NewClient(conn.client)
	if err != nil {
		return nil, fmt.Errorf("sshconn: opening sftp client: %w", err)
	}
	return client, nil
}
