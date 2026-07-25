# SOL-005: Fleet Health Monitoring — Backend Solution

**CR:** [CR-005](../../../../../../../docs/crs/v1/remote-server/CR-005-fleet-health-monitoring.md)  
**Backend TDD refs:** `07-runtime-service.md`, `04-rpc-server.md`, `09-ipc-handlers.md`  
**Depends on:** SOL-001, SOL-002  
**Effort:** Medium (2–3 ngày)  
**Phase:** 2

---

## 1. Phân tích backend hiện tại

Từ `TDD-07 (Runtime Service)` và code:

```typescript
// src/shared/ssh-types.ts — ĐÃ CÓ
type SshConnectionStatus =
  | 'disconnected' | 'connecting' | 'auth-failed'
  | 'deploying-relay' | 'connected' | 'reconnecting'
  | 'reconnection-failed' | 'error'

type SshConnectionState = {
  targetId: string
  status: SshConnectionStatus
  error: string | null
  reconnectAttempt: number
  remotePlatform?: SshRemotePlatform
}
```

**Gap:**
1. Connection states chỉ lưu trong memory — không persist → không có uptime history
2. `orca status` CLI chỉ check local runtime — không check remote fleet
3. Không có periodic health check
4. Không có metrics endpoint

---

## 2. Giải pháp backend

### 2.1 Fleet Health Store

```typescript
// src/main/ssh/fleet-health-store.ts — NEW FILE

/**
 * Persist fleet health state:
 * - Connection status history (24h rolling)
 * - Last seen timestamps
 * - Relay versions
 */

type HealthRecord = {
  targetId: string
  timestamp: number       // Unix ms
  status: SshConnectionStatus
  error?: string
  relayVersion?: string   // relay binary version string
  remotePlatform?: SshRemotePlatform
  pingLatencyMs?: number
}

type UptimeWindow = {
  windowMs: number        // e.g. 86400000 (24h)
  uptimeMs: number        // total connected time in window
  uptimePercent: number   // 0-100
}

const HEALTH_HISTORY_MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000  // 7 days

class FleetHealthStore {
  private history: Map<string, HealthRecord[]> = new Map()  // targetId → records
  private connectedSince: Map<string, number> = new Map()   // targetId → timestamp

  recordConnectionState(state: SshConnectionState, relayVersion?: string): void {
    const record: HealthRecord = {
      targetId: state.targetId,
      timestamp: Date.now(),
      status: state.status,
      error: state.error ?? undefined,
      relayVersion,
      remotePlatform: state.remotePlatform,
    }

    const records = this.history.get(state.targetId) ?? []
    records.push(record)

    // Prune old records
    const cutoff = Date.now() - HEALTH_HISTORY_MAX_AGE_MS
    const pruned = records.filter(r => r.timestamp > cutoff)
    this.history.set(state.targetId, pruned)

    // Track connected since
    if (state.status === 'connected' && !this.connectedSince.has(state.targetId)) {
      this.connectedSince.set(state.targetId, Date.now())
    } else if (state.status !== 'connected') {
      this.connectedSince.delete(state.targetId)
    }
  }

  getUptimeForTarget(targetId: string, windowMs = 24 * 60 * 60 * 1000): UptimeWindow {
    const records = this.history.get(targetId) ?? []
    const cutoff = Date.now() - windowMs
    const windowRecords = records.filter(r => r.timestamp > cutoff)

    if (!windowRecords.length) return { windowMs, uptimeMs: 0, uptimePercent: 0 }

    // Calculate connected time (status transitions)
    let uptimeMs = 0
    let lastConnectedAt: number | null = null

    for (let i = 0; i < windowRecords.length; i++) {
      const record = windowRecords[i]
      if (record.status === 'connected' && !lastConnectedAt) {
        lastConnectedAt = record.timestamp
      } else if (record.status !== 'connected' && lastConnectedAt) {
        uptimeMs += record.timestamp - lastConnectedAt
        lastConnectedAt = null
      }
    }
    // If still connected:
    if (lastConnectedAt) {
      uptimeMs += Date.now() - lastConnectedAt
    }

    return {
      windowMs,
      uptimeMs,
      uptimePercent: Math.round((uptimeMs / windowMs) * 100 * 10) / 10,
    }
  }

  getConnectedSince(targetId: string): number | null {
    return this.connectedSince.get(targetId) ?? null
  }

  getLastRecord(targetId: string): HealthRecord | null {
    const records = this.history.get(targetId) ?? []
    return records[records.length - 1] ?? null
  }
}
```

### 2.2 Fleet Status Aggregator trong Runtime Service

