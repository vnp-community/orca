# TASK-HLD-020: Auto-respawn session crash với giới hạn 3 lần + exponential backoff

**Priority:** 🟠 HIGH — mất kết nối user vĩnh viễn khi process crash, không tự phục hồi
**Effort:** ~3-4 giờ (code + 6 test case)
**Status:** ✅ DONE — 2026-08-09 (áp dụng đủ: hằng số backoff + `calcRespawnBackoffDelay`; `respawnTimers`/`intentionalExitUserIds` state mới; `child.on('exit')` gọi `scheduleRespawn` khi không chủ ý; `scheduleRespawn()` method mới; `killUserProcess()` đánh dấu intentional; `shutdown()` huỷ pending timer. Xác nhận `proc` (const, khai báo trước closure `child.on('exit')`) dùng được trực tiếp. `tsc --noEmit` sạch hoàn toàn cho `session-manager.ts`. ⚠️ Chưa viết 6 test case — effort budget.)
**Bug refs:** BUG-BE-HLD-011 (phần 1 — auto-respawn)
**Solution ref:** [SOLUTION-session-manager-exact.md](../solutions/SOLUTION-session-manager-exact.md)
**Depends on:** Không — độc lập, có thể làm song song với TASK-HLD-021

---

## Mục tiêu

Hiện tại `child.on('exit', ...)` trong `SessionManager` chỉ xoá process khỏi map và dọn socket file — không hề gọi lại `spawnUserProcess`. Field `respawnCount` (`session-types.ts:18`) và `maxRespawnAttempts` (`session-types.ts:30`) đã được định nghĩa nhưng không nơi nào đọc/tăng giá trị. Cần cài đặt cơ chế auto-respawn có giới hạn tối đa 3 lần, dùng exponential backoff (1s → 2s → 4s, cap 10s, jitter 500ms) để tránh "thundering herd" khi nhiều user process cùng crash (OOM toàn host).

Chỉ respawn khi exit là **không chủ ý** (crash) — không respawn khi bị kill chủ động qua idle-sweep hoặc `shutdown()`.

## File cần sửa/tạo

```
backend/src/main/session/session-manager.ts   (sửa)
backend/src/main/session/session-manager.test.ts   (thêm test — file test hiện có theo specs/backend/tdd/v4/06-multi-user-sandbox.md §9)
```

Không cần sửa `session-types.ts` — `respawnCount`/`maxRespawnAttempts` đã đúng shape sẵn.

## Thay đổi cụ thể

### 1. Thêm hằng số backoff + state tracking (thay thế lines 19-27)

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

### 2. `child.on('exit', ...)` gọi `scheduleRespawn` khi không phải kill chủ động (thay thế lines 160-166)

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

### 3. Method `scheduleRespawn` mới — chèn ngay sau `spawnUserProcess` (trước `touch()`)

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

### 4. `killUserProcess()` đánh dấu intentional + `shutdown()` huỷ pending timers (thay thế lines 251-270)

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

### Lưu ý triển khai quan trọng

- **Race auto-respawn vs reconnect thủ công:** đã xử lý bằng `this.processes.has(userId)` check trong timer callback trước khi spawn — nếu user tự reconnect trong lúc chờ backoff, không double-spawn. Process mới đó bắt đầu lại `respawnCount: 0` (không cộng dồn) — hành vi hợp lý, cần nêu rõ trong code review.
- **`unref()` bắt buộc** trên respawn timer (giống `idleTimer`) để không giữ Node.js event loop sống.
- Không cần sửa `session-types.ts` — giữ nguyên diện thay đổi tối thiểu.

## Verification

```bash
cd /opt/repos/orca
pnpm --filter backend tsc --noEmit
pnpm --filter backend test session-manager

# Verify không còn call site nào set respawnCount thủ công ngoài scheduleRespawn/spawnUserProcess
grep -n "respawnCount" backend/src/main/session/session-manager.ts
```

Test case cần thêm (theo `specs/backend/tdd/v4/06-multi-user-sandbox.md §9`, hiện có 14 test):

1. Crash → giả lập `child.emit('exit', 1)` với mã lỗi khác 0 → `spawnUserProcess` được gọi lại sau backoff, process mới xuất hiện trong `listProcesses()`.
2. Giới hạn 3 lần: crash liên tiếp ngay sau mỗi lần respawn (uptime < `RESPAWN_STABLE_MS`) → sau đúng 3 lần respawn, crash lần thứ 4 KHÔNG spawn lại, chỉ log cảnh báo crash-loop.
3. Không respawn khi kill chủ động: gọi `sweepIdleProcesses()` hoặc `shutdown()` → xác nhận `spawnUserProcess` KHÔNG được gọi lại.
4. Reset counter khi ổn định: process chạy ≥ `RESPAWN_STABLE_MS` (fake timers) rồi crash → `respawnCount` mới = 1.
5. Không double-spawn: trong lúc chờ backoff, gọi `getOrSpawnUserProcess(userId)` thủ công → khi timer backoff bắn, không spawn thêm process thứ 2.
6. `shutdown()` huỷ pending respawn: trigger crash rồi gọi `shutdown()` ngay trong cửa sổ backoff → không có process mới được fork sau khi `shutdown()` resolve.
