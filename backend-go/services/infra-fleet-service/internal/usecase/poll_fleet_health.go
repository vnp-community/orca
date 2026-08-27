package usecase

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// pollConcurrency bounds fan-out — mirrors BulkProvisionFleet's concurrency
// cap reasoning (many outbound SSH/agent round trips per tick, one poller
// process, don't open them all at once).
const pollConcurrency = 10

// fleetMetricsScript is run via shell.exec — cat /proc/stat first line
// gives cumulative CPU jiffies since boot (single-sample busy-fraction
// approximation, no baseline diff available across ticks in this design),
// free -b gives RAM in bytes, df -P ~ gives disk usage including a
// ready-to-use Capacity percent column. The "---" separators let
// parseFleetMetrics split the combined stdout back into its three sections
// without depending on exact line counts from any one command.
const fleetMetricsScript = "cat /proc/stat; echo ---; free -b; echo ---; df -P ~"

// PollFleetHealth is BUG-FLEET-03's missing writer — a ticking background
// job that polls every dev server (bounded concurrency, per-target
// Postgres advisory lock so exactly one replica polls each target per
// interval), writes infra.fleet_health, and emits a status-change
// event/webhook when a server's health status flips.
//
// Reachability uses the already-real devserveragent.Client.Health
// handshake check; metrics use the already-real shell.exec JSON-RPC
// method — BL-FLEET-03's spec'd "GET .../health via SSH tunnel" endpoint
// does not exist anywhere in agent/ (see SOL-FLEET-02's finding), so this
// substitutes a functionally-equivalent already-real primitive instead of
// blocking on new agent/ work, same substitution TASK-FLEET-02-05 made.
type PollFleetHealth struct {
	devServers DevServerRepository
	health     FleetHealthWriter
	agent      DevServerAgentClient
	lock       PollLockPort
	events     HealthEventPublisher
	webhook    WebhookAlerter
	logger     *slog.Logger
}

func NewPollFleetHealth(
	devServers DevServerRepository,
	health FleetHealthWriter,
	agent DevServerAgentClient,
	lock PollLockPort,
	events HealthEventPublisher,
	webhook WebhookAlerter,
	logger *slog.Logger,
) *PollFleetHealth {
	return &PollFleetHealth{devServers: devServers, health: health, agent: agent, lock: lock, events: events, webhook: webhook, logger: logger}
}

// Run ticks every interval until ctx is cancelled — called once from
// main.go as `go pollFleetHealth.Run(ctx, interval)`.
func (uc *PollFleetHealth) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			uc.pollOnce(ctx)
		}
	}
}

func (uc *PollFleetHealth) pollOnce(ctx context.Context) {
	servers, err := uc.devServers.ListAllForPolling(ctx)
	if err != nil {
		uc.logger.ErrorContext(ctx, "poll_fleet_health: listing dev servers failed", slog.Any("error", err))
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, pollConcurrency)
	for _, ds := range servers {
		wg.Add(1)
		sem <- struct{}{}
		go func(ds domain.DevServer) {
			defer wg.Done()
			defer func() { <-sem }()
			locked, unlock, err := uc.lock.TryLock(ctx, ds.ID)
			if err != nil || !locked {
				return
			}
			defer unlock()
			uc.pollOne(ctx, ds)
		}(ds)
	}
	wg.Wait()
}