```typescript
// src/main/runtime/orca-runtime.ts — ADD METHOD

async getFleetStatus(filter?: { project?: string; team?: string }): Promise<FleetStatusReport> {
  let targets = sshConnectionStore.listTargets()
  if (filter?.project) targets = targets.filter(t => t.project === filter.project)
  if (filter?.team) targets = targets.filter(t => t.team === filter.team)

  const servers: FleetServerStatus[] = targets.map(target => {
    const connState = sshManager.getConnectionState(target.id)
    const healthRecord = fleetHealthStore.getLastRecord(target.id)
    const connectedSince = fleetHealthStore.getConnectedSince(target.id)
    const uptime24h = fleetHealthStore.getUptimeForTarget(target.id, 24 * 60 * 60 * 1000)

    const uptimeSeconds = connectedSince ? Math.round((Date.now() - connectedSince) / 1000) : 0

    return {
      id: target.fleetId ?? target.id,
      label: target.label,
      host: target.host,
      project: target.project,
      team: target.team,
      environment: target.environment,
      status: connState?.status ?? 'disconnected',
      error: connState?.error ?? null,
      uptimeSeconds,
      uptimePercent24h: uptime24h.uptimePercent,
      relayVersion: healthRecord?.relayVersion ?? null,
      lastSeenAt: healthRecord?.timestamp ?? null,
      reconnectAttempt: connState?.reconnectAttempt ?? 0,
    }
  })

  const connected = servers.filter(s => s.status === 'connected').length
  const disconnected = servers.filter(s => s.status === 'disconnected').length
  const error = servers.filter(s => s.status === 'error' || s.status === 'reconnection-failed').length

  return {
    generatedAt: Date.now(),
    servers,
    summary: {
      total: servers.length,
      connected,
      disconnected,
      error,
      healthScore: Math.round((connected / Math.max(servers.length, 1)) * 100),
    },
  }
}

type FleetServerStatus = {
  id: string
  label: string
  host: string
  project?: string
  team?: string
  environment?: string
  status: SshConnectionStatus
  error: string | null
  uptimeSeconds: number
  uptimePercent24h: number
  relayVersion: string | null
  lastSeenAt: number | null
  reconnectAttempt: number
}

type FleetStatusReport = {
  generatedAt: number
  servers: FleetServerStatus[]
  summary: {
    total: number
    connected: number
    disconnected: number
    error: number
    healthScore: number  // 0-100
  }
}
```

### 2.3 Periodic Health Ping

```typescript
// src/main/ssh/fleet-health-monitor.ts — NEW FILE

const HEALTH_PING_INTERVAL_MS = 60_000  // 1 minute

class FleetHealthMonitor {
  private intervalId: NodeJS.Timeout | null = null

  start(): void {
    this.intervalId = setInterval(async () => {
      await this.runHealthCheck()
    }, HEALTH_PING_INTERVAL_MS)
  }

  stop(): void {
    if (this.intervalId) {
      clearInterval(this.intervalId)
      this.intervalId = null
    }
  }

  async runHealthCheck(): Promise<void> {
    const targets = sshConnectionStore.listTargets()

    for (const target of targets) {
      const connState = sshManager.getConnectionState(target.id)

      // Record current state in health store
      fleetHealthStore.recordConnectionState(
        connState ?? { targetId: target.id, status: 'disconnected', error: null, reconnectAttempt: 0 }
      )

      // Notify if status changed to error
      if (connState?.status === 'error' || connState?.status === 'reconnection-failed') {
        this.emitAlert({
          targetId: target.id,
          label: target.label,
          project: target.project,
          status: connState.status,
          error: connState.error,
        })
      }
    }
  }

  private emitAlert(alert: FleetAlert): void {
    // Notify renderer via IPC event
    BrowserWindow.getAllWindows().forEach(win => {
      win.webContents.send('fleet:serverAlert', alert)
    })

    // Send webhook if configured
    const webhookUrl = globalSettings?.fleetAlertWebhookUrl
    if (webhookUrl) {
      this.sendWebhookAlert(webhookUrl, alert).catch(err => {
        logger.error('Fleet webhook failed', { err })
      })
    }
  }

  private async sendWebhookAlert(url: string, alert: FleetAlert): Promise<void> {
    const payload = {
      text: `⚠️ Orca Fleet Alert: ${alert.label} (${alert.project}) → ${alert.status}`,
      attachments: [{
        color: 'danger',
        fields: [
          { title: 'Server', value: alert.label, short: true },
          { title: 'Status', value: alert.status, short: true },
          { title: 'Error', value: alert.error ?? 'No details', short: false },
        ],
      }],
    }
    await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
  }
}

type FleetAlert = {
  targetId: string
  label: string
  project?: string
  status: SshConnectionStatus
  error: string | null
}
```

