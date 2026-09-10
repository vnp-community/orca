# TASK-SSH-02-06: `launch()` wires `session.Stderr` to the capped buffer

**From Solution:** SOL-SSH-02
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go`
**Depends on:** TASK-SSH-02-05
**Status:** `[x] DONE — launch() now returns (*sshExecTransport, *diagnosticStderr, error) with session.Stderr wired to the capped buffer; TestLaunch_ReturnsACappedStderrBuffer passes`

---

## Context

`launch()` (`launch.go:18-45`) never sets `session.Stderr` — a crashing
relay process's stderr is lost entirely today. This wires it to
TASK-SSH-02-05's `diagnosticStderr`, and returns it alongside the transport
so `provisioner.go` can read it on handshake failure (TASK-SSH-02-07).

## Changes to make

Replace `launch()`'s signature and body in
`backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go`:

```go
const diagnosticStderrCapBytes = 64 * 1024 // a crash-looping process must never grow this unbounded

// launch opens a fresh SSH exec session over conn and starts
// `node agent.js --stdio` in remoteDir. Returns the transport and a capped
// buffer of the process's stderr — read by provisioner.go's
// collectDiagnostics on a handshake failure (A3), discarded otherwise.
func launch(conn *sshconn.Connection, remoteDir, devServerID string) (*sshExecTransport, *diagnosticStderr, error) {
	session, err := conn.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("sshrelay: opening launch session: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, nil, fmt.Errorf("sshrelay: opening stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		return nil, nil, fmt.Errorf("sshrelay: opening stdout pipe: %w", err)
	}

	stderrBuf := newDiagnosticStderr(diagnosticStderrCapBytes)
	session.Stderr = stderrBuf

	cmd := fmt.Sprintf(
		"cd %s && DEV_SERVER_ID=%s node %s --stdio",
		shellQuote(remoteDir), shellQuote(devServerID), shellQuote(remoteAgentFile),
	)
	if err := session.Start(cmd); err != nil {
		_ = session.Close()
		return nil, nil, fmt.Errorf("sshrelay: starting relay process: %w", err)
	}

	return newSSHExecTransport(conn, session, stdin, stdout), stderrBuf, nil
}
```

`launch`'s only caller, `Provisioner.Provision`, needs its call site updated
to the 3-return-value shape — see TASK-SSH-02-07, which does that update
alongside wiring `collectDiagnostics` into the handshake-failure path (both
land in `provisioner.go` together to avoid an intermediate broken-build
state).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/... 2>&1 | head -30
```

Expected: `launch.go` itself compiles; `provisioner.go` fails to compile
until TASK-SSH-02-07 updates its call site — expected at this point in the
sequence. A standalone `launch_test.go` test (new) can verify
`session.Stderr` is set and capped by driving `launch` against a fake SSH
server directly, independent of `provisioner.go`.
