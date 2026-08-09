# SOLUTION: BUG-BE-HLD-011 — Auto-respawn session khi crash (max 3) + cấu hình idle-timeout qua env var

**Source-verified:** ✅ Dựa trên source code thực tế
**Files nguồn đã đọc:** `session-manager.ts`, `session-types.ts`, `server-bootstrap.ts`, `backend/src/server/index.ts`, `specs/backend/tdd/v4/06-multi-user-sandbox.md`

---

## 1. Tóm tắt bug

Theo ticket [`BUG-BE-HLD-011`](../BUG-BE-HLD-011-session-manager-no-auto-respawn-no-idle-timeout-config.md), `docs/hld/backend-server-architecture.md §7` và `docs/features/F24-per-user-sandbox.md` liệt kê 2 tiêu chí đã hoàn thành nhưng thực tế **chưa cài đặt**:

1. **Auto-respawn khi process crash, tối đa 3 lần** — field `respawnCount` (`session-types.ts:18`) và `maxRespawnAttempts` (`session-types.ts:30`, dùng ở `session-manager.ts:32`) được định nghĩa nhưng **không có bất kỳ nơi nào đọc hay tăng giá trị này**. Handler `child.on('exit', ...)` (`session-manager.ts:161-166`) chỉ xoá process khỏi map và dọn socket file — không gọi lại `spawnUserProcess`. Process con chỉ được khởi động lại khi có kết nối WS mới của user đó (qua `getOrSpawnUserProcess`), không phải cơ chế auto-respawn có giới hạn số lần.

2. **Idle timeout cấu hình được qua `SESSION_IDLE_TIMEOUT_MS`** — grep toàn `backend/src/` cho chuỗi này cho **0 kết quả**. `SessionManager` được khởi tạo ở cả 2 nơi (`server-bootstrap.ts:311-316`, `backend/src/server/index.ts:137-142`) **không truyền `idleTimeoutMs`** — nên luôn dùng cứng `DEFAULT_IDLE_TIMEOUT_MS = 4h` (`session-manager.ts:19`), không có cách nào override qua env var.

**Hậu quả:** nếu 1 user process crash (OOM, uncaught exception...), user đó mất kết nối hoàn toàn cho tới khi tự reconnect thủ công thay vì được tự động respawn trong nền; vận hành production không thể tinh chỉnh idle-timeout mà không sửa code.

---

## 2. Vị trí code hiện tại có vấn đề (trích nguyên văn)

### 2.1. `child.on('exit', ...)` không respawn

**File:** `/opt/repos/orca/backend/src/main/session/session-manager.ts` — **Lines:** 149-166

```typescript
    const proc: UserProcess = {
      userId,
      pid:          child.pid!,
      socketPath,
      authToken:    rpcAuthToken,
      startedAt:    Date.now(),
      lastSeenAt:   Date.now(),
      process:      child,
      respawnCount: 0
    }

    // Auto-cleanup on exit: remove from map + delete socket file
    child.on('exit', (code) => {
      span.step('exit', { userId, code: code ?? -1 })
      console.warn(`[SessionManager] UserProcess exited: userId=${userId}, code=${code}`)
      this.processes.delete(userId)
      rm(socketPath, { force: true }).catch(() => {/* ignore cleanup errors */})
    })
```

`respawnCount: 0` được set một lần khi spawn nhưng không có nơi nào tăng nó lên, và không có `setTimeout(() => this.spawnUserProcess(userId), ...)` nào được gọi trong handler `exit`.

### 2.2. Hằng số idle-timeout hard-code, config field tồn tại nhưng không được đọc từ env

**File:** `/opt/repos/orca/backend/src/main/session/session-manager.ts` — **Lines:** 19-37

