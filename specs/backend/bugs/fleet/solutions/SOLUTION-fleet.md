# SOLUTION: Fleet Domain — Fix tất cả Bugs

**Domain:** fleet  
**TDD Reference:** TDD-13 (Dev Server Onboarding §8 Fleet Management), TDD-05 (SSH Relay)  
**Files cần thay đổi:** `src/main/ssh/fleet-health-monitor.ts`, `src/main/dev-server/DevServerManager.ts`  
**Tổng số bugs:** 2 (BE-FLEET-001, BE-FLEET-002)

---

## BUG-BE-FLEET-001 — Fix FleetHealthMonitor not implemented

**Mức độ:** 🔴 HIGH  
**Root cause:** `FleetHealthMonitor` class tồn tại nhưng logic không implement.

### Fix — Implement FleetHealthMonitor đầy đủ

Theo TDD v5 Addendum C `remote-server CRs`:
> "Key new files: `src/main/ssh/fleet-health-monitor.ts` — Periodic health + webhook"

```typescript
// src/main/ssh/fleet-health-monitor.ts

export interface ServerHealthMetrics {
  serverId:   string
  cpu:        number      // 0-100%
  ram:        number      // MB used
  disk:       number      // GB available
  latencyMs:  number      // WS round-trip
  agentAlive: boolean
  timestamp:  number
}

export interface FleetHealthMonitorConfig {
  checkIntervalMs:  number  // default: 60_000 (60s)
  webhookUrl?:      string  // optional alert webhook
  alertThresholds: {
    cpuPercent:  number  // default: 90
    ramMb:       number  // default: 256 (alert if < 256MB free)
    diskGb:      number  // default: 5 (alert if < 5GB)
  }
}

export class FleetHealthMonitor {
  private timer?: NodeJS.Timeout
  private lastMetrics = new Map<string, ServerHealthMetrics>()

  constructor(
    private readonly devServerManager: DevServerManager,
    private readonly repository: IDevServerRepository,
    private readonly eventBus: EventBus,
    private readonly config: FleetHealthMonitorConfig,
    private readonly log: Logger,
  ) {}

  start(): void {
    this.timer = setInterval(() => this.runHealthCheck(), this.config.checkIntervalMs)
    // Run immediately on start
    void this.runHealthCheck()
    this.log.info('[FleetHealth] Monitor started')
  }

  stop(): void {
    clearInterval(this.timer)
  }

  private async runHealthCheck(): Promise<void> {
    const servers = await this.repository.listAll()

    await Promise.allSettled(servers.map(async (server) => {
      const metrics = await this.checkServer(server)
      if (!metrics) return

      await this.repository.updateHealthMetrics(server.id, metrics)

      // Emit fleet.health.updated event
      this.eventBus.emit('fleet.health.updated', metrics)

      // Check thresholds and alert
      await this.checkAlerts(metrics)
    }))
  }

  private async checkServer(server: DevServer): Promise<ServerHealthMetrics | null> {
    const bridge = this.devServerManager.getBridge(server.id)
    if (!bridge) return null

    const start = Date.now()
    try {
      // Ping agent và collect metrics
      const health = await bridge.call('system.health', {}, 10_000)
      const latencyMs = Date.now() - start

      return {
        serverId:   server.id,
        cpu:        health.cpuPercent ?? 0,
        ram:        health.ramUsedMb ?? 0,
        disk:       health.diskAvailableGb ?? 0,
        latencyMs,
        agentAlive: true,
        timestamp:  Date.now(),
      }
    } catch {
      return {
        serverId:   server.id,
        cpu:        0,
        ram:        0,
        disk:       0,
        latencyMs:  -1,
        agentAlive: false,
        timestamp:  Date.now(),
      }
    }
  }

  private async checkAlerts(metrics: ServerHealthMetrics): Promise<void> {
    const { alertThresholds } = this.config
    const alerts: string[] = []

    if (!metrics.agentAlive) alerts.push('agent_offline')
    if (metrics.cpu > alertThresholds.cpuPercent) alerts.push(`cpu_high_${metrics.cpu}pct`)
    if (metrics.disk < alertThresholds.diskGb) alerts.push(`disk_low_${metrics.disk}gb`)

    if (alerts.length === 0) return

    this.log.warn(`[FleetHealth] Alert for ${metrics.serverId}: ${alerts.join(', ')}`)
    this.eventBus.emit('fleet.health.alert', { serverId: metrics.serverId, alerts, metrics })

    // Webhook notification
    if (this.config.webhookUrl) {
      await fetch(this.config.webhookUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ serverId: metrics.serverId, alerts, metrics }),
      }).catch(err => this.log.error('[FleetHealth] Webhook failed:', err))
    }
  }

  getMetrics(serverId: string): ServerHealthMetrics | undefined {
    return this.lastMetrics.get(serverId)
  }

  getAllMetrics(): ServerHealthMetrics[] {
    return Array.from(this.lastMetrics.values())
  }
}
```

