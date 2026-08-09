# SOLUTION: fleet (hld-v1) — Code-Level Exact Fixes

**Source-verified:** ✅ Đọc trực tiếp source hiện tại (không copy lại solution cũ chưa áp dụng)
**Files nguồn đã đọc:** `fleet-health-monitor.ts`, `fleet-health-store.ts`, `fleet-status-service.ts`, `fleet-metrics-handler.ts`, `fleet-bootstrap-service.ts`, `fleet-remote-commands.ts`, `ssh-relay-deploy.ts`, `fleet-config-parser.ts`, `fleet-types.ts`, `ssh-types.ts`, `ssh-connection-store.ts`, `ssh-connection-manager.ts`, `server-bootstrap.ts`, `ipc/ssh.ts`, `cli/*.ts`, `vite.config.ts` (electron stub alias), `docs/features/F27-fleet-health-monitoring.md`, `docs/features/F31-fleet-provisioning.md`

**Bugs:** BUG-BE-HLD-010, BUG-BE-HLD-012, BUG-BE-HLD-013

---

## Vì sao `specs/backend/bugs/fleet/solutions/SOLUTION-fleet.md` (bản cũ) không được áp dụng

Đọc lại solution cũ xác nhận nguyên nhân cụ thể, để tránh lặp lại:

1. **Sai domain model hoàn toàn.** Solution cũ viết lại `FleetHealthMonitor` từ đầu dựa trên `ServerHealthMetrics`, `FleetHealthMonitorConfig`, và inject `DevServerManager` + `IDevServerRepository` + `EventBus` + `Logger` qua constructor. Code thật hiện tại: `FleetHealthMonitor` không có constructor injection — nó dùng **property-based DI** (`getSshTargets`, `getConnectionState`, `getWebhookUrl`, `onAlert` gán trực tiếp sau `new FleetHealthMonitor()`, xem `server-bootstrap.ts:498-529`). Không có `IDevServerRepository`, không có `EventBus`, không có `repository.updateHealthMetrics()`. Áp code cũ vào sẽ không compile.
2. **Gọi `bridge.call('system.health', ...)`** — không tồn tại `DevServerRelayBridge.call()` với method `'system.health'` trong codebase thật; không có JSON-RPC `health.get`/`system.health` nào được relay implement. Ticket BUG-BE-HLD-010 tự ghi "relay.call('health.get') **hoặc tương đương**" — solution dưới đây chọn tương đương thật sự khả thi: SSH `exec()` trực tiếp qua `SshConnection` đã có sẵn (cùng cơ chế `execCommand()` mà `fleet-remote-commands.ts` đang dùng cho `detectRemotePlatform()`), không cần sửa relay protocol.
3. **Tự tạo migration DB** (`0010_fleet_health_metrics.ts`) — không cần thiết, `FleetHealthStore` hiện tại là in-memory theo đúng thiết kế đã ghi trong header file (`// In-memory only — no disk I/O`); mở rộng field là đủ.
4. Vì vậy solution cũ về bản chất là **pseudocode kiến trúc khác**, không phải patch trên code thật — đó là lý do audit 2026-08-09 xác nhận bug vẫn broken dù status ghi "FIXED". Solution dưới đây sửa **trực tiếp trên các hàm/type đang tồn tại**, từng dòng khớp với file thật.

---

## BUG-BE-HLD-010 — FleetHealthMonitor không thu thập CPU/RAM/disk/latency thật

**Mức độ:** 🟡 MEDIUM
**Root cause:** `runHealthCheck()` chỉ đọc lại state đã cache qua callback `getConnectionState` — không có exec nào chạy trên remote host để đo resource; `pingLatencyMs` là dead field từ lúc khai báo.

### Thiết kế

Không thêm relay RPC mới (thay đổi lớn, cần sửa `relay.js` + wire protocol version — ngoài phạm vi 1 bug MEDIUM). Thay vào đó tái dùng cơ chế **SSH exec** đã có (`SshConnection.exec()` / `execCommand()` từ `ssh-relay-deploy-helpers.ts`, đúng pattern `detectRemotePlatform()` đang dùng trong `fleet-remote-commands.ts`):

- 1 exec round-trip đo `pingLatencyMs` (lệnh no-op `true`).
- 1 exec round-trip khác thu CPU/RAM/disk cùng lúc (gộp 3 phép đo vào 1 command string → không tốn thêm SSH channel).
- Toàn bộ best-effort: lỗi exec/parse không được throw ra ngoài — track theo đúng style try/catch hiện có trong `fleet-remote-commands.ts` (không phá health-check loop).

