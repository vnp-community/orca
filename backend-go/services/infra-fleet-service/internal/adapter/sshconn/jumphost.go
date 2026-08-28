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
	if err == nil {
		return false
	}
	if err == context.DeadlineExceeded { //nolint:errorlint // exact sentinel match is intended here
		return true
	}
	var netErr net.Error
	if ok := asNetError(err, &netErr); ok {
		return netErr.Timeout()
	}
	return false
}

func asNetError(err error, target *net.Error) bool {
	ne, ok := err.(net.Error) //nolint:errorlint // net.Error is checked directly, matching Go's net package convention
	if !ok {
		return false
	}
	*target = ne
	return true
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
		if resolver == nil {
			return nil, fmt.Errorf("sshconn: target has jump_host_target_id %q but no resolver was configured", current.JumpHostTargetID)
		}
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