### 2.4 IPC Handlers

```typescript
// src/main/ipc/ssh.ts — ADD HANDLERS

// Handler: fleet.getStatus
ipcMain.handle('fleet:getStatus', async (_event, filter?: { project?: string; team?: string }) => {
  return orcaRuntime.getFleetStatus(filter)
})

// Handler: fleet.getUptimeHistory
ipcMain.handle('fleet:getUptimeHistory', async (_event, { targetId, windowMs }: {
  targetId: string
  windowMs?: number
}) => {
  return fleetHealthStore.getUptimeForTarget(targetId, windowMs)
})

// Handler: fleet.setAlertWebhook
ipcMain.handle('fleet:setAlertWebhook', async (_event, { url }: { url: string | null }) => {
  await updateGlobalSettings({ fleetAlertWebhookUrl: url ?? undefined })
  return { ok: true }
})
```

### 2.5 `orca fleet status` CLI

```typescript
// src/cli/handlers/fleet.ts — ADD

async function handleFleetStatus(args: {
  project?: string
  team?: string
  json?: boolean
}): Promise<void> {
  const report: FleetStatusReport = await callRuntimeIpc('fleet:getStatus', {
    project: args.project,
    team: args.team,
  })

  if (args.json) {
    console.log(JSON.stringify(report, null, 2))
    return
  }

  const col = (s: string, w: number) => String(s ?? '').padEnd(w).substring(0, w)

  console.log(`\nFleet Health — ${new Date(report.generatedAt).toLocaleString()}`)
  console.log('─'.repeat(90))
  console.log([
    col('SERVER', 20), col('PROJECT', 16), col('STATUS', 18),
    col('UPTIME', 10), col('24H%', 6), col('RELAY', 8),
  ].join('  '))
  console.log('─'.repeat(90))

  for (const server of report.servers) {
    const status = formatStatus(server.status)
    const uptime = formatUptime(server.uptimeSeconds)
    console.log([
      col(server.id, 20), col(server.project ?? '—', 16), col(status, 18),
      col(uptime, 10), col(`${server.uptimePercent24h}%`, 6), col(server.relayVersion ?? 'N/A', 8),
    ].join('  '))
  }

  console.log('─'.repeat(90))
  const s = report.summary
  console.log(`Summary: ${s.connected}/${s.total} connected | ${s.error} error | Health: ${s.healthScore}%`)
  process.exit(s.error > 0 ? 1 : 0)
}

function formatUptime(seconds: number): string {
  if (seconds === 0) return '0s'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function formatStatus(status: SshConnectionStatus): string {
  const map: Record<string, string> = {
    'connected': '✅ Connected',
    'connecting': '⊙  Connecting',
    'disconnected': '⚪ Disconnected',
    'deploying-relay': '⊙  Deploying relay',
    'reconnecting': '↻  Reconnecting',
    'reconnection-failed': '❌ Reconnect failed',
    'auth-failed': '🔑 Auth failed',
    'error': '❌ Error',
  }
  return map[status] ?? status
}
```

### 2.6 Prometheus Metrics Endpoint (optional)

```typescript
// src/main/runtime/rpc/fleet-metrics-handler.ts — NEW (optional)

// HTTP GET /metrics → Prometheus format
export function handleMetricsRequest(
  req: http.IncomingMessage,
  res: http.ServerResponse
): void {
  const report = orcaRuntime.getFleetStatusSync()  // sync version

  let output = ''
  output += '# HELP orca_server_connected Whether Orca relay is connected\n'
  output += '# TYPE orca_server_connected gauge\n'

  for (const server of report.servers) {
    const labels = `server="${server.id}",project="${server.project ?? ''}",team="${server.team ?? ''}"`
    output += `orca_server_connected{${labels}} ${server.status === 'connected' ? 1 : 0}\n`
  }

  output += '\n# HELP orca_server_uptime_seconds Current uptime in seconds\n'
  output += '# TYPE orca_server_uptime_seconds gauge\n'
  for (const server of report.servers) {
    const labels = `server="${server.id}"`
    output += `orca_server_uptime_seconds{${labels}} ${server.uptimeSeconds}\n`
  }

  output += '\n# HELP orca_fleet_health_score Fleet health score 0-100\n'
  output += '# TYPE orca_fleet_health_score gauge\n'
  output += `orca_fleet_health_score ${report.summary.healthScore}\n`

  res.writeHead(200, { 'Content-Type': 'text/plain; version=0.0.4' })
  res.end(output)
}
```

