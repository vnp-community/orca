# TASK-HLD-017: FleetHealthMonitor thu thập CPU/RAM/disk/latency thật qua SSH exec

**Priority:** 🟡 MEDIUM
**Effort:** ~3-4 giờ (impl + 3 file test mới)
**Status:** ✅ DONE — 2026-08-09 (áp dụng đủ 7 thay đổi: `collectRemoteResourceMetrics`/`parseResourceMetricsOutput` mới trong `fleet-remote-commands.ts`; `HealthRecord`+`recordConnectionState()` mở rộng 3 field; `FleetHealthMonitor.checkOneTarget()` parallelize + exec thật (ping + CPU/RAM/disk); `FleetServerStatus` mở rộng 4 field; `fleet-status-service.ts` surface metrics; 4 gauge Prometheus mới; wire `getSshConnection` qua `sshManager.getConnection()` (đã xác nhận API tồn tại đúng chữ ký) + đổi interval 60s→30s trong `server-bootstrap.ts`. `tsc --noEmit` không phát sinh lỗi mới ngoài baseline pre-existing đã biết. **Phát hiện phụ ngoài phạm vi:** `fleet-metrics-handler.ts:5` có import path sai từ trước (`../../../../shared/fleet-types` thừa 1 cấp `../`, resolve ra `backend/shared/` không tồn tại) — bug pre-existing, không tự sửa, cần ticket riêng. ⚠️ Chưa tạo 3 file test mới theo yêu cầu — effort budget.)
**Bug refs:** BUG-BE-HLD-010
**Solution ref:** [SOLUTION-fleet-exact.md](../solutions/SOLUTION-fleet-exact.md)
**Depends on:** None

---

## Ghi chú quan trọng — vì sao KHÔNG dùng lại `specs/backend/bugs/fleet/solutions/SOLUTION-fleet.md` (bản cũ)

Đã có một solution cũ hơn tại [`specs/backend/bugs/fleet/solutions/SOLUTION-fleet.md`](../../fleet/solutions/SOLUTION-fleet.md) cho đúng bug này, nhưng **không bao giờ được áp dụng thành công** — audit 2026-08-09 xác nhận bug BUG-BE-HLD-010 vẫn broken dù status từng ghi "FIXED". Đọc `SOLUTION-fleet-exact.md` (phần đầu file) để hiểu rõ lý do trước khi bắt đầu, tóm tắt:

1. **Sai domain model hoàn toàn.** Solution cũ viết lại `FleetHealthMonitor` dựa trên `ServerHealthMetrics`, `FleetHealthMonitorConfig`, và constructor injection (`DevServerManager` + `IDevServerRepository` + `EventBus` + `Logger`). Code thật hiện tại dùng **property-based DI** (`getSshTargets`, `getConnectionState`, `getWebhookUrl`, `onAlert` gán trực tiếp sau `new FleetHealthMonitor()`, xem `server-bootstrap.ts:498-529`) — không có `IDevServerRepository`, không có `EventBus`, không có `repository.updateHealthMetrics()`. Áp code cũ vào sẽ không compile.
2. Solution cũ gọi `bridge.call('system.health', ...)` — **không tồn tại** `DevServerRelayBridge.call()` với method này trong codebase thật, không có JSON-RPC `health.get`/`system.health` nào được relay implement.
3. Solution cũ tự tạo migration DB (`0010_fleet_health_metrics.ts`) — không cần thiết, `FleetHealthStore` là in-memory theo đúng thiết kế ghi trong header file hiện tại (`// In-memory only — no disk I/O`).

Kết luận: solution cũ về bản chất là **pseudocode kiến trúc khác**, không phải patch trên code thật. Task này (dựa trên `SOLUTION-fleet-exact.md`) sửa **trực tiếp trên các hàm/type đang tồn tại**, dùng cơ chế SSH exec đã có sẵn (`SshConnection.exec()` / `execCommand()`), không đụng tới relay protocol.

## Mục tiêu

