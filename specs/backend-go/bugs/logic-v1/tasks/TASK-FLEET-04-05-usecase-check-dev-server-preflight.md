# TASK-FLEET-04-05: `usecase.CheckDevServerPreflight` (Step 4)

**From Solution:** SOL-FLEET-04
**Priority:** P1
**Service:** `infra-fleet-service` (usecase)
**File:** `backend-go/services/infra-fleet-service/internal/usecase/check_dev_server_preflight.go` (new)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BL-FLEET-04 Step 4's checks (`git --version`, `node --version`,
`df -P ~/.orca`, a port probe, `gh --version`) are composable into one
`shell.exec` round trip — the same mechanism SOL-FLEET-03 uses for
fleet-health metrics collection, reused here for a different purpose. Per
BL-FLEET-04's "proceed with warnings" policy, this usecase never itself
hard-fails on a check result — it returns the full result and lets the
caller decide.

## Changes to make

```go
// internal/usecase/check_dev_server_preflight.go
package usecase

const preflightScript = `
echo "GIT:$(git --version 2>/dev/null)"
echo "NODE:$(node --version 2>/dev/null)"
echo "DISK:$(df -P ~/.orca 2>/dev/null | tail -1 | awk '{print $4}')"
echo "GH:$(gh --version 2>/dev/null | head -1)"
node -e "require('net').createServer().listen(%d,'127.0.0.1',()=>{console.log('PORT:FREE');process.exit(0)}).on('error',()=>{console.log('PORT:BUSY');process.exit(0)})"
`

type CheckResult struct {
    Installed bool
    Version   string
    MeetsMin  bool
}
type DiskCheckResult struct {
    FreeGB   float64
    MeetsMin bool
}
type PortCheckResult struct {
    Port      int32
    Available bool
}
type PreflightCheckResult struct {
    Git, Node CheckResult
    Disk      DiskCheckResult
    Port      PortCheckResult
    GH        CheckResult // installed-only, no version-min
}

type CheckDevServerPreflight struct {
    devServers DevServerRepository
    agent      DevServerAgentClient
}

func NewCheckDevServerPreflight(devServers DevServerRepository, agent DevServerAgentClient) *CheckDevServerPreflight {
    return &CheckDevServerPreflight{devServers: devServers, agent: agent}
}

func (uc *CheckDevServerPreflight) Execute(ctx context.Context, tenantID, devServerID string, probePort int32) (PreflightCheckResult, error) {
    ds, err := uc.devServers.Get(ctx, tenantID, devServerID)
    if err != nil {
        return PreflightCheckResult{}, err
    }
    result, err := uc.agent.Exec(ctx, ds, "shell.exec", map[string]any{
        "script": fmt.Sprintf(preflightScript, probePort), "timeoutMs": 8000,
    })
    if err != nil {
        // A shell.exec failure is itself informative (agent unreachable) —
        // surfaced as a typed error, not a synthesized all-false result.
        return PreflightCheckResult{}, apperrors.New(apperrors.KindUnavailable, "INFRA_PREFLIGHT_FAILED", "failed to run remote preflight check", err)
    }
    stdout, _ := result["stdout"].(string)
    return parsePreflightOutput(stdout), nil
}

// parsePreflightOutput is pure parsing — Git>=2.25, Node>=22 thresholds
// (shared test vectors with SOL-FLEET-02's prereq_test.go). Malformed
// output degrades every field to Installed=false/MeetsMin=false, never
// panics.
func parsePreflightOutput(stdout string) PreflightCheckResult {
    // ... line-by-line parse of GIT:/NODE:/DISK:/GH:/PORT: prefixes ...
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestCheckDevServerPreflight -v
```

Expected: fake `Exec` returns a fixture `stdout` block; asserts
`Git.MeetsMin=true` for `2.39.2` and `false` for `2.20.0`; `Port.Available`
parses both `PORT:FREE`/`PORT:BUSY`; malformed `stdout` degrades every
field, never panics.