### File 1: `backend/src/main/ssh/fleet-remote-commands.ts` — thêm hàm thu thập resource metrics

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

### File 2: `backend/src/main/ssh/fleet-health-store.ts` — mở rộng `HealthRecord` + `recordConnectionState()`

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

### File 3: `backend/src/main/ssh/fleet-health-monitor.ts` — exec thật trong `runHealthCheck()`

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

### File 4: `backend/src/shared/fleet-types.ts` — thêm field vào `FleetServerStatus`

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

### File 5: `backend/src/main/ssh/fleet-status-service.ts` — surface metrics vào report

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

### File 6: `backend/src/main/runtime/rpc/fleet-metrics-handler.ts` — 4 metric Prometheus mới

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

### File 7: `backend/src/main/server-bootstrap.ts` — wire `getSshConnection`

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

### Verification

```bash
cd backend
pnpm vitest run src/main/ssh/fleet-remote-commands.test.ts   # NEW — test parseResourceMetricsOutput với input mẫu
pnpm vitest run src/main/ssh/fleet-health-store.test.ts       # NEW — test recordConnectionState ghi đúng cpu/ram/disk/pingLatencyMs
pnpm vitest run src/main/ssh/fleet-health-monitor.test.ts     # NEW — mock getSshConnection, assert execCommand được gọi khi connected, không gọi khi disconnected
```

Test case tối thiểu cần có (không tồn tại file test nào cho fleet-*.ts hiện tại — tạo mới):
1. `parseResourceMetricsOutput('RAM=42.3\nDISK=67\nCPU=12.5\n')` → `{ ramPercent: 42.3, diskPercent: 67, cpuPercent: 12.5 }`.
2. `parseResourceMetricsOutput('garbage')` → tất cả `null` (không throw).
3. `FleetHealthMonitor.checkOneTarget` với `getConnectionState` trả `'disconnected'` → không gọi `getSshConnection`/`execCommand`.
4. `FleetHealthMonitor.checkOneTarget` với `getConnectionState` trả `'connected'` nhưng `getSshConnection` trả `undefined` → record vẫn ghi được (metrics null), không throw.
5. `createFleetMetricsHandler` output chứa `orca_server_cpu_percent` khi `report.servers[0].cpuPercent !== null`, và **không** chứa dòng đó khi `cpuPercent === null` (đúng convention Prometheus).

---

## BUG-BE-HLD-012 — CLI `orca fleet provision` không tồn tại

**Mức độ:** 🟡 MEDIUM
**Root cause:** Không có subcommand nào wire `parseFleetConfig()`/`groupSshTargetsByProject()`/`bootstrapServer()` lại thành một CLI flow.

### Bối cảnh kỹ thuật quan trọng (đọc trực tiếp code, khác giả định trong ticket)

- `backend/src/main/cli/` hiện chỉ chứa installer/dispatcher script cho **cài đặt** lệnh `orca` vào PATH (`appimage-cli-wrapper.ts`, `cli-installer.ts`, `linux-bare-orca-dispatcher.ts`, `linux-terminal-orca-cli-shim.ts`) — không có gì xử lý **subcommand logic**.
- `package.json` khai `"bin": { "orca": "./out/cli/index.js" }` nhưng **không có file nguồn `src/cli/index.ts`** trong repo hiện tại — nghĩa là entry point CLI thật sự chưa được implement/không nằm trong nhánh này. Solution dưới đây viết module logic độc lập, export sẵn hàm `runFleetProvisionCli()` để dispatcher gắn vào — không đoán mò cấu trúc file dispatcher chưa tồn tại.
- **Quan trọng:** `ssh-remote-cli-host-passthrough.ts` xác nhận lệnh `orca` khi chạy thật sự **spawn lại chính app Electron ở chế độ `ELECTRON_RUN_AS_NODE=1`** (không phải một client RPC mỏng gọi vào server đang chạy). `vite.config.ts:84` alias `'electron' → src/platform/stubs/electron-node-wrapper.ts`, nên `ipcMain`/`app` vẫn hoạt động (stub) trong tiến trình CLI. → CLI có thể khởi tạo trực tiếp `Store` + `registerSshHandlers()` trong cùng tiến trình, y hệt cách `server-bootstrap.ts` làm, rồi gọi thẳng `bootstrapServer()` — không cần dựng thêm cơ chế IPC/RPC client-server mới.
- Không có `p-limit` trong `package.json` dependencies (đã kiểm tra) → tự viết concurrency limiter tối giản (không kéo thêm dependency cho một semaphore 15 dòng).
- `SshConnectionStore.importFromFleetConfig(filePath)` (đã có sẵn, `ssh-connection-store.ts:266`) đã làm đúng việc "parse YAML → upsert SshTarget" — tái dùng thẳng, không viết lại logic import.