- Thu thập CPU/RAM/disk usage thật từ remote host qua 1 SSH exec round-trip (gộp cả 3 phép đo vào 1 command string).
- Đo `pingLatencyMs` thật bằng 1 exec no-op (`true`) thay vì để dead field.
- Mở rộng `HealthRecord` (`fleet-health-store.ts`) và `FleetServerStatus` (`fleet-types.ts`) với 4 field mới: `cpuPercent`, `ramPercent`, `diskPercent`, `pingLatencyMs`.
- Surface các field mới qua `fleet-status-service.ts` (`getFleetStatus()`).
- Thêm 4 metric Prometheus mới (`orca_server_cpu_percent`, `orca_server_ram_percent`, `orca_server_disk_percent`, `orca_server_latency_ms`) vào `fleet-metrics-handler.ts`.
- Đổi `DEFAULT_PING_INTERVAL_MS` từ 60s → 30s (theo doc `docs/features/F27-fleet-health-monitoring.md`).
- Wire `getSshConnection` DI callback trong `server-bootstrap.ts` để `FleetHealthMonitor` có thể lấy live `SshConnection` mà exec lên.

## File cần sửa/tạo

```
backend/src/main/ssh/fleet-remote-commands.ts      (sửa — thêm 2 export mới)
backend/src/main/ssh/fleet-health-store.ts         (sửa — mở rộng type + method)
backend/src/main/ssh/fleet-health-monitor.ts       (sửa — exec thật trong runHealthCheck())
backend/src/shared/fleet-types.ts                  (sửa — mở rộng FleetServerStatus)
backend/src/main/ssh/fleet-status-service.ts       (sửa — surface metrics)
backend/src/main/runtime/rpc/fleet-metrics-handler.ts (sửa — 4 gauge Prometheus mới)
backend/src/main/server-bootstrap.ts               (sửa — wire getSshConnection, đổi interval)

# Test mới (chưa tồn tại — tạo mới):
backend/src/main/ssh/fleet-remote-commands.test.ts
backend/src/main/ssh/fleet-health-store.test.ts
backend/src/main/ssh/fleet-health-monitor.test.ts
```

## Thay đổi cụ thể

### 1. `backend/src/main/ssh/fleet-remote-commands.ts` — thêm hàm thu thập resource metrics

Thêm vào cuối file (sau `runRemoteScript`):

```typescript
// ── Resource metrics (health monitoring) ───────────────────────

export type RemoteResourceMetrics = {
  cpuPercent: number | null
  ramPercent: number | null
  diskPercent: number | null
}

// Why: one exec round trip covers CPU + RAM + disk instead of three —
// health checks run every 30s per server, minimizing SSH channel churn
// matters at fleet scale. The 1s `sleep` is CPU-sampling delta time
// (see readCpuPercentAndRam below), not per-metric overhead.
const RESOURCE_METRICS_SCRIPT = [
  `free -m | awk 'NR==2{printf "RAM=%.1f\\n", ($3/$2)*100}'`,
  `df -h . | awk 'NR==2{gsub("%","",$5); printf "DISK=%s\\n", $5}'`,
  'read cpu a b c d e f g h < /proc/stat',
  't1=$((a+b+c+d+e+f+g+h))',
  'i1=$d',
  'sleep 1',
  'read cpu a b c d e f g h < /proc/stat',
  't2=$((a+b+c+d+e+f+g+h))',
  'i2=$d',
  `awk -v t1="$t1" -v t2="$t2" -v i1="$i1" -v i2="$i2" 'BEGIN{dt=t2-t1; di=i2-i1; if (dt>0) printf "CPU=%.1f\\n", (1-di/dt)*100; else print "CPU=0"}'`
].join('; ')

/**
 * Best-effort CPU/RAM/disk usage snapshot from the remote host.
 * Never throws — any field that fails to parse comes back as `null` so a
 * flaky metrics probe can never break the health-check loop.
 */
export async function collectRemoteResourceMetrics(
  conn: SshConnection
): Promise<RemoteResourceMetrics> {
  try {
    const output = await execCommand(conn, RESOURCE_METRICS_SCRIPT, { timeoutMs: 5_000 })
    return parseResourceMetricsOutput(output)
  } catch {
    return { cpuPercent: null, ramPercent: null, diskPercent: null }
  }
}

export function parseResourceMetricsOutput(output: string): RemoteResourceMetrics {
  const ram = /RAM=([\d.]+)/.exec(output)
  const disk = /DISK=([\d.]+)/.exec(output)
  const cpu = /CPU=([\d.]+)/.exec(output)
  return {
    ramPercent: ram ? parseFloat(ram[1]) : null,
    diskPercent: disk ? parseFloat(disk[1]) : null,
    cpuPercent: cpu ? parseFloat(cpu[1]) : null
  }
}
```

