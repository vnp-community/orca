# TASK-SSH-01-06: `sshconn.Connector.Connect` — per-target port, jump-host chaining, known-hosts verification, typed unreachable-host error

**From Solution:** SOL-SSH-01
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/sshconn/connector.go`
**Depends on:** TASK-SSH-01-02
**Status:** `[x] DONE — jumphost.go (resolveJumpChain, hostKeyCallback, ErrUnreachableHost) + Connect rewritten to dial per-target port through jump chain with known-hosts verification; new tests (jump-host dial, known-hosts mismatch, unreachable-host typed error) pass`

---

## Context

`Connector.Connect` today (`connector.go:137-196`) always dials port
`c.cfg.Port` (deployment-wide, not per-target), always uses
`ssh.InsecureIgnoreHostKey()`, and never walks a jump-host chain — despite
`domain.SshTarget` now carrying `Port`/`KnownHostsFingerprint`/
`JumpHostTargetID` (TASK-SSH-01-02). This closes BUG-SSH-01's A2/A3 findings
and BR-SSH-04's jump-host requirement.

## Changes to make

Add a new file `backend-go/services/infra-fleet-service/internal/adapter/sshconn/jumphost.go`:

```go
package sshconn

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"golang.org/x/crypto/ssh"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ErrUnreachableHost is Connect's typed error for a dial/handshake failure
// against one hop in the chain — the usecase layer maps this to
// apperrors.KindFailedPrecondition with a stable INFRA_SSH_UNREACHABLE code
// instead of establish_connection.go's current generic
// INFRA_SSH_CONNECT_FAILED for this specific cause (BUG-SSH-01 A2).
type ErrUnreachableHost struct {
	Host     string
	Port     int
	HopIndex int
	Cause    error
}

func (e *ErrUnreachableHost) Error() string {
	if timedOut(e.Cause) {
		return fmt.Sprintf("sshconn: connection to %s:%d timed out", e.Host, e.Port)
	}
	return fmt.Sprintf("sshconn: connection refused: %s:%d", e.Host, e.Port)
}

func (e *ErrUnreachableHost) Unwrap() error { return e.Cause }

func timedOut(err error) bool {
	return err != nil && err == context.DeadlineExceeded //nolint:errorlint // exact sentinel match is intended here
}

// SshTargetResolver resolves a jump-host chain's parent targets — narrowed
// to the one method this package needs, satisfied by postgres.SshTargetStore.
type SshTargetResolver interface {
	Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error)
}

// resolveJumpChain walks target.JumpHostTargetID back to its root, returning
// hops in dial order (root first, target itself last) — the standard
// dial-through-a-bastion order. Guards against a cycle with a bounded
// walk (max 8 hops) rather than an unbounded loop.
func resolveJumpChain(ctx context.Context, resolver SshTargetResolver, tenantID string, target domain.SshTarget) ([]domain.SshTarget, error) {
	hops := []domain.SshTarget{target}
	current := target
	for i := 0; i < 8 && current.JumpHostTargetID != ""; i++ {
		parent, err := resolver.Get(ctx, tenantID, current.JumpHostTargetID)
		if err != nil {
			return nil, fmt.Errorf("sshconn: resolving jump host %q: %w", current.JumpHostTargetID, err)
		}
		hops = append([]domain.SshTarget{parent}, hops...)
		current = parent
	}
	return hops, nil
}

func hostKeyCallback(target domain.SshTarget) ssh.HostKeyCallback {
	if target.KnownHostsFingerprint == "" {
		return ssh.InsecureIgnoreHostKey() //nolint:gosec // explicit, logged degrade — see Connect's caller
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fp := ssh.FingerprintSHA256(key)
		if fp != target.KnownHostsFingerprint {
			return fmt.Errorf("sshconn: host key fingerprint mismatch for %s: got %s, want %s", hostname, fp, target.KnownHostsFingerprint)
		}
		return nil
	}
}

func targetAddr(target domain.SshTarget) string {
	port := target.Port
	if port == 0 {
		port = defaultSSHPort
	}
	return net.JoinHostPort(target.Host, strconv.Itoa(port))
}
```

Rewrite `Connect` in `connector.go` to take a `resolver SshTargetResolver`
(added to `Connector`'s fields and `NewConnector`'s params), walk the chain,
and dial each hop through the previous one:

```go
type Connector struct {
	issuer   SSHCertIssuer
	resolver SshTargetResolver
	cfg      Config
	cap      *Cap // TASK-SSH-01-07; nil = no cap
}

func NewConnector(issuer SSHCertIssuer, resolver SshTargetResolver, cfg Config, cap *Cap) *Connector {
	if cfg.Port == 0 {
		cfg.Port = defaultSSHPort
	}
	return &Connector{issuer: issuer, resolver: resolver, cfg: cfg, cap: cap}
}

func (c *Connector) Connect(ctx context.Context, target domain.SshTarget) (*Connection, error) {
	hops, err := resolveJumpChain(ctx, c.resolver, target.TenantID, target)
	if err != nil {
		return nil, err
	}

	var current *ssh.Client
	for i, hop := range hops {
		clientConfig, err := c.buildClientConfig(ctx, hop) // extracted from today's Connect body: keypair+Vault-cert steps, unchanged
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
	return &Connection{client: current}, nil
}

// buildClientConfig runs today's Connect body's ephemeral-keypair + Vault-cert
// steps (connector.go:138-180, unchanged), then sets HostKeyCallback via
// hostKeyCallback(hop) instead of the hardcoded ssh.InsecureIgnoreHostKey().
func (c *Connector) buildClientConfig(ctx context.Context, hop domain.SshTarget) (*ssh.ClientConfig, error) {
	// ... identical body to today's Connect through certSigner construction ...
	return &ssh.ClientConfig{
		User:            hop.UserName,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(certSigner)},
		HostKeyCallback: hostKeyCallback(hop),
		Timeout:         c.cfg.DialTimeout,
	}, nil
}
```

`NewConnector`'s signature changed (added `resolver`, `cap` params) — update
`main.go`'s wiring and `sshrelay.NewProvisioner`'s caller accordingly (pass
`postgres.SshTargetStore` as both the existing `SshTargetResolver` and this
new `resolver` param — same concrete type, two narrow interfaces).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/sshconn/... -v
```

Expected: a jump-chain dial test against two local fake SSH servers (bastion
+ target) asserts traffic flows through the bastion; a known-hosts mismatch
rejects the dial; `ErrUnreachableHost.Error()` renders the exact
`"Connection refused: <host>:<port>"` / `"... timed out"` shapes.
