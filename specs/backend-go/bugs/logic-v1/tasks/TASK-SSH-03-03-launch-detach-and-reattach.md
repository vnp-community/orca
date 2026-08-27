# TASK-SSH-03-03: `sshrelay` — `launch()` uses `--detach`; new `reattach()`

**From Solution:** SOL-SSH-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go`
**Depends on:** TASK-SSH-03-01, TASK-SSH-03-02
**Status:** `[ ]` TODO

---

## Context

`launch()` (`launch.go:18-45`) starts `node agent.js --stdio` directly on
the exec channel — foreground, tied 1:1 to that SSH session, no detach.
With the agent-side detach mode built (TASK-SSH-03-01/02), the FIRST launch
should start the detached process and immediately bridge to it via
`--connect`; every SUBSEQUENT (re)attach for the lifetime of that detached
process should skip straight to `reattach()` — no new SFTP, no new node
process, just a fresh bridge session.

**Note:** SOL-SSH-02's `TASK-SSH-02-06` also changes `launch()`'s signature
(adds a `*diagnosticStderr` return for crash diagnostics). If that task has
already landed when this one is implemented, carry its `stderrBuf` return
through unchanged alongside the `sockPath` return this task adds — the two
are independent additions to the same function.

## Changes to make

Replace `launch()` in
`backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go`:

```go
package sshrelay

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
)

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
// not launch() again — see provisioner.go's Provision/relaySSHReconnect
// callers (TASK-SSH-03-04).
func launch(ctx context.Context, conn *sshconn.Connection, remoteDir, devServerID string) (transport *sshExecTransport, sockPath string, err error) {
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
		return nil, "", fmt.Errorf("sshrelay: starting detached relay process: %w", err)
	}
	return reattach(ctx, conn, remoteDir, sockPath)
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
// hits this path never redeploys at all.
func reattach(ctx context.Context, conn *sshconn.Connection, remoteDir, sockPath string) (*sshExecTransport, string, error) {
	alive, _, _ := conn.RunCommand(ctx, fmt.Sprintf("test -S %s && echo alive", shellQuote(sockPath)))
	if strings.TrimSpace(alive) != "alive" {
		return nil, "", ErrDetachedProcessGone
	}

	session, err := conn.NewSession()
	if err != nil {
		return nil, "", fmt.Errorf("sshrelay: opening reattach session: %w", err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, "", fmt.Errorf("sshrelay: opening stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		return nil, "", fmt.Errorf("sshrelay: opening stdout pipe: %w", err)
	}

	cmd := fmt.Sprintf("cd %s && node %s --connect --sock-path %s",
		shellQuote(remoteDir), shellQuote(remoteAgentFile), shellQuote(sockPath))
	if err := session.Start(cmd); err != nil {
		_ = session.Close()
		return nil, "", fmt.Errorf("sshrelay: starting connect bridge: %w", err)
	}

	return newSSHExecTransport(conn, session, stdin, stdout), sockPath, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/sshrelay/... -run 'TestLaunch|TestReattach' -v
```

Expected new tests: `launch_test.go` — first `launch()` sends the `--detach`
command; `reattach_test.go` — `test -S` reporting the socket present opens a
plain `--connect` session with no SFTP/checksum calls (assert a fake
`Connection` records zero `SFTPClient()` calls); `test -S` reporting the
socket gone returns `ErrDetachedProcessGone` without attempting a session.