---

## BUG-BE-FLEET-002 — Fix health monitor không collect relay metrics

**Mức độ:** 🟡 MEDIUM  
**Root cause:** Health monitor chỉ collect server OS metrics, không include relay connection quality.

### Fix — Thêm relay metrics vào health check

```typescript
// src/main/ssh/fleet-health-monitor.ts — mở rộng checkServer()

private async checkServer(server: DevServer): Promise<ServerHealthMetrics | null> {
  const bridge = this.devServerManager.getBridge(server.id)
  if (!bridge) return null

  // FIX BE-FLEET-002: Collect cả relay metrics
  const relayMetrics = this.collectRelayMetrics(bridge)

  const start = Date.now()
  try {
    const health = await bridge.call('system.health', {}, 10_000)
    const latencyMs = Date.now() - start

    return {
      serverId:       server.id,
      cpu:            health.cpuPercent ?? 0,
      ram:            health.ramUsedMb ?? 0,
      disk:           health.diskAvailableGb ?? 0,
      latencyMs,
      agentAlive:     true,
      timestamp:      Date.now(),
      // Relay metrics (FIX)
      relayConnected: relayMetrics.connected,
      relayRtt:       latencyMs,                    // round-trip time
      relayDropped:   relayMetrics.droppedFrames,
      relayMbPerSec:  relayMetrics.throughputMbps,
    }
  } catch (err) {
    return {
      serverId:       server.id,
      cpu:            0, ram: 0, disk: 0,
      latencyMs:      -1,
      agentAlive:     false,
      timestamp:      Date.now(),
      relayConnected: false,
      relayRtt:       -1,
      relayDropped:   0,
      relayMbPerSec:  0,
    }
  }
}

private collectRelayMetrics(bridge: DevServerRelayBridge): RelayMetrics {
  return {
    connected:      bridge.isConnected(),
    droppedFrames:  bridge.getDroppedFrameCount(),
    throughputMbps: bridge.getThroughputMbps(),
  }
}
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/ssh/fleet-health-monitor.ts` | Implement full health monitoring | BE-FLEET-001 |
| `src/main/ssh/fleet-health-monitor.ts` | Add relay metrics collection | BE-FLEET-002 |
| `src/main/dev-server/DevServerRelayBridge.ts` | Add metrics methods (isConnected, getDropped, getThroughput) | BE-FLEET-002 |
| `src/main/db/migrations/0010_fleet_health_metrics.ts` | NEW migration for health metrics table | BE-FLEET-001 |
| `src/main/server-bootstrap.ts` | Wire FleetHealthMonitor | BE-FLEET-001 |

---

## Verification Plan

```bash
pnpm vitest run src/main/ssh/__tests__/fleet-health-monitor.test.ts

# Integration test:
# 1. Start monitor → verify health check runs at interval
# 2. Server goes offline → verify alert emitted + webhook called
# 3. Relay connected → verify latency metrics collected
# 4. High CPU (>90%) → verify cpu_high alert triggered
```
