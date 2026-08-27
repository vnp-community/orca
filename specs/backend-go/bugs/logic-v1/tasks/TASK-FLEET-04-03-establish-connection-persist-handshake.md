# TASK-FLEET-04-03: `EstablishConnection` persists handshake facts (Step 2)

**From Solution:** SOL-FLEET-04
**Priority:** P1
**Service:** `infra-fleet-service` (usecase)
**File:** `backend-go/services/infra-fleet-service/internal/usecase/establish_connection.go`
**Depends on:** TASK-FLEET-02-01 (domain fields), TASK-FLEET-02-03 (`UpdateProvisionResult`), TASK-FLEET-04-02 (`LastHandshakeInfo`)
**Status:** `[ ]` TODO

---

## Context

Step 1 (Connect) is already a real SSH+handshake round trip, but the
handshake-derived facts it receives were discarded. This is the second
write path into the platform/arch/node-version columns SOL-FLEET-02
introduces (`BulkProvisionFleet` is the first) — both should persist
`HandshakeInfo` since both receive one.

## Changes to make

In `EstablishConnection`'s existing flow, right after the existing
`Health()` check and before constructing `Connection`:

```go
reachable, err := uc.agent.Health(ctx, devServer)
if err != nil || !reachable {
    return domain.Connection{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_SSH_CONNECT_FAILED", "failed to establish SSH connection to target", err)
}
// Persist handshake-derived platform facts — session already holds them
// post-handshake (session.attachTransport receives HandshakeInfo at
// connect time), so this call site can persist without a second round
// trip. Defensive: LastHandshakeInfo's ok=false should not happen here
// (we just confirmed reachable==true), but is handled without erroring
// the whole Execute.
if info, ok := uc.agent.LastHandshakeInfo(devServer.ID); ok {
    _ = uc.devServers.UpdateProvisionResult(ctx, tenantID, devServer.ID, domain.DevServerStatusHealthy, info, time.Now())
}
```

`DevServerAgentClient` interface in `ports.go` gains
`LastHandshakeInfo(devServerID string) (devserveragent.HandshakeInfo, bool)`
alongside its existing `Health`/`Exec` methods.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestEstablishConnection -v
```

Expected: fake `DevServerAgentClient` with `LastHandshakeInfo` returning a
fixture `HandshakeInfo` — `UpdateProvisionResult` is called with the right
platform/arch/version fields after a successful connect; `LastHandshakeInfo`'s
`ok=false` branch skips the persist call without erroring the whole
`Execute`.