```typescript
const DEFAULT_IDLE_TIMEOUT_MS = 4 * 60 * 60 * 1000   // 4 hours
const DEFAULT_MAX_RESPAWN     = 3
const IDLE_CHECK_INTERVAL_MS  = 5 * 60 * 1000          // check every 5 minutes
const SPAWN_TIMEOUT_MS        = 30_000                  // 30s for fork to be ready

export class SessionManager {
  private readonly processes = new Map<string, UserProcess>()
  private idleTimer: ReturnType<typeof setInterval> | null = null
  private readonly config: Required<Omit<SessionManagerConfig, 'serverSecret'>> & { serverSecret?: string }

  constructor(config: SessionManagerConfig) {
    this.config = {
      idleTimeoutMs:      config.idleTimeoutMs      ?? DEFAULT_IDLE_TIMEOUT_MS,
      maxRespawnAttempts: config.maxRespawnAttempts  ?? DEFAULT_MAX_RESPAWN,
      baseDataPath:       config.baseDataPath,
      userProcessEntry:   config.userProcessEntry,
      serverSecret:       config.serverSecret,
      devServerManager:   config.devServerManager
    }
```

`config.idleTimeoutMs` đã hỗ trợ override qua constructor — vấn đề là **cả 2 call site không truyền giá trị này**, nên luôn rơi về default.

**File:** `/opt/repos/orca/backend/src/main/server-bootstrap.ts` — **Lines:** 306-318

```typescript
  if (process.env['ORCA_MULTI_USER'] === '1') {
    const { SessionManager } = await import('./session/session-manager')
    const userProcessEntry = pathJoin(
      platform.app.getAppPath(), 'out', 'main', 'user-process-entry.js'
    )
    sessionManager = new SessionManager({
      baseDataPath: userDataPath,
      userProcessEntry,
      serverSecret: process.env['ORCA_SERVER_SECRET'],
      devServerManager
    })
    console.log('[ServerBootstrap] ✅ SessionManager initialized (multi-user mode, serverSecret present:', !!process.env['ORCA_SERVER_SECRET'], ')')
  }
```

**File:** `/opt/repos/orca/backend/src/server/index.ts` — **Lines:** 127-142

```typescript
  if (multiUserMode) {
    const { SessionManager }   = await import('../main/session/session-manager')
    const { WsSessionRouter }  = await import('../main/session/ws-session-router')
    const { WebSocketServer }  = await import('ws')
    const { resolve: resolvePath } = await import('node:path')
    const { AGENT_WS_PATH }    = await import('../shared/agent-wire-protocol')

    const baseDataPath      = adapter.app.getPath('userData')
    const userProcessEntry  = resolvePath(__dirname, 'user-process-entry.js')

    const sessionManager = new SessionManager({ 
      baseDataPath, 
      userProcessEntry,
      serverSecret: process.env['ORCA_SERVER_SECRET'],
      devServerManager
    })
```

Không nơi nào đọc `process.env['SESSION_IDLE_TIMEOUT_MS']`.

### 2.3. Type liên quan

**File:** `/opt/repos/orca/backend/src/main/session/session-types.ts` — **Lines:** 9-41

```typescript
/** Represents a running user process (forked child) for a specific userId */
export type UserProcess = {
  userId:       string
  pid:          number
  socketPath:   string  // Unix domain socket path the user process listens on
  authToken:    string  // RPC auth token (from OrcaRuntimeRpcServer) for Unix socket
  startedAt:    number  // Unix ms
  lastSeenAt:   number  // Unix ms — updated on WS activity
  process:      ChildProcess
  respawnCount: number
}

/** Configuration for SessionManager */
export type SessionManagerConfig = {
  /** Base directory for all per-user data: /data/orca — users/<userId>/ will be created here */
  baseDataPath:        string
  /** Absolute path to the built user-process-entry.js (compiled entry point) */
  userProcessEntry:    string
  /** Milliseconds before an idle process is killed. Default: 4h */
  idleTimeoutMs?:      number
  /** Max times a crashed process will be respawned. Default: 3 */
  maxRespawnAttempts?: number
  /**
   * Master secret for WebCredentialStore (from ORCA_SERVER_SECRET env var).
   * When set, credential env vars are injected into each user child process
   * at spawn time so integration clients read from env without calling safeStorage.
   */
  serverSecret?:       string
  /**
   * Global DevServerManager to proxy dev server requests from user processes.
   */
  devServerManager: import('../dev-server/dev-server-manager').DevServerManager
}
```

