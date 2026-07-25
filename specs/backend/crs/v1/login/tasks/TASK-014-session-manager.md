# TASK-014: Tạo `src/main/session/session-manager.ts`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 2 — User Sandbox
**Solution:** [SOL-LG-002](../solutions/SOL-LG-002-user-sandbox.md) §4.2
**Depends on:** TASK-013
**Blocks:** TASK-015 (test), TASK-016 (ws-router)

---

## Mục tiêu

Tạo `SessionManager` — spawn/track/lifecycle user processes (per-user fork).

---

## File cần tạo

**Path:** `src/main/session/session-manager.ts`

---

## Nội dung (implement từ SOL-LG-002 §4.2)

```typescript
// src/main/session/session-manager.ts
import { fork } from 'node:child_process'
import { mkdir, rm } from 'node:fs/promises'
import { join } from 'node:path'
import type { UserProcess, SessionManagerConfig } from './session-types'

const DEFAULT_IDLE_TIMEOUT_MS = 4 * 60 * 60 * 1000   // 4h
const DEFAULT_MAX_RESPAWN     = 3
const IDLE_CHECK_INTERVAL_MS  = 5 * 60 * 1000          // check mỗi 5 phút
const SPAWN_TIMEOUT_MS        = 30_000                  // 30s để fork ready

export class SessionManager {
  private processes  = new Map<string, UserProcess>()
  private idleTimer: ReturnType<typeof setInterval> | null = null
  private readonly config: Required<SessionManagerConfig>

  constructor(config: SessionManagerConfig) {
    this.config = {
      idleTimeoutMs:      config.idleTimeoutMs      ?? DEFAULT_IDLE_TIMEOUT_MS,
      maxRespawnAttempts: config.maxRespawnAttempts  ?? DEFAULT_MAX_RESPAWN,
      baseDataPath:       config.baseDataPath,
      userProcessEntry:   config.userProcessEntry
    }
    this.idleTimer = setInterval(() => this.sweepIdleProcesses(), IDLE_CHECK_INTERVAL_MS)
    if (this.idleTimer.unref) this.idleTimer.unref()
  }

  async getOrSpawnUserProcess(userId: string): Promise<UserProcess> {
    const existing = this.processes.get(userId)
    if (existing) {
      existing.lastSeenAt = Date.now()
      return existing
    }
    return this.spawnUserProcess(userId)
  }

  private async spawnUserProcess(userId: string): Promise<UserProcess> {
    const userDataPath = join(this.config.baseDataPath, 'users', userId)
    const socketPath   = join(userDataPath, 'orca.sock')

    await mkdir(userDataPath, { recursive: true, mode: 0o700 })

    const child = fork(this.config.userProcessEntry, [], {
      env: {
        ...process.env,
        ORCA_USER_DATA_PATH: userDataPath,
        ORCA_USER_ID:        userId,
        ORCA_SOCKET_PATH:    socketPath,
        NODE_OPTIONS:        '--max-old-space-size=512'
      },
      stdio: ['ignore', 'pipe', 'pipe', 'ipc'],
      detached: false
    })

    // Pipe stdout/stderr với userId prefix
    child.stdout?.on('data', (d: Buffer) => process.stdout.write(`[user:${userId}] ${d}`))
    child.stderr?.on('data', (d: Buffer) => process.stderr.write(`[user:${userId}] ${d}`))

    // Wait for 'ready' IPC message (max 30s)
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => {
        child.kill('SIGKILL')
        reject(new Error(`UserProcess spawn timeout (30s): userId=${userId}`))
      }, SPAWN_TIMEOUT_MS)

      child.on('message', (msg: any) => {
        if (msg?.type === 'ready') { clearTimeout(timer); resolve() }
      })
      child.on('error', (err) => { clearTimeout(timer); reject(err) })
    })

    const proc: UserProcess = {
      userId, pid: child.pid!, socketPath,
      startedAt: Date.now(), lastSeenAt: Date.now(),
      process: child, respawnCount: 0
    }

    // Auto-cleanup on exit
    child.on('exit', (code) => {
      console.warn(`[SessionManager] UserProcess exited: userId=${userId}, code=${code}`)
      this.processes.delete(userId)
      rm(socketPath, { force: true }).catch(() => {})
    })

    this.processes.set(userId, proc)
    return proc
  }

  touch(userId: string): void {
    const proc = this.processes.get(userId)
    if (proc) proc.lastSeenAt = Date.now()
  }

  getProcess(userId: string): UserProcess | null {
    return this.processes.get(userId) ?? null
  }

  listProcesses(): readonly UserProcess[] {
    return [...this.processes.values()]
  }

  private sweepIdleProcesses(): void {
    const now = Date.now()
    for (const [userId, proc] of this.processes) {
      if (now - proc.lastSeenAt > this.config.idleTimeoutMs) {
        console.log(`[SessionManager] Idle shutdown: userId=${userId}`)
        this.killUserProcess(userId)
      }
    }
  }

  private killUserProcess(userId: string): void {
    const proc = this.processes.get(userId)
    if (!proc) return
    try { proc.process.kill('SIGTERM') } catch { /* already exited */ }
    this.processes.delete(userId)
    rm(proc.socketPath, { force: true }).catch(() => {})
  }

  async shutdown(): Promise<void> {
    if (this.idleTimer) { clearInterval(this.idleTimer); this.idleTimer = null }
    const kills = [...this.processes.keys()].map(userId => this.killUserProcess(userId))
    await Promise.allSettled(kills)
  }
}
```

---

## Acceptance Criteria

- [x] File tồn tại, TypeScript compile sạch
- [x] `getOrSpawnUserProcess()` reuse existing process nếu cùng userId
- [x] `spawnUserProcess()` gọi `mkdir` với `mode: 0o700`
- [x] Env vars đúng: `ORCA_USER_DATA_PATH`, `ORCA_USER_ID`, `ORCA_SOCKET_PATH`, `NODE_OPTIONS`
- [x] Timeout 30s nếu không nhận 'ready' → kill + reject
- [x] Process exit → auto-remove từ map + cleanup socket file
- [x] `touch()` cập nhật `lastSeenAt`
- [x] `shutdown()` kill tất cả processes và clear idle timer
