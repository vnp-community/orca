package sshrelay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
)

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
		return "", fmt.Errorf("sshrelay: relay bundle checksum mismatch after upload (local=%s remote=%s) — aborting deploy", localHex, remoteHex)
	}

	return remoteDir, nil
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