`session-types.ts` **không cần thay đổi field** — `respawnCount` và `maxRespawnAttempts` đã tồn tại đúng shape, chỉ là chưa được dùng ở `session-manager.ts`.

---

## 3. Thiết kế giải pháp

### 3.1. Auto-respawn trong `child.on('exit', ...)`

**Nguyên tắc:**

- Chỉ respawn khi exit là **không chủ ý** (không phải do `killUserProcess()` gọi từ idle-sweep hoặc `shutdown()`). Hiện tại `killUserProcess()` xoá `proc` khỏi map *trước khi* tiến trình thực sự thoát, nên handler `exit` không có cách phân biệt "tôi vừa bị kill chủ động" với "tôi crash". Cần một cờ đánh dấu tường minh — dùng `Set<string> intentionalExitUserIds`, set trước khi gọi `.kill()` trong `killUserProcess()`, và handler `exit` kiểm tra + xoá cờ này trước khi quyết định respawn.
- **Backoff:** codebase đã có pattern exponential backoff đã được dùng cho reconnect (`backend/src/main/dev-server/dev-server-relay-bridge.ts:44-54`, hàm `calcBackoffDelay`: base 2s, cap 60s, jitter 1s, reset về 0 khi thành công). Áp dụng lại đúng pattern này nhưng với hằng số nhỏ hơn vì đây là respawn nội bộ chỉ tối đa 3 lần, người dùng đang chờ reconnect nên delay phải ngắn:
  - `RESPAWN_BACKOFF_BASE_MS = 1_000` (1s), `RESPAWN_BACKOFF_MAX_MS = 10_000` (10s cap), jitter `RESPAWN_BACKOFF_JITTER_MS = 500`ms để tránh nhiều user crash cùng lúc (OOM toàn host) gây "thundering herd" khi respawn đồng loạt.
  - Với 3 lần thử: delay xấp xỉ 1s → 2s → 4s (không chạm cap 10s ở lần thứ 3, nhưng cap vẫn giữ để an toàn nếu `maxRespawnAttempts` được config lớn hơn qua `SessionManagerConfig.maxRespawnAttempts`).
- **`maxRespawnAttempts` (default giữ nguyên `DEFAULT_MAX_RESPAWN = 3`, đã có sẵn ở `session-manager.ts:20`)** — dừng respawn khi `proc.respawnCount >= maxRespawnAttempts`, log cảnh báo rõ ràng để vận hành biết user đó cần can thiệp thủ công.
- **Reset counter khi ổn định:** nếu process đã chạy ổn định ≥ `RESPAWN_STABLE_MS = 60_000` (60 giây) trước khi crash lần này, coi đây là một sự cố **mới**, không cộng dồn vào `respawnCount` cũ — vì `spawnUserProcess()` luôn tạo `proc` mới với `respawnCount: 0`, ta chỉ cần gán lại `respawnCount = 1` (thay vì `oldRespawnCount + 1`) cho lần crash "mới" này. Ngưỡng 60s được chọn vì nó dài hơn nhiều so với toàn bộ chuỗi backoff (≤ vài giây) nên loại trừ được trường hợp "crash ngay sau khi respawn" (crash-loop thật) trong khi vẫn đủ ngắn để phát hiện sớm — không cần chờ hàng giờ như `IDLE_CHECK_INTERVAL_MS` (5 phút) mới có thể coi 1 process "đã ổn định".
- **Tránh double-spawn:** nếu trong lúc chờ backoff, có 1 WS connection mới của cùng user gọi `getOrSpawnUserProcess()` và tự spawn được một process khác, thì khi timer backoff bắn, phải kiểm tra `this.processes.has(userId)` trước khi spawn lại — nếu đã có, bỏ qua (không tạo process thứ 2 cho cùng 1 userId).
- **Huỷ respawn timer khi `shutdown()`:** thêm `respawnTimers: Map<string, Timeout>`, clear toàn bộ trong `shutdown()` để server không fork thêm process mới sau khi đã bắt đầu graceful shutdown.

