# TASK-FLEET-03-01: `HealthStatus` + `ComputeHealthStatus` + `DevServerHealth.Status`

**From Solution:** SOL-FLEET-03
**Priority:** P0
**Service:** `infra-fleet-service` (domain)
**File:** `backend-go/services/infra-fleet-service/internal/domain/dev_server_health.go`
**Depends on:** none
**Status:** [x] DONE — added HealthStatus type + ComputeHealthStatus + DevServerHealth.Status; table test covers the full threshold matrix (79/81, 84/86, exactly-80/85 boundaries, unreachable/unhealthy priority). Passes.

---

## Context

`PollFleetHealth` (TASK-FLEET-03-05) needs a pure, unit-testable function
applying BL-FLEET-03's threshold table, and a `Status` field on
`DevServerHealth` to carry the result.

## Changes to make

```go
// internal/domain/dev_server_health.go (extended)
type HealthStatus string

const (
    HealthStatusHealthy     HealthStatus = "healthy"
    HealthStatusDegraded    HealthStatus = "degraded"
    HealthStatusUnhealthy   HealthStatus = "unhealthy"
    HealthStatusUnreachable HealthStatus = "unreachable"
)

// ComputeHealthStatus applies BL-FLEET-03's threshold table verbatim
// (BL-FLEET-03-health-monitoring.md:16-21) — a pure function so its
// thresholds are unit-testable without any I/O.
func ComputeHealthStatus(reachable, relayReachable bool, cpuPercent, ramPercent float64) HealthStatus {
    if !reachable {
        return HealthStatusUnreachable // SSH/agent connect itself failed
    }
    if !relayReachable {
        return HealthStatusUnhealthy // SSH/session alive, agent handshake not
    }
    if cpuPercent > 80 || ramPercent > 85 {
        return HealthStatusDegraded
    }
    return HealthStatusHealthy
}

type DevServerHealth struct {
    DevServerID string
    Reachable   bool
    CPUPercent, RAMPercent, DiskPercent float64
    LatencyMS   int64
    Status      HealthStatus // new
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/domain/... -run TestComputeHealthStatus -v
```

Expected: table test over BL-FLEET-03's exact threshold matrix (CPU 79/81,
RAM 84/86, unreachable, handshake-alive-but-relay-down).