### File mới: `backend/src/main/cli/fleet-provision-cli.ts`

```typescript
// src/main/cli/fleet-provision-cli.ts
// `orca fleet provision` — bulk-provision Dev Servers from orca-fleet.yaml.
// FIX BUG-BE-HLD-012: CR-003 (F31) had no CLI surface at all.
import { Store } from '../persistence'
import { registerSshHandlers, getSshConnectionStore } from '../ipc/ssh'
import { bootstrapServer } from '../ssh/fleet-bootstrap-service'
import { groupSshTargetsByProject } from '../../shared/ssh-types'
import type { SshTarget } from '../../shared/ssh-types'

export type FleetProvisionArgs = {
  configPath: string
  project?: string
  serverId?: string
  concurrency: number
  dryRun: boolean
}

const DEFAULT_CONCURRENCY = 3
const DEFAULT_FLEET_CONFIG_PATH = 'deploy/dev/orca-fleet.yaml'

export function parseFleetProvisionArgs(argv: string[]): FleetProvisionArgs {
  let configPath = DEFAULT_FLEET_CONFIG_PATH
  let project: string | undefined
  let serverId: string | undefined
  let concurrency = DEFAULT_CONCURRENCY
  let dryRun = false

  for (let i = 0; i < argv.length; i++) {
    switch (argv[i]) {
      case '--config':
        configPath = argv[++i] ?? configPath
        break
      case '--project':
        project = argv[++i]
        break
      case '--server':
        serverId = argv[++i]
        break
      case '--concurrency': {
        const parsed = Number(argv[++i])
        if (Number.isFinite(parsed) && parsed > 0) concurrency = Math.floor(parsed)
        break
      }
      case '--dry-run':
        dryRun = true
        break
    }
  }

  return { configPath, project, serverId, concurrency, dryRun }
}

// Why: no p-limit dependency in package.json — a bounded-concurrency runner
// is ~15 lines, not worth pulling in a package for.
function createConcurrencyLimiter(max: number) {
  let active = 0
  const queue: Array<() => void> = []
  return function limit<T>(fn: () => Promise<T>): Promise<T> {
    return new Promise((resolve, reject) => {
      const run = (): void => {
        active++
        fn()
          .then(resolve, reject)
          .finally(() => {
            active--
            const next = queue.shift()
            if (next) next()
          })
      }
      if (active < max) run()
      else queue.push(run)
    })
  }
}

/**
 * Entry point for `orca fleet provision`. Returns a process exit code
 * (0 = all servers provisioned/planned OK, 1 = at least one failure).
 */
export async function runFleetProvisionCli(argv: string[]): Promise<number> {
  const args = parseFleetProvisionArgs(argv)

  // Why: CLI runs as its own ELECTRON_RUN_AS_NODE process (see
  // ssh-remote-cli-host-passthrough.ts) — same pattern server-bootstrap.ts
  // uses to bring up the SSH subsystem, minus the renderer-facing IPC wiring
  // that doesn't apply headlessly (getMainWindow → () => null is safe: every
  // caller in ssh.ts null-checks the window before use).
  const store = new Store()
  registerSshHandlers(store, () => null)

  const sshStore = getSshConnectionStore()
  if (!sshStore) {
    console.error('[fleet provision] Failed to initialize SSH store.')
    return 1
  }

  console.log(`[fleet provision] Importing fleet config: ${args.configPath}`)
  const importResult = await sshStore.importFromFleetConfig(args.configPath)
  const failedImports = importResult.servers.filter((s) => s.action === 'skipped')
  for (const failed of failedImports) {
    console.warn(`[fleet provision] Skipped ${failed.fleetId}: ${failed.error}`)
  }

  let targets: SshTarget[] = sshStore
    .listTargets()
    .filter((t) => t.fleetConfigSource === args.configPath)
  if (args.project) targets = targets.filter((t) => t.project === args.project)
  if (args.serverId) targets = targets.filter((t) => t.fleetId === args.serverId)

  if (targets.length === 0) {
    console.log('[fleet provision] No matching servers to provision.')
    return 0
  }

  const groups = groupSshTargetsByProject(targets)
  console.log(`[fleet provision] Plan (${targets.length} server(s)):`)
  for (const group of groups) {
    console.log(`  Group: ${group.label} (${group.targets.length} server${group.targets.length === 1 ? '' : 's'})`)
    for (const t of group.targets) {
      console.log(`    - ${t.fleetId ?? t.id} (${t.label}) @ ${t.host}`)
    }
  }

  if (args.dryRun) {
    console.log('[fleet provision] --dry-run: no servers were touched.')
    return 0
  }

  const limit = createConcurrencyLimiter(args.concurrency)
  console.log(`[fleet provision] Provisioning with concurrency=${args.concurrency}...`)

  const results = await Promise.all(
    targets.map((target) =>
      limit(async () => {
        try {
          const { connectRegisteredSshTarget } = await import('../ipc/ssh')
          const state = await connectRegisteredSshTarget(target.id)
          if (state.status !== 'connected') {
            console.error(`❌ ${target.label}: cannot connect (status: ${state.status})`)
            return { targetId: target.id, ok: false }
          }
          const result = await bootstrapServer(target.id, { fleetConfigPath: args.configPath })
          if (result.success) {
            const stepSummary = result.steps
              .filter((s) => s.status === 'ok')
              .map((s) => s.step)
              .join(', ')
            console.log(`✅ ${target.label}: ${stepSummary}`)
          } else {
            console.error(`❌ ${target.label}: ${result.error}`)
          }
          return { targetId: target.id, ok: result.success }
        } catch (err) {
          console.error(`❌ ${target.label}: ${err instanceof Error ? err.message : String(err)}`)
          return { targetId: target.id, ok: false }
        }
      })
    )
  )

  const failed = results.filter((r) => !r.ok)
  console.log(`[fleet provision] Done: ${results.length - failed.length}/${results.length} succeeded.`)
  return failed.length > 0 ? 1 : 0
}
```