### 3.2. Đọc `SESSION_IDLE_TIMEOUT_MS` ở cả 2 entry point

- Thêm hàm thuần **export** từ `session-manager.ts` (cùng module đang định nghĩa `idleTimeoutMs`, tránh tạo file `utils`/`helpers` mới theo quy tắc đặt tên trong AGENTS.md): `resolveIdleTimeoutMsFromEnv(env)`.
- Parse an toàn: `undefined`/chuỗi rỗng → trả `undefined` (constructor tự fallback `DEFAULT_IDLE_TIMEOUT_MS`, hành vi hiện tại được giữ nguyên 100% khi không set env); `NaN` hoặc `<= 0` → log cảnh báo + trả `undefined` (không throw, không phá vỡ server boot); số hợp lệ (`> 0`) → trả về số đó (ms).
- Gọi hàm này ở **cả `server-bootstrap.ts`** và **`backend/src/server/index.ts`**, truyền kết quả vào field `idleTimeoutMs` khi tạo `SessionManager`.

---

## 4. Code cụ thể

### 4.1. `session-manager.ts` — thêm hằng số backoff + hàm parse env

**File:** `/opt/repos/orca/backend/src/main/session/session-manager.ts` — **Lines: 19-22 (thay thế)**

```typescript
const DEFAULT_IDLE_TIMEOUT_MS   = 4 * 60 * 60 * 1000   // 4 hours
const DEFAULT_MAX_RESPAWN       = 3
const IDLE_CHECK_INTERVAL_MS    = 5 * 60 * 1000          // check every 5 minutes
const SPAWN_TIMEOUT_MS          = 30_000                  // 30s for fork to be ready

// FIX BUG-BE-HLD-011: bounded auto-respawn backoff for crashed user processes.
// Pattern mirrors calcBackoffDelay() in dev-server-relay-bridge.ts (base/cap/jitter),
// scaled down: only 3 attempts total and the user is actively waiting to reconnect.
const RESPAWN_BACKOFF_BASE_MS   = 1_000   // 1s
const RESPAWN_BACKOFF_MAX_MS    = 10_000  // 10s cap
const RESPAWN_BACKOFF_JITTER_MS = 500     // avoid thundering herd on host-wide crashes
// A process that stayed up this long before crashing is treated as a fresh
// failure, not a continuation of an earlier crash loop — this is well above
// the ≤ few-second span of the full backoff sequence above.
const RESPAWN_STABLE_MS         = 60_000  // 60s

// FIX BUG-BE-HLD-011: exponential backoff — 1s, 2s, 4s, 8s... capped at 10s, +jitter.
function calcRespawnBackoffDelay(attempt: number): number {
  const exponential = RESPAWN_BACKOFF_BASE_MS * Math.pow(2, attempt)
  const capped = Math.min(exponential, RESPAWN_BACKOFF_MAX_MS)
  return capped + Math.random() * RESPAWN_BACKOFF_JITTER_MS
}

/**
 * Parse SESSION_IDLE_TIMEOUT_MS from the environment. Returns undefined (so the
 * SessionManager constructor's DEFAULT_IDLE_TIMEOUT_MS fallback applies) when the
 * var is unset, blank, non-numeric, or <= 0 — an invalid value must never silently
 * disable idle cleanup.
 */
export function resolveIdleTimeoutMsFromEnv(env: NodeJS.ProcessEnv = process.env): number | undefined {
  const raw = env['SESSION_IDLE_TIMEOUT_MS']
  if (raw === undefined || raw.trim() === '') return undefined

  const parsed = Number(raw)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    console.warn(
      `[SessionManager] Ignoring invalid SESSION_IDLE_TIMEOUT_MS="${raw}" — must be a positive number (ms). Using default.`
    )
    return undefined
  }
  return parsed
}
```

