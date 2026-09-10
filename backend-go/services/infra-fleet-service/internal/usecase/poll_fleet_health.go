package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fleetMetricsScript is run via shell.exec — cat /proc/stat first line
// gives cumulative CPU jiffies since boot (single-sample busy-fraction
// approximation, no baseline diff available across ticks in this design),
// free -b gives RAM in bytes, df -P ~ gives disk usage including a
// ready-to-use Capacity percent column. The "---" separators let
// parseFleetMetrics split the combined stdout back into its three sections
// without depending on exact line counts from any one command.
const fleetMetricsScript = "cat /proc/stat; echo ---; free -b; echo ---; df -P ~"

// PollFleetHealth is the fleet health poller both BUG-FLEET-03 and the "30s-
// cadence poller" specs/backend-go/services/infra-fleet-service.md §8 calls
// for — a per-tick job that polls every registered dev server (sequentially,
// each behind a per-target Postgres advisory lock so exactly one replica
// polls each target per interval when uc.lock is wired), writes
// infra.fleet_health, and reacts to a health-status change three ways:
//
//   - Emits a HealthEventPublisher/WebhookAlerter status-change notification
//     when BL-FLEET-03's coarse healthy/degraded/unhealthy/unreachable
//     classification flips (SOL-FLEET-03).
//   - Drives the Connection state machine (BE-SOL-STORAGE-003 §2) on a plain
//     reachable transition: true->false marks the dev server's active
//     Connection degraded (and alerts via outbox/log), false->true
//     reestablishes it (or closes it if the grace period already expired,
//     closing its terminal_sessions too).
//   - Updates the in-process MetricsCollector cache every tick, reachable or
//     not, so Prometheus scrapes read live data without re-querying Postgres.
//
// Reachability uses the already-real devserveragent.Client.Health
// handshake check; CPU/RAM/disk metrics use the already-real shell.exec
// JSON-RPC method — BL-FLEET-03's spec'd "GET .../health via SSH tunnel"
// endpoint does not exist anywhere in agent/ (see SOL-FLEET-02's finding),
// so this substitutes a functionally-equivalent already-real primitive
// instead of blocking on new agent/ work, same substitution
// TASK-FLEET-02-05 made.
type PollFleetHealth struct {
	devServers FleetHealthPollerRepository
	writer     FleetHealthWriter
	outbox     OutboxWriter
	agent      DevServerAgentClient
	lock       PollLockPort
	conns      ConnectionRepository
	sessions   TerminalSessionRepository
	events     HealthEventPublisher
	webhook    WebhookAlerter
	collector  MetricsCollector
	logger     *slog.Logger
}

// lock may be nil — a single-replica deployment (or a test) can skip
// advisory locking entirely; every server is then polled unconditionally
// every tick. outbox may also be nil (a nil outbox just means a disconnect
// transition still gets logged, never enqueued). conns may also be nil — a
// poll with no ConnectionRepository configured simply skips the
// degraded/reestablish state machine below (BE-SOL-STORAGE-003 §2) and
// behaves exactly as it did before that state machine existed. sessions may
// also be nil — a poll with no TerminalSessionRepository configured simply
// skips closing terminal_sessions on the degraded -> closed edge
// (TASK-BE-STORAGE-010); it never affects the degraded/reestablish
// transitions themselves. collector may also be nil (no Prometheus cache
// wired).
func NewPollFleetHealth(
	devServers FleetHealthPollerRepository,
	writer FleetHealthWriter,
	outbox OutboxWriter,
	agent DevServerAgentClient,
	lock PollLockPort,
	conns ConnectionRepository,
	sessions TerminalSessionRepository,
	events HealthEventPublisher,
	webhook WebhookAlerter,
	collector MetricsCollector,
	logger *slog.Logger,
) *PollFleetHealth {
	if logger == nil {
		logger = slog.Default()
	}
	return &PollFleetHealth{
		devServers: devServers, writer: writer, outbox: outbox, agent: agent, lock: lock,
		conns: conns, sessions: sessions, events: events, webhook: webhook, collector: collector, logger: logger,
	}
}

// Run ticks every interval until ctx is cancelled — called once from
// main.go as `go pollFleetHealth.Run(ctx, interval)`. Each tick's error (if
// any) is logged and never stops the ticker — one bad tick must not end
// polling for the rest of the process's lifetime.
func (uc *PollFleetHealth) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := uc.Execute(ctx); err != nil {
				uc.logger.ErrorContext(ctx, "poll_fleet_health: tick failed", slog.Any("error", err))
			}
		}
	}
}