### Wiring vào CLI dispatcher

File `src/cli/index.ts` (build target của `bin.orca`) không tồn tại trong nhánh source hiện tại — không đoán cấu trúc file đó. Bất kỳ đâu argv top-level được parse (`process.argv.slice(2)`, theo đúng pattern `parseArgs()` trong `daemon-entry.ts:22-44` mà codebase đã dùng cho các entry khác), thêm nhánh:

```typescript
if (argv[0] === 'fleet' && argv[1] === 'provision') {
  const { runFleetProvisionCli } = await import('./main/cli/fleet-provision-cli')
  process.exitCode = await runFleetProvisionCli(argv.slice(2))
  return
}
```

### Verification

```bash
pnpm vitest run src/main/cli/fleet-provision-cli.test.ts   # NEW
```

Test tối thiểu:
1. `parseFleetProvisionArgs(['--project', 'vnp-blc', '--concurrency', '5', '--dry-run'])` → `{ project: 'vnp-blc', concurrency: 5, dryRun: true, configPath: DEFAULT_FLEET_CONFIG_PATH }`.
2. `--concurrency abc` (không phải số) → fallback về `DEFAULT_CONCURRENCY`, không throw.
3. `createConcurrencyLimiter(2)` với 5 task async (mock `setTimeout`) → tối đa 2 task chạy đồng thời tại mọi thời điểm (assert qua counter).
4. `runFleetProvisionCli(['--dry-run', '--config', fixturePath])` với fixture YAML 2 servers → in đúng plan, **không** gọi `connectRegisteredSshTarget`/`bootstrapServer` (mock + assert `not.toHaveBeenCalled()`), return `0`.
5. `runFleetProvisionCli([...])` không `--dry-run`, mock `bootstrapServer` trả về 1 success + 1 failure → return `1`.

---

## BUG-BE-HLD-013 — `bootstrapServer()` thiếu disk-check và SHA256 verify

**Mức độ:** 🟡 MEDIUM
**Root cause:** `bootstrapServer()` (7-step flow theo CR-004) thiếu 2 bước: disk-space check trước khi làm gì nặng, và SHA256 verify sau khi SFTP upload relay binary. Bước install/start relay cũng tách rời hoàn toàn khỏi `bootstrapServer()` (`ssh-relay-deploy.ts` là luồng riêng, chỉ được gọi từ chỗ connect SSH target, không từ bootstrap).

### File 1: `backend/src/main/ssh/fleet-remote-commands.ts` — thêm `checkRemoteDiskSpace()`

Thêm vào cuối file (cùng khu vực với `collectRemoteResourceMetrics` ở BUG-010, hoặc ngay sau `installPackages`):