### 4.2. `session-manager.ts` — thêm state cho respawn tracking

**File:** `/opt/repos/orca/backend/src/main/session/session-manager.ts` — **Lines: 24-27 (thay thế)**

```typescript
export class SessionManager {
  private readonly processes = new Map<string, UserProcess>()
  private idleTimer: ReturnType<typeof setInterval> | null = null
  private readonly config: Required<Omit<SessionManagerConfig, 'serverSecret'>> & { serverSecret?: string }
  // FIX BUG-BE-HLD-011: pending auto-respawn timers per userId, cancelled on shutdown().
  private readonly respawnTimers = new Map<string, ReturnType<typeof setTimeout>>()
  // FIX BUG-BE-HLD-011: userIds whose current exit was triggered intentionally
  // (idle sweep or shutdown) — the exit handler consumes this to skip respawn.
  private readonly intentionalExitUserIds = new Set<string>()
```

### 4.3. `session-manager.ts` — respawn trong `child.on('exit', ...)`

**File:** `/opt/repos/orca/backend/src/main/session/session-manager.ts` — **Lines: 160-166 (thay thế)**

```typescript
    // Auto-cleanup on exit: remove from map + delete socket file
    child.on('exit', (code) => {
      span.step('exit', { userId, code: code ?? -1 })
      console.warn(`[SessionManager] UserProcess exited: userId=${userId}, code=${code}`)
      this.processes.delete(userId)
      rm(socketPath, { force: true }).catch(() => {/* ignore cleanup errors */})

      // FIX BUG-BE-HLD-011: idle-sweep / shutdown kills are intentional — do not
      // auto-respawn those. Only unexpected crashes get a bounded auto-respawn.
      if (this.intentionalExitUserIds.delete(userId)) return
      this.scheduleRespawn(userId, proc)
    })
```

### 4.4. `session-manager.ts` — method `scheduleRespawn` (mới)

Thêm ngay sau method `spawnUserProcess` (trước `touch()`), giữ cùng style log/comment với phần còn lại của class.

**File:** `/opt/repos/orca/backend/src/main/session/session-manager.ts` — vị trí mới, chèn sau dòng 172 (`}` đóng `spawnUserProcess`)

```typescript
  /**
   * FIX BUG-BE-HLD-011: bounded auto-respawn for a user process that exited
   * unexpectedly (crash, OOM, uncaught exception) — up to config.maxRespawnAttempts
   * times, with short exponential backoff. Stops permanently past the limit to
   * avoid a crash loop; the user then needs a manual reconnect to re-spawn.
   */
  private scheduleRespawn(userId: string, proc: UserProcess): void {
    if (proc.respawnCount >= this.config.maxRespawnAttempts) {
      console.warn(
        `[SessionManager] UserProcess crash-loop detected: userId=${userId} ` +
        `respawnCount=${proc.respawnCount} >= maxRespawnAttempts=${this.config.maxRespawnAttempts}. ` +
        `Giving up auto-respawn — user must reconnect manually.`
      )
      return
    }

    // A crash after RESPAWN_STABLE_MS of healthy uptime is a fresh failure,
    // not a continuation of an earlier crash loop — restart the counter at 1.
    const uptimeMs = Date.now() - proc.startedAt
    const nextRespawnCount = uptimeMs >= RESPAWN_STABLE_MS ? 1 : proc.respawnCount + 1

    const delayMs = calcRespawnBackoffDelay(nextRespawnCount - 1)
    console.warn(
      `[SessionManager] Auto-respawning userId=${userId} in ${Math.round(delayMs)}ms ` +
      `(attempt ${nextRespawnCount}/${this.config.maxRespawnAttempts})`
    )

    const timer = setTimeout(() => {
      this.respawnTimers.delete(userId)
      // A new WS connection may have already respawned this user via
      // getOrSpawnUserProcess() while we were waiting — don't double-spawn.
      if (this.processes.has(userId)) return
      this.spawnUserProcess(userId)
        .then((respawned) => { respawned.respawnCount = nextRespawnCount })
        .catch((err) => {
          console.warn(`[SessionManager] Auto-respawn failed: userId=${userId}`, (err as Error)?.message)
        })
    }, delayMs)
    if (timer.unref) timer.unref()
    this.respawnTimers.set(userId, timer)
  }

```