func (uc *PollFleetHealth) pollOne(ctx context.Context, ds domain.DevServer) {
	pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	start := time.Now()
	reachable, err := uc.agent.Health(pollCtx, ds)
	latencyMS := time.Since(start).Milliseconds()
	if err != nil {
		reachable = false
	}

	var cpu, ram, disk float64
	relayReachable := reachable
	if reachable {
		result, execErr := uc.agent.Exec(pollCtx, ds, "shell.exec", map[string]any{
			"script": fleetMetricsScript, "timeoutMs": 5000,
		})
		if execErr != nil {
			relayReachable = false
		} else {
			cpu, ram, disk = parseFleetMetrics(result)
		}
	}

	status := domain.ComputeHealthStatus(reachable, relayReachable, cpu, ram)
	sample := domain.DevServerHealth{
		DevServerID: ds.ID, Reachable: reachable, CPUPercent: cpu, RAMPercent: ram, DiskPercent: disk,
		LatencyMS: latencyMS, Status: status,
	}

	previous, hadPrevious, _ := uc.health.GetPrevious(ctx, ds.ID)
	if err := uc.health.UpsertFleetHealth(ctx, sample); err != nil {
		uc.logger.ErrorContext(ctx, "poll_fleet_health: upsert failed", slog.String("devServerId", ds.ID), slog.Any("error", err))
		return
	}

	if hadPrevious && previous.Status != status {
		uc.events.PublishStatusChange(ctx, ds, previous.Status, status)
		uc.webhook.NotifyStatusChange(ctx, ds, previous.Status, status, sample)
	}
}

// parseFleetMetrics is pure parsing of fleetMetricsScript's combined stdout
// (three "---"-separated sections: /proc/stat, free -b, df -P ~) — never
// panics; any section that fails to parse degrades to 0 for that value.
func parseFleetMetrics(result map[string]any) (cpuPercent, ramPercent, diskPercent float64) {
	stdout, _ := result["stdout"].(string)
	sections := strings.Split(stdout, "---")
	var statOut, freeOut, dfOut string
	if len(sections) > 0 {
		statOut = sections[0]
	}
	if len(sections) > 1 {
		freeOut = sections[1]
	}
	if len(sections) > 2 {
		dfOut = sections[2]
	}
	return parseCPUPercent(statOut), parseRAMPercent(freeOut), parseDiskPercent(dfOut)
}

// parseCPUPercent reads /proc/stat's first "cpu " line — cumulative
// jiffies since boot, so this is a single-sample busy-since-boot
// approximation (user+nice+system+irq+softirq+steal vs. total), not an
// instantaneous load — there is no prior sample to diff against in this
// design (see fleetMetricsScript's doc comment). Format:
// "cpu  user nice system idle iowait irq softirq steal guest guest_nice".
func parseCPUPercent(statOut string) float64 {
	for _, line := range strings.Split(statOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}
		var total float64
		values := make([]float64, 0, len(fields)-1)
		for _, f := range fields[1:] {
			v, err := strconv.ParseFloat(f, 64)
			if err != nil {
				return 0
			}
			values = append(values, v)
			total += v
		}
		if total <= 0 || len(values) < 4 {
			return 0
		}
		idle := values[3] // idle
		if len(values) > 4 {
			idle += values[4] // + iowait, also idle time
		}
		busy := total - idle
		if busy < 0 {
			return 0
		}
		return busy / total * 100
	}
	return 0
}

// parseRAMPercent reads free -b's "Mem:" line — columns are
// total/used/free/shared/buff-cache/available (bytes, -b flag).
func parseRAMPercent(freeOut string) float64 {
	for _, line := range strings.Split(freeOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "Mem:" {
			continue
		}
		total, err := strconv.ParseFloat(fields[1], 64)
		if err != nil || total <= 0 {
			return 0
		}
		used, err := strconv.ParseFloat(fields[2], 64)
		if err != nil || used < 0 {
			return 0
		}
		return used / total * 100
	}
	return 0
}

// parseDiskPercent reads df -P ~'s data row — the Capacity column is
// already a ready-to-use percent-used value (e.g. "41%"), so this parses
// that directly rather than recomputing from the Used/Available byte
// columns.
func parseDiskPercent(dfOut string) float64 {
	lines := strings.Split(strings.TrimSpace(dfOut), "\n")
	if len(lines) < 2 {
		return 0
	}
	// df -P's second line is the data row: Filesystem 1024-blocks Used Available Capacity Mounted-on.
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return 0
	}
	capacity := strings.TrimSuffix(fields[4], "%")
	v, err := strconv.ParseFloat(capacity, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}