```typescript
// ── Disk space check ────────────────────────────────────────────

export type DiskSpaceCheck = {
  availableGb: number
  ok: boolean
}

export const MIN_BOOTSTRAP_DISK_SPACE_GB = 5

/**
 * Check free disk space in the current remote working directory (typically
 * $HOME for a fresh SSH session) via `df -h .`, per CR-004's disk-check step.
 */
export async function checkRemoteDiskSpace(
  conn: SshConnection,
  minGb: number = MIN_BOOTSTRAP_DISK_SPACE_GB
): Promise<DiskSpaceCheck> {
  const output = await execCommand(conn, 'df -h .')
  const dataLine = output.trim().split('\n')[1] ?? ''
  const columns = dataLine.trim().split(/\s+/)
  // df -h columns: Filesystem Size Used Avail Use% Mounted-on
  const availableRaw = columns[3] ?? '0'
  const availableGb = parseDfSizeToGb(availableRaw)
  return { availableGb, ok: availableGb >= minGb }
}

/** Parses a `df -h`-style size string ("47G", "512M", "1.2T", "900K") into GB. */
export function parseDfSizeToGb(raw: string): number {
  const match = /^([\d.]+)\s*([KMGT]?)/i.exec(raw.trim())
  if (!match) return 0
  const value = parseFloat(match[1])
  if (Number.isNaN(value)) return 0
  switch (match[2].toUpperCase()) {
    case 'T':
      return value * 1024
    case 'G':
      return value
    case 'M':
      return value / 1024
    case 'K':
      return value / (1024 * 1024)
    default:
      // No suffix — df reported raw bytes (rare with -h, but be safe).
      return value / (1024 * 1024 * 1024)
  }
}
```

### File 2: `backend/src/shared/fleet-types.ts` — thêm 2 step mới vào `BootstrapStepName`

```typescript
export type BootstrapStepName =
  | 'node-check'
  | 'node-install'
  | 'git-check'
  | 'disk-check'    // FIX BUG-BE-HLD-013
  | 'packages'
  | 'relay-deploy'  // FIX BUG-BE-HLD-013 — install + SHA256 verify + start, gộp từ ssh-relay-deploy.ts
  | 'repo-clone'
  | 'setup-script'
  | 'verify'
```

### File 3: `backend/src/main/ssh/fleet-bootstrap-service.ts` — thêm 2 bước vào `bootstrapServer()`

**Thêm import (đầu file, sau import `fleet-remote-commands`):**
```typescript
import {
  installNodeJs,
  ensureGitInstalled,
  cloneOrUpdateRepo,
  installPackages,
  runRemoteScript,
  checkRemoteDiskSpace,       // FIX BUG-BE-HLD-013
  MIN_BOOTSTRAP_DISK_SPACE_GB, // FIX BUG-BE-HLD-013
} from './fleet-remote-commands'
import { deployAndLaunchRelay } from './ssh-relay-deploy' // FIX BUG-BE-HLD-013
```

**Mở rộng `BootstrapOptions` (dòng 23–38):**
```typescript
export type BootstrapOptions = {
  fleetConfigPath?: string
  skipNodeInstall?: boolean
  skipGitInstall?: boolean
  /** FIX BUG-BE-HLD-013 — skip the ≥5GB free-space check. */
  skipDiskCheck?: boolean
  /** FIX BUG-BE-HLD-013 — minimum free disk space required, in GB. Default 5. */
  minDiskSpaceGb?: number
  /** FIX BUG-BE-HLD-013 — skip installing/verifying/starting the relay binary. */
  skipRelayDeploy?: boolean
  skipRepoClone?: boolean
  skipSetupScript?: boolean
  nodeVersion?: string
  onProgress?: (step: BootstrapStep) => void
}
```

**Chèn Step 2.5 (disk-check) ngay sau Step 2 "Git check & install" (sau dòng 135, trước "── Step 3: OS packages"):**
```typescript
    // ── Step 2.5: Disk space check ──────────────────────────────
    // FIX BUG-BE-HLD-013: fail fast before any install/clone work — a
    // server that's nearly out of disk should never reach npm install or
    // git clone, where it fails halfway through with a confusing error.
    if (!options.skipDiskCheck) {
      notify({ step: 'disk-check', status: 'running' })
      try {
        const minGb = options.minDiskSpaceGb ?? MIN_BOOTSTRAP_DISK_SPACE_GB
        const disk = await checkRemoteDiskSpace(conn, minGb)
        if (!disk.ok) {
          const msg = `Insufficient disk space: ${disk.availableGb.toFixed(1)}GB available, need >= ${minGb}GB`
          notify({ step: 'disk-check', status: 'error', error: msg })
          throw new Error(msg)
        }
        notify({ step: 'disk-check', status: 'ok', message: `${disk.availableGb.toFixed(1)}GB available` })
      } catch (err) {
        notify({ step: 'disk-check', status: 'error', error: String(err) })
        throw err
      }
    } else {
      notify({ step: 'disk-check', status: 'skipped' })
    }
```

