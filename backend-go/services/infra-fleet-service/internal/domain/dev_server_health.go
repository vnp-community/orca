package domain

import "errors"

var (
	// ErrEmptyHealthDevServerID guards against an orphaned health sample.
	ErrEmptyHealthDevServerID = errors.New("domain: dev_server_id is required")
	// ErrPercentOutOfRange guards CPU/RAM/disk readings against corrupt
	// samples silently skewing fleet dashboards.
	ErrPercentOutOfRange = errors.New("domain: percent must be within [0, 100]")
	// ErrNegativeLatency guards against a nonsensical negative round-trip time.
	ErrNegativeLatency = errors.New("domain: latency_ms must be non-negative")
)

// DevServerHealth is a fleet-health snapshot for one DevServer — CPU/RAM/
// disk/latency, collected by the health poller described in
// specs/backend-go/services/infra-fleet-service.md §8 (30s cadence; the
// poller itself is not implemented in this scaffold — see this service's
// README "Known gaps").
type DevServerHealth struct {
	DevServerID string
	Reachable   bool
	CPUPercent  float64
	RAMPercent  float64
	DiskPercent float64
	LatencyMS   int64
	Status      HealthStatus
}

// HealthStatus is BL-FLEET-03's per-server health classification — see
// ComputeHealthStatus's doc comment for the threshold table it implements.
type HealthStatus string

const (
	HealthStatusHealthy     HealthStatus = "healthy"
	HealthStatusDegraded    HealthStatus = "degraded"
	HealthStatusUnhealthy   HealthStatus = "unhealthy"
	HealthStatusUnreachable HealthStatus = "unreachable"
)

// ComputeHealthStatus applies BL-FLEET-03's threshold table verbatim
// (docs/logic/fleet/BL-FLEET-03-health-monitoring.md's Status Model table)
// — a pure function so its thresholds are unit-testable without any I/O.
//
//	healthy     — SSH + relay reachable, CPU < 80%, RAM < 85%
//	degraded    — relay reachable but CPU > 80% or RAM > 85%
//	unhealthy   — relay not reachable but SSH still is
//	unreachable — SSH connect timeout/fail
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

// NewDevServerHealth constructs a DevServerHealth, enforcing the invariants
// above. An unreachable sample (Reachable=false) still validates its
// percent/latency fields — callers pass zeros for an unreachable host,
// which are valid inputs, not a special-cased skip.
func NewDevServerHealth(devServerID string, reachable bool, cpuPercent, ramPercent, diskPercent float64, latencyMS int64) (DevServerHealth, error) {
	if devServerID == "" {
		return DevServerHealth{}, ErrEmptyHealthDevServerID
	}
	for _, p := range []float64{cpuPercent, ramPercent, diskPercent} {
		if p < 0 || p > 100 {
			return DevServerHealth{}, ErrPercentOutOfRange
		}
	}
	if latencyMS < 0 {
		return DevServerHealth{}, ErrNegativeLatency
	}
	return DevServerHealth{
		DevServerID: devServerID,
		Reachable:   reachable,
		CPUPercent:  cpuPercent,
		RAMPercent:  ramPercent,
		DiskPercent: diskPercent,
		LatencyMS:   latencyMS,
	}, nil
}