// Execute polls every registered dev server once, in order, and persists
// its sample. A per-dev-server failure (Health erroring, or the write
// itself failing) is logged and skipped — one unreachable/misbehaving
// target must never stop the rest of the fleet from being polled in the
// same pass. When uc.lock is wired, a target another replica already holds
// the advisory lock for this tick is skipped entirely (locked=false is not
// an error).
func (uc *PollFleetHealth) Execute(ctx context.Context) error {
	servers, err := uc.devServers.ListAllDevServers(ctx)
	if err != nil {
		return err
	}
	for _, ds := range servers {
		if uc.lock != nil {
			locked, unlock, lockErr := uc.lock.TryLock(ctx, ds.ID)
			if lockErr != nil || !locked {
				continue
			}
			uc.pollOne(ctx, ds)
			unlock()
			continue
		}
		uc.pollOne(ctx, ds)
	}
	return nil
}

func (uc *PollFleetHealth) pollOne(ctx context.Context, ds domain.DevServer) {
	pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	start := time.Now()
	reachable, err := uc.agent.Health(pollCtx, ds)
	latencyMS := time.Since(start).Milliseconds()
	if err != nil {
		uc.logger.WarnContext(ctx, "poll_fleet_health: health check failed, recording unreachable",
			slog.String("devServerId", ds.ID), slog.Any("error", err))
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
	sample, err := domain.NewDevServerHealth(ds.ID, reachable, cpu, ram, disk, latencyMS)
	if err != nil {
		uc.logger.WarnContext(ctx, "poll_fleet_health: constructing sample failed, skipping",
			slog.String("devServerId", ds.ID), slog.Any("error", err))
		return
	}
	sample.Status = status
	if uc.collector != nil {
		uc.collector.Update(ds.ID, ds.Host, sample)
	}

	// Read the previous sample BEFORE UpsertFleetHealth overwrites it — this
	// is the only way to see a status/reachability edge at all, since
	// UpsertFleetHealth is a plain upsert with no history. A read failure
	// (e.g. no sample exists yet for a brand-new dev server) just disables
	// the transition checks for this one poll; it must never block
	// recording the current sample.
	previous, hadPrevious, err := uc.writer.GetDevServerHealth(ctx, ds.ID)
	if err != nil {
		uc.logger.WarnContext(ctx, "poll_fleet_health: reading previous sample failed, skipping transition check",
			slog.String("devServerId", ds.ID), slog.Any("error", err))
		hadPrevious = false
	}

	if err := uc.writer.UpsertFleetHealth(ctx, sample); err != nil {
		uc.logger.ErrorContext(ctx, "poll_fleet_health: upsert failed", slog.String("devServerId", ds.ID), slog.Any("error", err))
		return
	}

	if hadPrevious && previous.Status != status {
		uc.events.PublishStatusChange(ctx, ds, previous.Status, status)
		uc.webhook.NotifyStatusChange(ctx, ds, previous.Status, status, sample)
	}

	if hadPrevious && previous.Reachable && !sample.Reachable {
		uc.alertDevServerDisconnected(ctx, ds)
		uc.markConnectionDegraded(ctx, ds)
	}
	if hadPrevious && !previous.Reachable && sample.Reachable {
		uc.reestablishConnection(ctx, ds)
	}
}

// markConnectionDegraded transitions ds's active Connection (if any)
// established -> degraded via the domain state machine
// (BE-SOL-STORAGE-003 §2) — called on a reachable=true -> false edge, the
// same edge alertDevServerDisconnected already fires on. A connection that
// is already degraded/closed, or that doesn't exist, is left untouched
// (MarkDegraded's own guard handles the "wrong status" case; this is not an
// error, just nothing to do).
func (uc *PollFleetHealth) markConnectionDegraded(ctx context.Context, ds domain.DevServer) {
	if uc.conns == nil {
		return
	}
	conn, found, err := uc.conns.GetActiveByDevServer(ctx, ds.TenantID, ds.ID)
	if err != nil {
		uc.logger.WarnContext(ctx, "poll_fleet_health: looking up active connection failed, skipping degraded transition",
			slog.String("devServerId", ds.ID), slog.Any("error", err))
		return
	}
	if !found {
		return
	}
	if err := conn.MarkDegraded(time.Now()); err != nil {
		// Already degraded/closed — nothing to do, not an error worth logging
		// at WARN (this is the expected steady state for a still-down dev
		// server whose connection was already marked degraded on a prior poll).
		return
	}
	if err := uc.conns.UpdateStatus(ctx, ds.TenantID, conn); err != nil {
		uc.logger.WarnContext(ctx, "poll_fleet_health: persisting degraded connection status failed",
			slog.String("devServerId", ds.ID), slog.String("connectionId", conn.ID), slog.Any("error", err))
	}
}

// reestablishConnection transitions ds's active Connection (if any) back to
// established when the agent reconnects within its grace period — or, if
// the grace period already expired, closes it instead (BE-SOL-STORAGE-003
// §2's "degraded -> closed" edge). Either way this is a health-poll-driven
// transition, never an inline status assignment — see domain.Connection's
// MarkDegraded/Reestablish/CloseAfterGracePeriodExpiry doc comments.
func (uc *PollFleetHealth) reestablishConnection(ctx context.Context, ds domain.DevServer) {
	if uc.conns == nil {
		return
	}
	conn, found, err := uc.conns.GetActiveByDevServer(ctx, ds.TenantID, ds.ID)
	if err != nil {
		uc.logger.WarnContext(ctx, "poll_fleet_health: looking up active connection failed, skipping reestablish",
			slog.String("devServerId", ds.ID), slog.Any("error", err))
		return
	}
	if !found || conn.Status != domain.ConnectionStatusDegraded {
		return
	}

	now := time.Now()
	transitionedToClosed := false
	if err := conn.Reestablish(now); err != nil {
		if !errors.Is(err, domain.ErrGracePeriodExpired) {
			uc.logger.WarnContext(ctx, "poll_fleet_health: reestablish failed",
				slog.String("devServerId", ds.ID), slog.String("connectionId", conn.ID), slog.Any("error", err))
			return
		}
		// Grace period already expired before the agent came back — this is
		// a real failure, not a transient blip; close it instead (§2's
		// "degraded -> closed" edge). See BE-SOL-STORAGE-003 §4: a caller
		// observing this closed transition is what should trigger
		// FailDispatch downstream, not the earlier degraded transition.
		if closeErr := conn.CloseAfterGracePeriodExpiry(now); closeErr != nil {
			uc.logger.WarnContext(ctx, "poll_fleet_health: closing connection after grace period expiry failed",
				slog.String("devServerId", ds.ID), slog.String("connectionId", conn.ID), slog.Any("error", closeErr))
			return
		}
		transitionedToClosed = true
	}
	if err := uc.conns.UpdateStatus(ctx, ds.TenantID, conn); err != nil {
		uc.logger.WarnContext(ctx, "poll_fleet_health: persisting reestablished/closed connection status failed",
			slog.String("devServerId", ds.ID), slog.String("connectionId", conn.ID), slog.Any("error", err))
		return
	}
	// Only close terminal_sessions once the connection has genuinely
	// transitioned to closed (grace period truly expired) — NOT on the
	// earlier degraded transition (markConnectionDegraded never calls this),
	// per BE-SOL-STORAGE-003 §3/TASK-BE-STORAGE-010.
	if transitionedToClosed {
		uc.closeTerminalSessions(ctx, ds, conn)
	}
}

// closeTerminalSessions closes every terminal_sessions row bound to conn's
// ID — called ONLY after conn has genuinely transitioned to closed
// (CloseAfterGracePeriodExpiry), never on a merely degraded connection.
// uc.sessions may be nil (no TerminalSessionRepository wired), in which case
// this is a no-op, same nil-safety convention as uc.conns.
func (uc *PollFleetHealth) closeTerminalSessions(ctx context.Context, ds domain.DevServer, conn domain.Connection) {
	if uc.sessions == nil {
		return
	}
	if err := NewCloseTerminalSessionsForConnection(uc.sessions).Execute(ctx, ds.TenantID, conn.ID); err != nil {
		uc.logger.WarnContext(ctx, "poll_fleet_health: closing terminal sessions for connection failed",
			slog.String("devServerId", ds.ID), slog.String("connectionId", conn.ID), slog.Any("error", err))
	}
}

// alertDevServerDisconnected fires exactly once per true->false edge (never
// on repeated false samples — pollOne only calls this when the PREVIOUS
// sample was reachable) — a dev server down for an hour alerts once, not
// every tick.
func (uc *PollFleetHealth) alertDevServerDisconnected(ctx context.Context, ds domain.DevServer) {
	uc.logger.WarnContext(ctx, "dev server disconnected",
		slog.String("devServerId", ds.ID), slog.String("host", ds.Host), slog.String("tenantId", ds.TenantID))

	if uc.outbox == nil {
		return
	}
	payload, err := json.Marshal(domain.DevServerDisconnectedPayload{
		DevServerID: ds.ID, Host: ds.Host, TenantID: ds.TenantID,
	})
	if err != nil {
		uc.logger.WarnContext(ctx, "poll_fleet_health: marshaling disconnect alert payload failed",
			slog.String("devServerId", ds.ID), slog.Any("error", err))
		return
	}
	event := domain.OutboxEvent{
		ID:          uuid.NewString(),
		TenantID:    ds.TenantID,
		Subject:     domain.DevServerDisconnectedSubject,
		OccurredAt:  time.Now().UTC(),
		PayloadJSON: payload,
	}
	if err := uc.outbox.InsertOutboxEvent(ctx, event); err != nil {
		uc.logger.WarnContext(ctx, "poll_fleet_health: enqueuing disconnect alert failed",
			slog.String("devServerId", ds.ID), slog.Any("error", err))
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