### 4.5. `session-manager.ts` — đánh dấu intentional kill + huỷ timer khi shutdown

**File:** `/opt/repos/orca/backend/src/main/session/session-manager.ts` — **Lines: 251-270 (thay thế)**

```typescript
  private killUserProcess(userId: string): void {
    const proc = this.processes.get(userId)
    if (!proc) return
    // FIX BUG-BE-HLD-011: mark intentional so the exit handler skips auto-respawn.
    this.intentionalExitUserIds.add(userId)
    try { proc.process.kill('SIGTERM') } catch { /* already exited */ }
    this.processes.delete(userId)
    rm(proc.socketPath, { force: true }).catch(() => {/* ignore */})
  }

  /** Graceful shutdown: stop idle timer + SIGTERM all user processes */
  async shutdown(): Promise<void> {
    if (this.idleTimer) {
      clearInterval(this.idleTimer)
      this.idleTimer = null
    }
    // FIX BUG-BE-HLD-011: cancel pending auto-respawn timers so shutdown doesn't
    // fork new user processes after the server has started tearing down.
    for (const timer of this.respawnTimers.values()) clearTimeout(timer)
    this.respawnTimers.clear()

    const userIds = [...this.processes.keys()]
    console.log(`[SessionManager] Shutdown: killing ${userIds.length} user process(es)`)
    for (const userId of userIds) {
      this.killUserProcess(userId)
    }
  }
```

### 4.6. `session-types.ts` — không cần thay đổi

`UserProcess.respawnCount` và `SessionManagerConfig.maxRespawnAttempts` / `idleTimeoutMs` đã đúng shape (xem mục 2.3). Không có thay đổi nào cần thiết ở file này.

### 4.7. `server-bootstrap.ts` — đọc env var idle timeout

**File:** `/opt/repos/orca/backend/src/main/server-bootstrap.ts` — **Lines: 306-318 (thay thế)**

```typescript
  if (process.env['ORCA_MULTI_USER'] === '1') {
    const { SessionManager, resolveIdleTimeoutMsFromEnv } = await import('./session/session-manager')
    const userProcessEntry = pathJoin(
      platform.app.getAppPath(), 'out', 'main', 'user-process-entry.js'
    )
    sessionManager = new SessionManager({
      baseDataPath: userDataPath,
      userProcessEntry,
      serverSecret: process.env['ORCA_SERVER_SECRET'],
      // FIX BUG-BE-HLD-011: allow ops to override idle timeout without a code change.
      idleTimeoutMs: resolveIdleTimeoutMsFromEnv(process.env),
      devServerManager
    })
    console.log('[ServerBootstrap] ✅ SessionManager initialized (multi-user mode, serverSecret present:', !!process.env['ORCA_SERVER_SECRET'], ')')
  }
```

### 4.8. `backend/src/server/index.ts` — đọc env var idle timeout

**File:** `/opt/repos/orca/backend/src/server/index.ts` — **Lines: 128-142 (thay thế)**