---

## 3. Files cần thay đổi

| File | Action | Chi tiết |
|------|--------|---------|
| `src/main/ssh/fleet-health-store.ts` | **NEW** | Health history, uptime calc |
| `src/main/ssh/fleet-health-monitor.ts` | **NEW** | Periodic ping, webhook alerts |
| `src/main/runtime/orca-runtime.ts` | MODIFY | `getFleetStatus()`, start health monitor |
| `src/main/ipc/ssh.ts` | MODIFY | 3 IPC handlers mới |
| `src/cli/handlers/fleet.ts` | MODIFY | `handleFleetStatus()` |
| `src/main/runtime/rpc/fleet-metrics-handler.ts` | **NEW** (optional) | Prometheus /metrics |
| `src/shared/types.ts` | MODIFY | Thêm `fleetAlertWebhookUrl` vào GlobalSettings |

---

## 4. GlobalSettings extension

```typescript
// src/shared/types.ts — ADD to GlobalSettings
type GlobalSettings = {
  // ... existing fields ...
  fleetAlertWebhookUrl?: string    // Slack/Discord webhook for fleet alerts
  fleetHealthPingIntervalMs?: number  // default 60000
  fleetMetricsEnabled?: boolean    // enable /metrics endpoint
}
```

---

## 5. Implementation Status

> **✅ IMPLEMENTED — Phase 2 Complete**  
> Ngày: 2026-07-22

### Đã triển khai

| File | Status | Deviation từ spec |
|------|--------|-------------------|
| [`src/main/ssh/fleet-health-store.ts`](../../../../../src/main/ssh/fleet-health-store.ts) | ✅ Done | **NEW** — In-memory 7-day rolling history, `getUptimeForTarget()`, `fleetHealthStore` singleton |
| [`src/main/ssh/fleet-health-monitor.ts`](../../../../../src/main/ssh/fleet-health-monitor.ts) | ✅ Done | **NEW** — DI via properties, error-state transition alerts, `fleet:serverAlert` IPC event |
| [`src/main/ssh/fleet-status-service.ts`](../../../../../src/main/ssh/fleet-status-service.ts) | ✅ Done | **NEW** Standalone (không modify `orca-runtime.ts`) — `getFleetStatus(filter?)` |
| [`src/shared/fleet-types.ts`](../../../../../src/shared/fleet-types.ts) | ✅ Done | **NEW** — `FleetServerStatus`, `FleetStatusReport` shared types |
| [`src/main/ipc/ssh.ts`](../../../../../src/main/ipc/ssh.ts) | ✅ Done | `fleet:getStatus`, `fleet:getUptimeHistory`, `fleet:setAlertWebhook` handlers |
| [`src/cli/handlers/fleet.ts`](../../../../../src/cli/handlers/fleet.ts) | ✅ Done | `fleet status` command với table output, health summary, exit code 1 on errors |
| [`src/main/runtime/rpc/fleet-metrics-handler.ts`](../../../../../src/main/runtime/rpc/fleet-metrics-handler.ts) | ✅ Done | **NEW** — Prometheus `/metrics` factory. Metrics: `orca_server_connected`, `uptime_seconds`, `uptime_24h_percent`, `reconnect_attempts`, fleet aggregates |
| [`src/main/runtime/rpc/methods/ssh.ts`](../../../../../src/main/runtime/rpc/methods/ssh.ts) | ✅ Done | `ssh.getFleetStatus`, `ssh.getUptimeHistory` RPC methods |

### Deviation từ design gốc

> **GlobalSettings** (`fleetAlertWebhookUrl`, `fleetHealthPingIntervalMs`, `fleetMetricsEnabled`) chưa được thêm vào `src/shared/types.ts` — webhook URL hiện được set runtime qua `fleet:setAlertWebhook` IPC thay vì persist trong settings. Đây là pending work nếu cần persistence qua restart.

### Notes

- **TASK-015** (fleet-health-store): ✅ Done  
- **TASK-016** (fleet-health-monitor): ✅ Done  
- **TASK-017** (getFleetStatus + fleet-types): ✅ Done  
- **TASK-018** (IPC handlers fleet:*): ✅ Done  
- **TASK-019** (CLI fleet status): ✅ Done  
- **TASK-020** (Prometheus metrics): ✅ Done

### Pending (không block)

- [x] `fleetAlertWebhookUrl` persist vào `GlobalSettings` qua `types.ts`
- [x] Wire `fleetHealthMonitor.start()` vào app startup (hiện cần gọi thủ công)
- [x] Wire `createFleetMetricsHandler()` vào HTTP server của runtime-rpc.ts
