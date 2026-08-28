# TASK-FLEET-03-05: `usecase.PollFleetHealth` — the missing writer

**From Solution:** SOL-FLEET-03
**Priority:** P1
**Service:** `infra-fleet-service` (usecase)
**File:** `backend-go/services/infra-fleet-service/internal/usecase/poll_fleet_health.go` (new)
**Depends on:** TASK-FLEET-03-01, TASK-FLEET-03-03
**Status:** `[ ]` TODO

---

## Context

This is the headline fix for BUG-FLEET-03: a ticking background job that
polls every dev server (bounded concurrency, per-target Postgres advisory
lock so exactly one replica polls each target per interval), writes
`infra.fleet_health`, and emits a status-change event/webhook when a
server's health status flips. Reachability uses the already-real
`devserveragent.Client.Health` handshake check; metrics use the already-real
`shell.exec` JSON-RPC method — no `agent/` change needed.

## Changes to make

```go
// internal/usecase/poll_fleet_health.go
const pollConcurrency = 10 // bounded fan-out — mirrors BulkProvisionFleet's concurrency cap reasoning

const fleetMetricsScript = "cat /proc/stat; echo ---; free -b; echo ---; df -P ~"

type DevServerAgentClient interface {
    Health(ctx context.Context, ds domain.DevServer) (bool, error)
    Exec(ctx context.Context, ds domain.DevServer, method string, params map[string]any) (map[string]any, error)
}

type PollFleetHealth struct {
    devServers DevServerRepository
    health     FleetHealthWriter
    agent      DevServerAgentClient
    lock       PollLockPort
    events     HealthEventPublisher
    webhook    WebhookAlerter
    logger     *slog.Logger
}

func NewPollFleetHealth(devServers DevServerRepository, health FleetHealthWriter, agent DevServerAgentClient, lock PollLockPort, events HealthEventPublisher, webhook WebhookAlerter, logger *slog.Logger) *PollFleetHealth {
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
    sample := domain.DevServerHealth{DevServerID: ds.ID, Reachable: reachable, CPUPercent: cpu, RAMPercent: ram, DiskPercent: disk, LatencyMS: latencyMS, Status: status}

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
```

`parseFleetMetrics` is pure parsing of the `cat /proc/stat; free -b; df -P
~` output, unit-tested independently; malformed output degrades to zero
values, never panics.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run 'TestPollFleetHealth|TestParseFleetMetrics' -v
```

Expected (fake `DevServerAgentClient`/`FleetHealthWriter`/`PollLockPort`/
`HealthEventPublisher`/`WebhookAlerter`): a healthy server writes one
`UpsertFleetHealth` call with `status=healthy`; a status transition
(`healthy`→`degraded`) triggers exactly one `PublishStatusChange` and one
`NotifyStatusChange` call; no transition triggers neither; a `TryLock`
returning `locked=false` results in zero calls to `agent.Health`/`agent.Exec`
for that server.
