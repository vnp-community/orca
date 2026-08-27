# TASK-FLEET-02-04: Remote prerequisite checks (`sshrelay/prereq.go`) + `Provisioner.Provision` integration

**From Solution:** SOL-FLEET-02
**Priority:** P1
**Service:** `infra-fleet-service` (sshrelay adapter)
**File:** `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/prereq.go` (new), `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

Prerequisite checks (Node/Git/disk) must run over the raw SSH connection
before the agent is deployed — there is nothing to relay a JSON-RPC call to
yet, so this cannot go through `devserveragent.Client.Exec`. Matches
BL-FLEET-02's step ordering: SSH connect → check prerequisites → deploy.

## Changes to make

`internal/adapter/sshrelay/prereq.go`:

```go
package sshrelay

type PrereqResult struct {
    NodeVersion, GitVersion string
    NodeOK, GitOK, DiskOK   bool
    FreeDiskGB              float64
}

var ErrPrerequisitesNotMet = errors.New("sshrelay: remote host does not meet minimum prerequisites")

// checkPrerequisites runs node --version / git --version / df -P against
// conn and parses the results — same conn.RunCommand primitive deploy.go's
// checksum step already uses (sshconn.Connection.RunCommand), no new
// transport needed. Node >= 22, Git >= 2.25 (matches
// docs/reference/git-compatibility.md's core-workflow baseline — a
// different concern: this checks the TARGET host's git, not
// git-gateway-service's executor), disk >= 5GB free.
func checkPrerequisites(ctx context.Context, conn *sshconn.Connection, minNode, minGit semver.Version) (PrereqResult, error) {
    var result PrereqResult
    nodeOut, _, err := conn.RunCommand(ctx, "node --version")
    if err == nil {
        result.NodeVersion, result.NodeOK = parseAndCompareVersion(nodeOut, minNode)
    }
    gitOut, _, err := conn.RunCommand(ctx, "git --version")
    if err == nil {
        result.GitVersion, result.GitOK = parseAndCompareVersion(gitOut, minGit)
    }
    diskOut, _, err := conn.RunCommand(ctx, "df -P ~ | tail -1 | awk '{print $4}'")
    if err == nil {
        result.FreeDiskGB, result.DiskOK = parseDiskKB(diskOut) // >= 5GB
    }
    return result, nil // never returns an error itself — unparseable output means NodeOK/GitOK/DiskOK stay false, not a crash
}
```

In `provisioner.go`, add a `checkPrerequisites` call inside
`Provisioner.Provision` right after `p.connector.Connect` and before
`deploy`. A failed prerequisite does not abort the pipeline — record the
`PrereqResult` and continue to attempt deploy, but return a distinguishable
`ErrPrerequisitesNotMet`-wrapped result (or side-channel) the usecase layer
(TASK-FLEET-02-05) maps to `degraded` rather than `unhealthy`, and which
does not consume a retry attempt.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/sshrelay/... -run TestCheckPrerequisites -v
```

Expected: fake `conn.RunCommand` outputs `v22.3.0`/`git version 2.39.2`/
sufficient disk → all-OK; `v18.0.0` → `NodeOK=false`; unparseable output →
treated as not-OK, not a crash.
