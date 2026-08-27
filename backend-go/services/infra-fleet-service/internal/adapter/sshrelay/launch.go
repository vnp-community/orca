package sshrelay

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
)

const diagnosticStderrCapBytes = 64 * 1024 // a crash-looping process must never grow this unbounded

// relaySockPath is fixed per remoteDir (one dedicated SSH connection/relay
// process per relay-ssh session — see deploy.go's doc comment on
// remoteDir/remoteAgentFile's non-collision rationale, which applies
// identically here).
func relaySockPath(remoteDir string) string {
	return remoteDir + "/relay.sock"
}

// launch starts the relay process in DETACHED mode on first provision
// (agent-connection-stdio.ts's runDetachedStdioMode — TASK-SSH-03-01) and
// immediately bridges to it via reattach(). Every subsequent call for the
// lifetime of that detached process should go through reattach() directly,
// not launch() again — see provisioner.go's Provision/Reattach callers.
// Returns the transport, the socket path (cached by the caller for a later
// background reattach), a capped buffer of the bridge session's stderr (read
// by provisioner.go's collectDiagnostics on a handshake failure — A3), and
// an error.
func launch(ctx context.Context, conn *sshconn.Connection, remoteDir, devServerID string) (transport *sshExecTransport, sockPath string, stderrBuf *diagnosticStderr, err error) {
	sockPath = relaySockPath(remoteDir)
	cmd := fmt.Sprintf(
		"cd %s && DEV_SERVER_ID=%s node %s --detach --sock-path %s",
		shellQuote(remoteDir), shellQuote(devServerID), shellQuote(remoteAgentFile), shellQuote(sockPath),
	)
	// Blocks only until the detached child reports "listening" and the
	// parent (still attached to THIS exec channel) exits 0 — see
	// runDetachedStdioMode's doc comment. This session then closes cleanly;
	// it is not reused as the bridge.
	if _, _, err := conn.RunCommand(ctx, cmd); err != nil {
		return nil, "", nil, fmt.Errorf("sshrelay: starting detached relay process: %w", err)
	}
	transport, sockPath, stderrBuf, err = reattach(ctx, conn, remoteDir, sockPath)
	return transport, sockPath, stderrBuf, err
}

// ErrDetachedProcessGone signals the detached process itself is no longer
// alive (crashed, host rebooted, socket file stale) — the caller's cue to
// fall back to a full Provision (redeploy+relaunch) rather than retrying
// reattach() against a dead socket.
var ErrDetachedProcessGone = errors.New("sshrelay: detached relay process is no longer running")

// reattach opens a FRESH SSH exec session running
// `node agent.js --connect --sock-path <path>` — the cheap path every
// reconnect after the first takes: no SFTP, no checksum, no new node
// process, just a new bridge over the SSH exec channel onto the SAME
// already-running detached agent. This is what makes SOL-SSH-02's
// version-check matter for more than just first-connect: a reconnect that
// hits this path never redeploys at all. session.Stderr is wired to a capped
// diagnosticStderr the same way launch()'s original single-process mode did,
// in case the bridge process itself (not the detached agent) fails.
func reattach(ctx context.Context, conn *sshconn.Connection, remoteDir, sockPath string) (*sshExecTransport, string, *diagnosticStderr, error) {
	alive, _, _ := conn.RunCommand(ctx, fmt.Sprintf("test -S %s && echo alive", shellQuote(sockPath)))
	if strings.TrimSpace(alive) != "alive" {
		return nil, "", nil, ErrDetachedProcessGone
	}

	session, err := conn.NewSession()
	if err != nil {
		return nil, "", nil, fmt.Errorf("sshrelay: opening reattach session: %w", err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, "", nil, fmt.Errorf("sshrelay: opening stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		return nil, "", nil, fmt.Errorf("sshrelay: opening stdout pipe: %w", err)
	}

	stderrBuf := newDiagnosticStderr(diagnosticStderrCapBytes)
	session.Stderr = stderrBuf

	cmd := fmt.Sprintf("cd %s && node %s --connect --sock-path %s",
		shellQuote(remoteDir), shellQuote(remoteAgentFile), shellQuote(sockPath))
	if err := session.Start(cmd); err != nil {
		_ = session.Close()
		return nil, "", nil, fmt.Errorf("sshrelay: starting connect bridge: %w", err)
	}

	return newSSHExecTransport(conn, session, stdin, stdout), sockPath, stderrBuf, nil
}
