package sshrelay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
)

// ErrChecksumMismatch is deploy()'s checksum-mismatch sentinel — lets
// deployWithRetry distinguish it from a network/SFTP error via errors.Is
// rather than string-matching, so dispatch survives message wording changes.
var ErrChecksumMismatch = errors.New("sshrelay: relay bundle checksum mismatch")

// remoteDir/remoteAgentFile are fixed — one dedicated SSH connection is
// opened per relay-ssh session (see transport.go's Connection field), so
// there's no cross-session collision to guard against the way the TS
// reference's shared-daemon model needs a target-ID-hashed socket name for
// (see agent/api/connection-modes.md §4's launchRelay notes — not
// applicable here, this pass's launch model is one-shot, not a
// reattach-to-a-running-daemon design, see package doc comment).
const (
	remoteDir       = ".orca-relay"
	remoteAgentFile = "agent.js"
)

// deploy SFTP-uploads cfg.BundlePath to remoteDir/remoteAgentFile on conn's
// target and SHA-256-checksum-verifies it, mirroring
// ssh-relay-deploy.ts's verifyRelayChecksum — the trust boundary for the
// one file this pass actually executes remotely. The remote checksum is
// computed via a portable `node -e` one-liner (same trick the TS reference
// uses) rather than assuming `sha256sum` exists on the target.
func deploy(ctx context.Context, conn *sshconn.Connection, cfg Config) (string, error) {
	if cfg.BundlePath == "" {
		return "", fmt.Errorf("sshrelay: ORCA_RELAY_BUNDLE_PATH is not configured — nothing to deploy")
	}

	localBytes, err := os.ReadFile(cfg.BundlePath)
	if err != nil {
		return "", fmt.Errorf("sshrelay: reading local relay bundle %q: %w", cfg.BundlePath, err)
	}

	if _, _, err := conn.RunCommand(ctx, fmt.Sprintf("mkdir -p %s", shellQuote(remoteDir))); err != nil {
		return "", fmt.Errorf("sshrelay: creating remote deploy dir: %w", err)
	}

	sftpClient, err := conn.SFTPClient()
	if err != nil {
		return "", fmt.Errorf("sshrelay: opening sftp client: %w", err)
	}
	defer func() { _ = sftpClient.Close() }()

	remotePath := remoteDir + "/" + remoteAgentFile
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return "", fmt.Errorf("sshrelay: creating remote file %q: %w", remotePath, err)
	}
	if _, err := remoteFile.Write(localBytes); err != nil {
		_ = remoteFile.Close()
		return "", fmt.Errorf("sshrelay: uploading relay bundle: %w", err)
	}
	if err := remoteFile.Close(); err != nil {
		return "", fmt.Errorf("sshrelay: closing remote file after upload: %w", err)
	}

	localSum := sha256.Sum256(localBytes)
	localHex := hex.EncodeToString(localSum[:])
	checksumCmd := fmt.Sprintf(
		`node -e "const c=require('crypto'),fs=require('fs');process.stdout.write(c.createHash('sha256').update(fs.readFileSync(%s)).digest('hex'))"`,
		jsStringLiteral(remotePath),
	)
	remoteHexOut, _, err := conn.RunCommand(ctx, checksumCmd)
	if err != nil {
		return "", fmt.Errorf("sshrelay: computing remote checksum: %w", err)
	}
	remoteHex := strings.TrimSpace(remoteHexOut)
	if remoteHex != localHex {
		return "", fmt.Errorf("%w after upload (local=%s remote=%s)", ErrChecksumMismatch, localHex, remoteHex)
	}

	return remoteDir, nil
}

const maxDeployNetworkRetries = 3

// deployWithRetry wraps deploy() with up to maxDeployNetworkRetries attempts
// (A1). A checksum mismatch (A2) triggers exactly one immediate
// re-upload-and-recheck, not folded into the network-retry budget — a
// persistent mismatch after that one retry fails outright rather than
// retrying identically 3x (which would just repeat the same corrupted
// transfer and mask a real corruption/tampering signal).
func deployWithRetry(ctx context.Context, conn *sshconn.Connection, cfg Config) (string, error) {
	var lastErr error
	for attempt := 0; attempt < maxDeployNetworkRetries; attempt++ {
		dir, err := deploy(ctx, conn, cfg)
		if err == nil {
			return dir, nil
		}
		lastErr = err
		if errors.Is(err, ErrChecksumMismatch) {
			if dir, rerr := deploy(ctx, conn, cfg); rerr == nil {
				return dir, nil
			} else if errors.Is(rerr, ErrChecksumMismatch) {
				return "", fmt.Errorf("sshrelay: relay bundle checksum mismatch persisted after re-upload — refusing to launch a possibly-corrupted/tampered bundle: %w", rerr)
			} else {
				lastErr = rerr
			}
			break // don't network-retry after a checksum-specific failure path
		}
		if attempt < maxDeployNetworkRetries-1 {
			time.Sleep(deployBackoffDelay(attempt))
		}
	}
	return "", fmt.Errorf("sshrelay: deploy failed after %d attempts: %w", maxDeployNetworkRetries, lastErr)
}

// deployBackoffDelay: 500ms, 1s, 2s — small on purpose, deploy sits on the
// connect-latency-sensitive path (a caller is waiting for EstablishConnection
// to return).
func deployBackoffDelay(attempt int) time.Duration {
	return 500 * time.Millisecond * time.Duration(1<<uint(attempt))
}

// shellQuote wraps s in single quotes for POSIX shell, escaping any
// embedded single quote by closing the quote, emitting an escaped literal
// quote, then reopening the quote — same convention used elsewhere in this
// service (see devserveragent's now-removed relay-ssh shell.exec path,
// superseded by this package; the escaping logic is small enough not to
// share as an exported helper across packages for it).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// jsStringLiteral double-quote-escapes s for embedding inside the
// single-quoted `node -e "..."` command above — the remote path never
// contains untrusted input (it's built entirely from this package's own
// constants), but this is cheap correctness insurance rather than assuming
// a fixed path never needs escaping.
func jsStringLiteral(s string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s)
	return "'" + escaped + "'"
}
