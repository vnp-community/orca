# TASK-SSH-02-05: `diagnosticStderr` capped buffer + `collectDiagnostics` (A3)

**From Solution:** SOL-SSH-02
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/diagnostics.go` (new)
**Depends on:** none
**Status:** `[x] DONE — diagnostics.go's diagnosticStderr (capped, tail-truncating) + collectDiagnostics added; tests pass`

---

## Context

A3 wants crash diagnostics surfaced when the launched relay process fails to
handshake. `launch()` currently discards `session.Stderr` entirely, and no
diagnostic collection exists. This task builds the two standalone pieces;
wiring them into `launch()`/`provisioner.go` is TASK-SSH-02-06/07.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/diagnostics.go`:

```go
package sshrelay

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// diagnosticStderr is a capped, thread-safe io.Writer collecting a launched
// relay process's stderr — bounded so a crash-looping or verbose process
// can't exhaust memory, matching the "no unbounded buffer" discipline
// devserveragent's routeNotification (drop-on-full) already applies
// elsewhere in this service. Truncates from the front, keeping the tail
// (the most recent, most diagnostic-relevant output).
type diagnosticStderr struct {
	mu  sync.Mutex
	buf []byte
	cap int
}

func newDiagnosticStderr(capBytes int) *diagnosticStderr {
	return &diagnosticStderr{cap: capBytes}
}

func (d *diagnosticStderr) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.buf = append(d.buf, p...)
	if len(d.buf) > d.cap {
		d.buf = d.buf[len(d.buf)-d.cap:]
	}
	return len(p), nil
}

func (d *diagnosticStderr) String() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return string(d.buf)
}

// collectDiagnostics runs a handful of cheap remote probes after a launch
// failure — information only, no automatic remediation. Best-effort: a
// probe failure is folded into the diagnostic text ("uname -s: <error>"),
// never returned as a second error that could mask the original
// handshake-timeout cause.
func collectDiagnostics(ctx context.Context, conn *sshconn.Connection, stderrBuf *diagnosticStderr) string {
	osOut, _, osErr := conn.RunCommand(ctx, "uname -s")
	archOut, _, archErr := conn.RunCommand(ctx, "uname -m")
	nodeOut, _, nodeErr := conn.RunCommand(ctx, "node --version")
	whoamiOut, _, whoamiErr := conn.RunCommand(ctx, "whoami")
	return fmt.Sprintf("diagnostics: os=%s arch=%s node=%s user=%s stderr_tail=%q",
		probeResult(osOut, osErr), probeResult(archOut, archErr),
		probeResult(nodeOut, nodeErr), probeResult(whoamiOut, whoamiErr),
		stderrBuf.String())
}

func probeResult(out string, err error) string {
	if err != nil {
		return fmt.Sprintf("<error: %v>", err)
	}
	return strings.TrimSpace(out)
}
```

Add the `"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"`
import.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/sshrelay/... -run TestDiagnosticStderr -v
```

Expected new test (`diagnostics_test.go`): writing more than `cap` bytes
keeps only the tail; `collectDiagnostics` against a fake `Connection` embeds
each probe's output, and a probe failure degrades to `<error: ...>` text
rather than aborting.
