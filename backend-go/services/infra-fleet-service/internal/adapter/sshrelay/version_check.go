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
