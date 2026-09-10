# TASK-SSH-02-07: `Provision` collects diagnostics on handshake failure (A3)

**From Solution:** SOL-SSH-02
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go`
**Depends on:** TASK-SSH-02-04, TASK-SSH-02-06
**Status:** `[x] DONE — Provision's handshake-failure path appends collectDiagnostics' output; TestProvision_HandshakeTimeout_IncludesDiagnostics passes`

---

## Context

Completes A3: when `receiveHandshake` times out, `Provision` today returns
a bare timeout error with no information about *why* the remote process
never sent `agent.handshake`. This updates `launch`'s call site to the new
`(*sshExecTransport, *diagnosticStderr, error)` signature (TASK-SSH-02-06)
and appends `collectDiagnostics`'s output on failure.

## Changes to make

In `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go`,
update `Provision`'s `launch(...)` call and handshake-failure branch (the
rest of `Provision`, including TASK-SSH-02-04's version-gate, is unchanged):

```go
	transport, stderrBuf, err := launch(conn, remoteDir, devServer.ID)
	if err != nil {
		_ = conn.Close()
		return nil, devserveragent.HandshakeInfo{}, err
	}

	info, err := p.receiveHandshake(ctx, transport)
	if err != nil {
		_ = transport.Close("handshake failed")
		diag := collectDiagnostics(ctx, conn, stderrBuf)
		return nil, devserveragent.HandshakeInfo{}, fmt.Errorf("%w\n%s", err, diag)
	}

	return transport, info, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/sshrelay/... -v
```

Expected: `TestProvision_*` suite passes; a new
`TestProvision_HandshakeTimeout_IncludesDiagnostics` asserts the returned
error's message contains the diagnostic probes' output (`os=`, `arch=`,
`node=`, `user=`, `stderr_tail=`); a probe failure (e.g. `whoami`
unsupported on a minimal fake server) degrades gracefully into the
diagnostic text rather than swallowing the original timeout error.
