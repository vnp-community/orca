# TASK-SSH-02-02: `remoteVersionAndPresence` — probe a deployed relay's running version (BR-SSH-07)

**From Solution:** SOL-SSH-02
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/version_check.go` (new)
**Depends on:** TASK-SSH-02-01
**Status:** `[ ]` TODO

---

## Context

`Provisioner.Provision` always redeploys via `deploy()`'s SFTP
upload+checksum round trip on every call, even when the already-deployed
bundle is the exact version this backend-go instance would deploy. BR-SSH-07
wants a version check before that upload. This task adds the cheap probe;
wiring it into `Provision` is TASK-SSH-02-04.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/version_check.go`:

```go
package sshrelay

import (
	"context"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
)

// remoteVersionAndPresence probes an already-deployed relay for its
// AGENT_VERSION export (see agent/build.mjs's esbuild `define` and
// agent-entry.ts's export — TASK-SSH-02-01) by attempting a lightweight
// `node -e` handshake-only read of remoteDir/remoteAgentFile if it exists —
// cheaper than deploy()'s SFTP upload+checksum round trip. Returns
// ("", false, nil) when no prior deployment exists (first-time provision),
// which Provisioner treats as "must deploy," not an error. A probe error
// (e.g. `node` missing on the target) is also treated as "must redeploy" —
// deploy() itself will surface a clearer failure if something is
// structurally wrong with the target, so this probe never blocks a first
// attempt on its own uncertainty.
func remoteVersionAndPresence(ctx context.Context, conn *sshconn.Connection) (version string, present bool, err error) {
	remotePath := remoteDir + "/" + remoteAgentFile
	cmd := fmt.Sprintf(
		`test -f %s && node -e "console.log(require(%s).AGENT_VERSION||'unknown')" 2>/dev/null || true`,
		shellQuote(remotePath), jsStringLiteral("./"+remotePath),
	)
	out, _, runErr := conn.RunCommand(ctx, cmd)
	if runErr != nil {
		return "", false, runErr
	}
	v := strings.TrimSpace(out)
	if v == "" || v == "unknown" {
		return "", false, nil
	}
	return v, true, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/sshrelay/... -run TestRemoteVersionAndPresence -v
```

Expected new test (`version_check_test.go`, fake exec transport à la
`connector_test.go`'s fake SSH server): no prior file returns
`("", false, nil)`; a present file with a version prints returns
`(version, true, nil)`; a probe command failure returns `("", false, err)`.