```typescript
    const { SessionManager, resolveIdleTimeoutMsFromEnv } = await import('../main/session/session-manager')
    const { WsSessionRouter }  = await import('../main/session/ws-session-router')
    const { WebSocketServer }  = await import('ws')
    const { resolve: resolvePath } = await import('node:path')
    const { AGENT_WS_PATH }    = await import('../shared/agent-wire-protocol')

    const baseDataPath      = adapter.app.getPath('userData')
    const userProcessEntry  = resolvePath(__dirname, 'user-process-entry.js')

    const sessionManager = new SessionManager({ 
      baseDataPath, 
      userProcessEntry,
      serverSecret: process.env['ORCA_SERVER_SECRET'],
      // FIX BUG-BE-HLD-011: allow ops to override idle timeout without a code change.
      idleTimeoutMs: resolveIdleTimeoutMsFromEnv(process.env),
      devServerManager
    })
```

---

## 5. Tóm tắt thay đổi

| Vấn đề | File | Lines (hiện tại) | Thay đổi |
|---|---|---|---|
| Không auto-respawn | `session-manager.ts` | 19-22 | Thêm hằng số backoff + `RESPAWN_STABLE_MS` |
| Không auto-respawn | `session-manager.ts` | 24-27 | Thêm field `respawnTimers`, `intentionalExitUserIds` |
| Không auto-respawn | `session-manager.ts` | 161-166 | `child.on('exit')` gọi `scheduleRespawn()` nếu không phải kill chủ động |
| Không auto-respawn | `session-manager.ts` | mới, sau 172 | Method `scheduleRespawn()` |
| Không auto-respawn | `session-manager.ts` | 251-270 | `killUserProcess()` đánh dấu intentional; `shutdown()` huỷ `respawnTimers` |
| Idle timeout không đọc env | `session-manager.ts` | mới (cạnh hằng số) | Export `resolveIdleTimeoutMsFromEnv()` |
| Idle timeout không đọc env | `server-bootstrap.ts` | 306-318 | Truyền `idleTimeoutMs: resolveIdleTimeoutMsFromEnv(process.env)` |
| Idle timeout không đọc env | `backend/src/server/index.ts` | 128-142 | Truyền `idleTimeoutMs: resolveIdleTimeoutMsFromEnv(process.env)` |
| Type liên quan | `session-types.ts` | 9-41 | Không đổi — shape đã đúng |

---

## 6. Test cần bổ sung

Trong `session-manager.test.ts` (đã có 14 test theo `specs/backend/tdd/v4/06-multi-user-sandbox.md §9`, cần thêm ít nhất các case sau):

1. **Crash → auto-respawn:** giả lập `child.emit('exit', 1)` với mã lỗi khác 0 → `spawnUserProcess` được gọi lại sau backoff, process mới xuất hiện trong `listProcesses()`.
2. **Giới hạn 3 lần:** crash liên tiếp ngay sau mỗi lần respawn (uptime < `RESPAWN_STABLE_MS`) → sau đúng 3 lần respawn, crash lần thứ 4 KHÔNG spawn lại, chỉ log cảnh báo crash-loop.
3. **Không respawn khi kill chủ động:** gọi `sweepIdleProcesses()` (idle) hoặc `shutdown()` để kill process → xác nhận `spawnUserProcess` KHÔNG được gọi lại.
4. **Reset counter khi ổn định:** giả lập process chạy ≥ `RESPAWN_STABLE_MS` (mock `Date.now`/fake timers) rồi crash → `respawnCount` mới = 1 (không cộng dồn từ lần crash trước đó).
5. **Không double-spawn:** trong lúc chờ backoff, gọi `getOrSpawnUserProcess(userId)` thủ công (giả lập reconnect) → khi timer backoff bắn, không spawn thêm process thứ 2 cho cùng userId.
6. **`shutdown()` huỷ pending respawn:** trigger crash rồi gọi `shutdown()` ngay trong cửa sổ backoff → không có process mới được fork sau khi `shutdown()` resolve.
7. **`resolveIdleTimeoutMsFromEnv`:** unset → `undefined`; `"7200000"` → `7200000`; `"0"`, `"-100"`, `"abc"`, `"  "` → `undefined` + có log cảnh báo (spy `console.warn`).
8. **Constructor honor override:** tạo `SessionManager` với `idleTimeoutMs` từ `resolveIdleTimeoutMsFromEnv({ SESSION_IDLE_TIMEOUT_MS: '60000' })` → `sweepIdleProcesses()` kill process sau 60s idle thay vì 4h (dùng fake timers).