**Chèn Step 3.5 (relay-deploy) ngay sau Step 3 "OS packages" (sau dòng 150, trước "── Step 4: Clone / update repos"):**
```typescript
    // ── Step 3.5: Relay deploy (install + SHA256 verify + start) ──
    // FIX BUG-BE-HLD-013: was a disconnected flow (ssh-relay-deploy.ts only
    // ran on-demand at connect time), never part of bootstrap — two flows
    // that could desync (bootstrap "succeeds" but relay never verified/started).
    if (!options.skipRelayDeploy) {
      notify({ step: 'relay-deploy', status: 'running' })
      try {
        await deployAndLaunchRelay(conn, (status) => {
          notify({ step: 'relay-deploy', status: 'running', message: status })
        })
        notify({ step: 'relay-deploy', status: 'ok', message: 'Relay installed, SHA256-verified, started' })
      } catch (err) {
        notify({ step: 'relay-deploy', status: 'error', error: String(err) })
        throw err
      }
    } else {
      notify({ step: 'relay-deploy', status: 'skipped' })
    }
```

> `deployAndLaunchRelay` đã idempotent (kiểm tra `isRelayAlreadyInstalled` trước khi upload lại) — gọi nó trong `bootstrapServer()` không double-install nếu relay đã chạy từ trước lúc connect.

### File 4: `backend/src/main/ssh/ssh-relay-deploy.ts` — SHA256 verify sau SFTP upload

**Thêm import ở đầu file (cùng nhóm `node:fs`):**
```typescript
import { existsSync, readFileSync } from 'node:fs'
import { createHash } from 'node:crypto'
```

**Sửa `uploadRelay()` (dòng 350–388) — verify SHA256 của `relay.js` ngay sau khi upload directory, trước khi ghi `.version`:**

Code hiện tại:
```typescript
  // Create remote directory
  await execHostCommand(conn, hostPlatform, makeRemoteDirectoryCommand(hostPlatform, remoteDir))

  await uploadDirectoryForConnection(conn, localRelayDir, remoteDir, hostPlatform)

  // Make the node binary executable
  if (!isWindowsRemoteHost(hostPlatform)) {
    await execHostCommand(
      conn,
      hostPlatform,
      makeRemoteExecutableCommand(hostPlatform, joinRemotePath(hostPlatform, remoteDir, 'node'))
    )
  }

  // Why: write `.version` via SFTP rather than shell to avoid quoting issues
  // with content-hashed version strings. The remote daemon reads this same
  // file on startup so the wire-handshake validates against it.
  await writeRemoteFile(
    conn,
    hostPlatform,
    joinRemotePath(hostPlatform, remoteDir, '.version'),
    fullVersion
  )
}
```