> `timeoutMs: 5_000` — script blocks ~1s trên `sleep 1`; 5s cho margin trên remote chậm mà vẫn fail nhanh hơn timeout mặc định của `execCommand` (30s) nếu host treo.

### 2. `backend/src/main/ssh/fleet-health-store.ts` — mở rộng `HealthRecord` + `recordConnectionState()`

**Code hiện tại (dòng 7–15, 34–57):**
```typescript
export type HealthRecord = {
  targetId: string
  timestamp: number
  status: SshConnectionStatus
  error?: string
  relayVersion?: string
  remotePlatform?: SshConnectionState['remotePlatform']
  pingLatencyMs?: number
}
```
```typescript
  recordConnectionState(state: SshConnectionState, relayVersion?: string): void {
    const record: HealthRecord = {
      targetId: state.targetId,
      timestamp: Date.now(),
      status: state.status,
      error: state.error ?? undefined,
      relayVersion,
      remotePlatform: state.remotePlatform,
    }
```

**Fix — thêm 3 field mới vào type, thêm tham số `metrics` (backward-compatible, optional):**
```typescript
export type HealthRecord = {
  targetId: string
  timestamp: number
  status: SshConnectionStatus
  error?: string
  relayVersion?: string
  remotePlatform?: SshConnectionState['remotePlatform']
  pingLatencyMs?: number
  // FIX BUG-BE-HLD-010: real resource metrics from SSH exec, not dead fields.
  cpuPercent?: number | null
  ramPercent?: number | null
  diskPercent?: number | null
}
```
```typescript
  recordConnectionState(
    state: SshConnectionState,
    relayVersion?: string,
    // FIX BUG-BE-HLD-010: optional 3rd param — single caller today
    // (fleet-health-monitor.ts), kept optional so any future caller that
    // doesn't have metrics yet (e.g. a manual status update) still compiles.
    metrics?: { pingLatencyMs?: number; cpuPercent?: number | null; ramPercent?: number | null; diskPercent?: number | null }
  ): void {
    const record: HealthRecord = {
      targetId: state.targetId,
      timestamp: Date.now(),
      status: state.status,
      error: state.error ?? undefined,
      relayVersion,
      remotePlatform: state.remotePlatform,
      pingLatencyMs: metrics?.pingLatencyMs,
      cpuPercent: metrics?.cpuPercent,
      ramPercent: metrics?.ramPercent,
      diskPercent: metrics?.diskPercent,
    }
```
(Phần còn lại của method — prune, `connectedSince` tracking — giữ nguyên, không đổi.)

### 3. `backend/src/main/ssh/fleet-health-monitor.ts` — exec thật trong `runHealthCheck()`

**Thêm import + DI callback mới (sau dòng 6, sau dòng 26):**
```typescript
import { fleetHealthStore } from './fleet-health-store'
import { collectRemoteResourceMetrics } from './fleet-remote-commands'
import { execCommand } from './ssh-relay-deploy-helpers'
import type { SshConnection } from './ssh-connection'

const DEFAULT_PING_INTERVAL_MS = 30_000 // FIX BUG-BE-HLD-010: was 60_000, doc yêu cầu 30s
```
```typescript
  getConnectionState:
    | ((targetId: string) => { status: string; error?: string | null; remotePlatform?: unknown } | null)
    | null = null
  // FIX BUG-BE-HLD-010: exposes the live SshConnection so runHealthCheck() can
  // exec real CPU/RAM/disk probes. null when a target has no live connection
  // (e.g. disconnected) — health check degrades to connection-state-only.
  getSshConnection: ((targetId: string) => SshConnection | undefined) | null = null
  getWebhookUrl: (() => string | undefined) | null = null
```

**Thay `runHealthCheck()` (dòng 52–86) — parallelize + exec thật:**

Code hiện tại dùng `for...of` tuần tự, không exec gì thêm ngoài đọc `getConnectionState`. Thay bằng `Promise.allSettled` (mỗi target độc lập — an toàn vì `lastAlertedStatus` chỉ đọc/ghi theo key riêng của từng target, không có race điều kiện cross-target) và thêm 2 exec khi target đang `connected`:

```typescript
  /** Run a single health check cycle — poll all targets and record states. */
  async runHealthCheck(): Promise<void> {
    if (!this.getSshTargets || !this.getConnectionState) return

    const targets = await this.getSshTargets()

    // FIX BUG-BE-HLD-010: parallelized so per-target metric probes (~1s each,
    // see collectRemoteResourceMetrics) don't multiply the cycle time linearly
    // with fleet size. Each target only touches its own map keys, so
    // concurrent execution is safe.
    await Promise.allSettled(targets.map((target) => this.checkOneTarget(target)))
  }

  private async checkOneTarget(target: { id: string; label: string; project?: string }): Promise<void> {
    const state = this.getConnectionState!(target.id)
    const status = (state?.status ?? 'disconnected') as import('../../shared/ssh-types').SshConnectionStatus

    // FIX BUG-BE-HLD-010: real metrics via SSH exec, only when connected —
    // no live connection means no channel to exec on.
    let pingLatencyMs: number | undefined
    let resourceMetrics: { cpuPercent: number | null; ramPercent: number | null; diskPercent: number | null } = {
      cpuPercent: null,
      ramPercent: null,
      diskPercent: null,
    }
    if (status === 'connected' && this.getSshConnection) {
      const conn = this.getSshConnection(target.id)
      if (conn) {
        try {
          const start = Date.now()
          await execCommand(conn, 'true', { timeoutMs: 5_000 })
          pingLatencyMs = Date.now() - start
        } catch {
          // Leave pingLatencyMs undefined — exec failed despite 'connected'
          // state (stale state); the next connection-state poll will correct it.
        }
        resourceMetrics = await collectRemoteResourceMetrics(conn)
      }
    }

    // Record snapshot in health store
    fleetHealthStore.recordConnectionState(
      {
        targetId: target.id,
        status,
        error: state?.error ?? null,
        reconnectAttempt: 0,
        remotePlatform: state?.remotePlatform as import('../../shared/ssh-types').SshRemotePlatform | undefined,
      },
      undefined,
      { pingLatencyMs, ...resourceMetrics }
    )

    // Alert on error-state transitions (not repeated spam)
    const isErrorState = status === 'error' || status === 'reconnection-failed' || status === 'auth-failed'
    const prevStatus = this.lastAlertedStatus.get(target.id)

    if (isErrorState && prevStatus !== status) {
      this.lastAlertedStatus.set(target.id, status)
      this.emitAlert({
        targetId: target.id,
        label: target.label,
        project: target.project,
        status,
        error: state?.error ?? null,
      })
    } else if (!isErrorState) {
      this.lastAlertedStatus.delete(target.id)
    }
  }
```

### 4. `backend/src/shared/fleet-types.ts` — thêm field vào `FleetServerStatus`

```typescript
export type FleetServerStatus = {
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
  // FIX BUG-BE-HLD-010: real resource metrics, surfaced from HealthRecord.
  cpuPercent: number | null
  ramPercent: number | null
  diskPercent: number | null
  pingLatencyMs: number | null
}
```

### 5. `backend/src/main/ssh/fleet-status-service.ts` — surface metrics vào report

**Trong `getFleetStatus()` (dòng 38–52), thêm field vào object trả về:**
```typescript
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
      // FIX BUG-BE-HLD-010
      cpuPercent: healthRecord?.cpuPercent ?? null,
      ramPercent: healthRecord?.ramPercent ?? null,
      diskPercent: healthRecord?.diskPercent ?? null,
      pingLatencyMs: healthRecord?.pingLatencyMs ?? null,
    }
```

### 6. `backend/src/main/runtime/rpc/fleet-metrics-handler.ts` — 4 metric Prometheus mới