---

## 7. Rủi ro / lưu ý khi triển khai

- **Tương thích ngược:** khi `SESSION_IDLE_TIMEOUT_MS` không được set (trường hợp hiện tại của mọi deployment), `resolveIdleTimeoutMsFromEnv()` trả `undefined` và constructor fallback về `DEFAULT_IDLE_TIMEOUT_MS = 4h` y hệt hành vi cũ — không có breaking change cho các cấu hình đang chạy.
- **Session đang chạy không bị ảnh hưởng:** thay đổi chỉ tác động đến đường xử lý `exit` — các user process đang hoạt động bình thường (không crash) không bị restart hay gián đoạn.
- **Race giữa auto-respawn và reconnect thủ công:** đã xử lý bằng kiểm tra `this.processes.has(userId)` trước khi spawn trong timer callback — nhưng cần lưu ý logic này coi 1 lần reconnect "thành công xen giữa" là user đã tự phục hồi, nên KHÔNG cộng `respawnCount` cho process mới đó (nó bắt đầu lại từ 0 qua `spawnUserProcess`). Đây là hành vi hợp lý (không phạt oan user vì 1 crash cũ), nhưng cần nêu rõ trong code review vì nó có thể "reset" bộ đếm crash-loop sớm hơn dự kiến nếu user vô tình reconnect đúng lúc.
- **Thundering herd khi crash hàng loạt:** nếu nhiều user process cùng OOM do sự cố host-wide, mỗi userId có backoff/jitter độc lập (~1-4.5s cho 3 lần) — có thể vẫn gây spike fork đồng loạt trong vài giây. Chấp nhận được ở quy mô hiện tại; nếu số lượng user tăng cao, cân nhắc thêm global rate-limit cho tổng số respawn/giây (không nằm trong phạm vi fix này).
- **`unref()` trên respawn timer:** giống `idleTimer`, cần gọi `timer.unref()` để timer chờ backoff không giữ Node.js event loop sống, tránh chặn shutdown tự nhiên của process cha khi test hoặc khi server exit đột ngột (không qua `shutdown()`).
- **SSH use case (theo `AGENTS.md`):** `SessionManager` chạy hoàn toàn ở phía backend host (fork `user-process-entry.js` cục bộ qua Unix domain socket), không phụ thuộc transport của client (SSH-relay, WS trực tiếp, v.v.), nên auto-respawn/backoff không có nhánh xử lý riêng cho SSH — khi user process được respawn, `rpcAuthToken` và `socketPath` mới được cấp lại từ đầu (giống spawn lần đầu), client (qua `WsSessionRouter`) cần tự phát hiện mất kết nối và reconnect — không có gì đặc biệt cần thêm cho use case SSH ở lớp này.
- **Log noise có kiểm soát:** mỗi lần respawn ghi `console.warn`, giới hạn tối đa `maxRespawnAttempts` (default 3) lần/user trước khi dừng hẳn — không tạo vòng lặp log vô hạn.
- **Không đổi `session-types.ts`:** giữ nguyên type hiện có, giảm thiểu diện thay đổi và rủi ro breaking cho các nơi khác import các type này (`ws-session-router.ts`, `user-process-entry.ts`).