**Fix — thêm bước verify ngay sau `uploadDirectoryForConnection`:**
```typescript
  // Create remote directory
  await execHostCommand(conn, hostPlatform, makeRemoteDirectoryCommand(hostPlatform, remoteDir))

  await uploadDirectoryForConnection(conn, localRelayDir, remoteDir, hostPlatform)

  // FIX BUG-BE-HLD-013: verify the transferred relay entry point matches its
  // local SHA256 before trusting/executing it — catches SFTP corruption or
  // in-transit tampering. relay.js is the trust boundary: it's the file
  // that actually gets `node relay.js`-executed on the remote host.
  await verifyRelayChecksum(conn, hostPlatform, localRelayDir, remoteDir, nodePathForChecksum(hostPlatform))

  // Make the node binary executable
  if (!isWindowsRemoteHost(hostPlatform)) {
    await execHostCommand(
      conn,
      hostPlatform,
      makeRemoteExecutableCommand(hostPlatform, joinRemotePath(hostPlatform, remoteDir, 'node'))
    )
  }

  // Why: write `.version` via SFTP rather than shell to avoid quoting issues
  // with content-hashed version strings. The remote daemon reads this same
  // file on startup so the wire-handshake validates against it.
  await writeRemoteFile(
    conn,
    hostPlatform,
    joinRemotePath(hostPlatform, remoteDir, '.version'),
    fullVersion
  )
}

// FIX BUG-BE-HLD-013: local SHA256 vs remote SHA256 of relay.js, computed
// remotely via `node -e` (node is guaranteed present — it's what we just
// uploaded) rather than depending on the `sha256sum` binary existing on
// every distro (not guaranteed on minimal Alpine images).
async function verifyRelayChecksum(
  conn: SshConnection,
  hostPlatform: RemoteHostPlatform,
  localRelayDir: string,
  remoteDir: string,
  nodePath: string
): Promise<void> {
  const localEntryPath = join(localRelayDir, 'relay.js')
  const localHash = createHash('sha256').update(readFileSync(localEntryPath)).digest('hex')

  const remoteEntryPath = joinRemotePath(hostPlatform, remoteDir, 'relay.js')
  const hashScript =
    'const c=require("crypto"),fs=require("fs");' +
    'process.stdout.write(c.createHash("sha256").update(fs.readFileSync(process.argv[1])).digest("hex"))'

  const command = isWindowsRemoteHost(hostPlatform)
    ? commandWithNodePath(
        hostPlatform,
        nodePath,
        remoteDir,
        `& ${powerShellLiteral(nodePath)} -e ${powerShellNativeArg(hashScript)} ${powerShellLiteral(remoteEntryPath)}`
      )
    : commandWithNodePath(
        hostPlatform,
        nodePath,
        remoteDir,
        `${shellEscape(nodePath)} -e '${hashScript}' ${shellEscape(remoteEntryPath)}`
      )

  const remoteHash = (await execHostCommand(conn, hostPlatform, command)).trim()

  if (remoteHash !== localHash) {
    throw new Error(
      `Relay binary checksum mismatch after upload: local sha256=${localHash} remote sha256=${remoteHash}. ` +
        `The transferred relay.js may be corrupted or tampered with in transit — aborting deploy.`
    )
  }
}
```

**Về `nodePath` cho việc hash:** tại điểm gọi `uploadRelay()` (trong `deployAndLaunchRelayInner`, dòng 295), biến `nodePath` (từ `resolveRelayBootstrapState`) đã có sẵn trong scope — sửa call site thay vì thêm hàm `nodePathForChecksum` giả:

```typescript
// deployAndLaunchRelayInner — chỗ gọi uploadRelay hiện tại (dòng 295):
await uploadRelay(conn, platform, remoteRelayDir, fullVersion, hostPlatform, nodePath)
```

Và cập nhật signature `uploadRelay` để nhận thêm `nodePath: string`, dùng trực tiếp thay vì hàm `nodePathForChecksum` placeholder ở trên (xoá dòng `nodePathForChecksum(hostPlatform)`, gọi `verifyRelayChecksum(conn, hostPlatform, localRelayDir, remoteDir, nodePath)` với `nodePath` là tham số mới của `uploadRelay`):

```typescript
async function uploadRelay(
  conn: SshConnection,
  platform: RelayPlatform,
  remoteDir: string,
  fullVersion: string,
  hostPlatform: RemoteHostPlatform,
  nodePath: string // FIX BUG-BE-HLD-013 — new param, needed for checksum verify
): Promise<void> {
```

Và tại `repairInstalledNativeDeps` / mọi call site khác của `uploadRelay` — chỉ có 1 call site (`deployAndLaunchRelayInner`), nên đổi signature không phá caller nào khác (đã grep xác nhận — search `uploadRelay(` trong file chỉ khớp định nghĩa + 1 lần gọi).

### Verification

```bash
pnpm vitest run src/main/ssh/fleet-remote-commands.test.ts   # parseDfSizeToGb + checkRemoteDiskSpace
pnpm vitest run src/main/ssh/fleet-bootstrap-service.test.ts # disk-check step order + failure short-circuits
pnpm vitest run src/main/ssh/ssh-relay-deploy.test.ts        # checksum mismatch throws before finalizeInstall
```