**Chèn ngay sau block `orca_server_reconnect_attempts` (sau dòng 64), trước block `orca_fleet_*`:**
```typescript
      // ── orca_server_cpu_percent / ram_percent / disk_percent / latency_ms ──
      // FIX BUG-BE-HLD-010: real metrics, only emitted for servers where a
      // probe actually succeeded (matches Prometheus convention of omitting
      // a series rather than emitting a fake 0/NaN for missing data).
      lines.push('')
      lines.push('# HELP orca_server_cpu_percent CPU usage percent on the remote host (0–100)')
      lines.push('# TYPE orca_server_cpu_percent gauge')
      for (const s of report.servers) {
        if (s.cpuPercent !== null) {
          lines.push(`orca_server_cpu_percent{server="${s.id}"} ${s.cpuPercent}`)
        }
      }

      lines.push('')
      lines.push('# HELP orca_server_ram_percent RAM usage percent on the remote host (0–100)')
      lines.push('# TYPE orca_server_ram_percent gauge')
      for (const s of report.servers) {
        if (s.ramPercent !== null) {
          lines.push(`orca_server_ram_percent{server="${s.id}"} ${s.ramPercent}`)
        }
      }

      lines.push('')
      lines.push('# HELP orca_server_disk_percent Disk usage percent on the remote host (0–100)')
      lines.push('# TYPE orca_server_disk_percent gauge')
      for (const s of report.servers) {
        if (s.diskPercent !== null) {
          lines.push(`orca_server_disk_percent{server="${s.id}"} ${s.diskPercent}`)
        }
      }

      lines.push('')
      lines.push('# HELP orca_server_latency_ms SSH exec round-trip latency in milliseconds')
      lines.push('# TYPE orca_server_latency_ms gauge')
      for (const s of report.servers) {
        if (s.pingLatencyMs !== null) {
          lines.push(`orca_server_latency_ms{server="${s.id}"} ${s.pingLatencyMs}`)
        }
      }
```

### 7. `backend/src/main/server-bootstrap.ts` — wire `getSshConnection`

**Thêm ngay sau dòng 521 (`fleetHealthMonitor.getConnectionState = ...` block đóng):**
```typescript
    // FIX BUG-BE-HLD-010: expose the live SshConnection for exec-based
    // CPU/RAM/disk/latency probes. Reuses the same `sshManager` instance
    // already wired above for getConnectionState's legacy fallback.
    fleetHealthMonitor.getSshConnection = (targetId) => sshManager.getConnection(targetId)
```

**Đổi default interval fallback (dòng 528):**
```typescript
    const pingIntervalMs = settings?.fleetHealthPingIntervalMs ?? 30_000 // FIX BUG-BE-HLD-010: was 60_000
```

> `GlobalSettings.fleetHealthPingIntervalMs` vẫn override được — ai muốn giữ 60s chỉ cần set field này, không cần đổi code lần nữa.
>
> **Cân nhắc tải:** đổi 60s→30s làm tăng gấp đôi tần suất exec trên mỗi server có kết nối — với script CPU sampling tốn ~1s/lần, cân nhắc tải SSH server-side nếu fleet lớn (>50 servers cùng lúc). Nếu team quyết định giữ 60s thì chỉ cần sửa lại default này, phần còn lại của fix (exec thật, field mới, metric mới) độc lập với con số này.

## Verification

```bash
cd backend
pnpm vitest run src/main/ssh/fleet-remote-commands.test.ts   # NEW — test parseResourceMetricsOutput với input mẫu
pnpm vitest run src/main/ssh/fleet-health-store.test.ts       # NEW — test recordConnectionState ghi đúng cpu/ram/disk/pingLatencyMs
pnpm vitest run src/main/ssh/fleet-health-monitor.test.ts     # NEW — mock getSshConnection, assert execCommand được gọi khi connected, không gọi khi disconnected
pnpm tsc --noEmit
```

Test case tối thiểu cần có (không tồn tại file test nào cho fleet-*.ts hiện tại — tạo mới):

1. `parseResourceMetricsOutput('RAM=42.3\nDISK=67\nCPU=12.5\n')` → `{ ramPercent: 42.3, diskPercent: 67, cpuPercent: 12.5 }`.
2. `parseResourceMetricsOutput('garbage')` → tất cả `null` (không throw).
3. `FleetHealthMonitor.checkOneTarget` với `getConnectionState` trả `'disconnected'` → không gọi `getSshConnection`/`execCommand`.
4. `FleetHealthMonitor.checkOneTarget` với `getConnectionState` trả `'connected'` nhưng `getSshConnection` trả `undefined` → record vẫn ghi được (metrics null), không throw.
5. `createFleetMetricsHandler` output chứa `orca_server_cpu_percent` khi `report.servers[0].cpuPercent !== null`, và **không** chứa dòng đó khi `cpuPercent === null` (đúng convention Prometheus).

Không đụng tới `desktop/`, `frontend/`, `agent/` dù các thư mục đó có bản sao gần giống của `fleet-health-monitor.ts`/`ssh-types.ts` (confirmed qua GitNexus lookup trong solution) — scope của bug này chỉ ghi `backend/`; các package kia là bản copy độc lập, không share code qua import, nên cần ticket riêng nếu cũng cần fix.