Test tối thiểu:
1. `parseDfSizeToGb('47G')` → `47`; `parseDfSizeToGb('512M')` → `0.5`; `parseDfSizeToGb('1.2T')` → `1228.8`.
2. `checkRemoteDiskSpace(conn, 5)` với mock `execCommand` trả `df -h` output có `Avail=3.2G` → `{ availableGb: 3.2, ok: false }`.
3. `bootstrapServer(...)` với disk-check mock trả `ok: false` → steps chứa `{ step: 'disk-check', status: 'error' }`, **không** chạy tiếp `packages`/`relay-deploy`/`repo-clone` (assert các mock đó `not.toHaveBeenCalled()`), `success: false`.
4. `verifyRelayChecksum` với remote hash mock khác local hash → throw đúng message chứa `"checksum mismatch"`, và `finalizeInstall` **không** được gọi (dòng `.install-complete` không ghi cho install lỗi — đúng invariant đã có sẵn trong `deployAndLaunchRelayInner`'s catch/`abandonInstall` path).
5. Test tích hợp nhẹ: `bootstrapServer()` full happy-path với tất cả bước mock `ok` → `steps` chứa đúng thứ tự `node-check, git-check, disk-check, packages, relay-deploy, repo-clone, setup-script, verify`.

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `backend/src/main/ssh/fleet-remote-commands.ts` | + `collectRemoteResourceMetrics`, `parseResourceMetricsOutput` | BUG-BE-HLD-010 |
| `backend/src/main/ssh/fleet-remote-commands.ts` | + `checkRemoteDiskSpace`, `parseDfSizeToGb`, `MIN_BOOTSTRAP_DISK_SPACE_GB` | BUG-BE-HLD-013 |
| `backend/src/main/ssh/fleet-health-store.ts` | `HealthRecord` +cpu/ram/disk; `recordConnectionState()` +metrics param | BUG-BE-HLD-010 |
| `backend/src/main/ssh/fleet-health-monitor.ts` | `runHealthCheck()` → parallel + exec thật; `DEFAULT_PING_INTERVAL_MS` 60s→30s; + `getSshConnection` DI | BUG-BE-HLD-010 |
| `backend/src/shared/fleet-types.ts` | `FleetServerStatus` +4 field; `BootstrapStepName` +`disk-check`/`relay-deploy` | BUG-BE-HLD-010, 013 |
| `backend/src/main/ssh/fleet-status-service.ts` | `getFleetStatus()` surface metrics mới | BUG-BE-HLD-010 |
| `backend/src/main/runtime/rpc/fleet-metrics-handler.ts` | + 4 gauge Prometheus mới | BUG-BE-HLD-010 |
| `backend/src/main/server-bootstrap.ts` | wire `getSshConnection`; interval default 30s | BUG-BE-HLD-010 |
| `backend/src/main/cli/fleet-provision-cli.ts` | **NEW** — `orca fleet provision` full impl | BUG-BE-HLD-012 |
| `backend/src/main/ssh/fleet-bootstrap-service.ts` | + Step 2.5 disk-check, Step 3.5 relay-deploy; `BootstrapOptions` +3 field | BUG-BE-HLD-013 |
| `backend/src/main/ssh/ssh-relay-deploy.ts` | `uploadRelay()` +`nodePath` param + `verifyRelayChecksum()` | BUG-BE-HLD-013 |

## Ghi chú cho người triển khai

- **BUG-BE-HLD-012**: entry point `src/cli/index.ts` (build target của `bin.orca` trong `package.json`) không có trong nhánh source đã đọc — người triển khai cần xác định vị trí thật của argv-dispatch hiện tại (hoặc tạo mới nếu chưa tồn tại) trước khi gắn nhánh `fleet provision`. Hàm `runFleetProvisionCli()` đã sẵn sàng dùng độc lập với việc đó.
- **BUG-BE-HLD-010**: `DEFAULT_PING_INTERVAL_MS` đổi 60s→30s làm tăng gấp đôi tần suất exec trên mỗi server có kết nối — với script CPU sampling tốn ~1s/lần, cân nhắc tải SSH server-side nếu fleet lớn (>50 servers cùng lúc). Không cần đổi nếu team quyết định giữ 60s — chỉ cần sửa lại default, phần còn lại của fix (exec thật, field mới, metric mới) độc lập với con số này.
- **BUG-BE-HLD-013**: `verifyRelayChecksum` chỉ hash `relay.js` (entry point thực thi), không hash toàn bộ thư mục package (bao gồm `node_modules` sau `installNativeDeps`). Đây là trust boundary quan trọng nhất (file được `node relay.js` chạy trực tiếp) nhưng không phải toàn bộ supply-chain surface — mở rộng thành manifest hash cho mọi file trong `localRelayDir` là bước tiếp theo hợp lý nếu cần bảo vệ chặt hơn (không nằm trong scope bug MEDIUM này).
- Cả 3 bug **không đụng tới `desktop/`, `frontend/`, `agent/`** dù các thư mục đó có bản sao gần giống của `fleet-health-monitor.ts`/`ssh-types.ts` (xác nhận qua GitNexus lookup) — ticket phạm vi rõ ràng ghi `backend/`; nếu các package kia cũng cần fix tương tự, cần ticket riêng vì chúng là các bản copy độc lập, không share code qua import.
