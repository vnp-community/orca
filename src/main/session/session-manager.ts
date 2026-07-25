/**
 * SessionManager — Fork and track per-user child processes (sandbox)
 *
 * Each authenticated user gets their own OrcaRuntime child process,
 * communicating via a Unix domain socket at `baseDataPath/users/<userId>/orca.sock`.
 *
 * @module main/session/session-manager
 */

import { fork } from 'node:child_process'
import { mkdir, rm } from 'node:fs/promises'
import { join } from 'node:path'
import type { UserProcess, SessionManagerConfig } from './session-types'

const DEFAULT_IDLE_TIMEOUT_MS = 4 * 60 * 60 * 1000   // 4 hours
const DEFAULT_MAX_RESPAWN     = 3
const IDLE_CHECK_INTERVAL_MS  = 5 * 60 * 1000          // check every 5 minutes
const SPAWN_TIMEOUT_MS        = 30_000                  // 30s for fork to be ready

export class SessionManager {
  private readonly processes = new Map<string, UserProcess>()
  private idleTimer: ReturnType<typeof setInterval> | null = null
  private readonly config:   Required<SessionManagerConfig>

  constructor(config: SessionManagerConfig) {
    this.config = {
      idleTimeoutMs:      config.idleTimeoutMs      ?? DEFAULT_IDLE_TIMEOUT_MS,
      maxRespawnAttempts: config.maxRespawnAttempts  ?? DEFAULT_MAX_RESPAWN,
      baseDataPath:       config.baseDataPath,
      userProcessEntry:   config.userProcessEntry
    }

    // Periodic idle-process sweep — unref so it won't prevent process exit
    this.idleTimer = setInterval(() => this.sweepIdleProcesses(), IDLE_CHECK_INTERVAL_MS)
    if (this.idleTimer.unref) this.idleTimer.unref()
  }

  /**
   * Return existing process for userId, or fork a new one.
   * Concurrent calls for the same userId are safe: second call waits
   * on the same spawn promise (map is populated before promise resolves).
   */
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

    // Create per-user directory with restricted permissions (700)
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

    // Prefix stdout/stderr logs with userId for easy filtering
    child.stdout?.on('data', (d: Buffer) => process.stdout.write(`[user:${userId}] ${d}`))
    child.stderr?.on('data', (d: Buffer) => process.stderr.write(`[user:${userId}] ${d}`))

    // Wait for 'ready' IPC message — timeout 30s
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => {
        child.kill('SIGKILL')
        reject(new Error(`UserProcess spawn timeout (30s): userId=${userId}`))
      }, SPAWN_TIMEOUT_MS)

      child.on('message', (msg: unknown) => {
        if (msg && typeof msg === 'object' && (msg as Record<string, unknown>)['type'] === 'ready') {
          clearTimeout(timer)
          resolve()
        }
      })
      child.on('error', (err) => { clearTimeout(timer); reject(err) })
    })

    const proc: UserProcess = {
      userId,
      pid:          child.pid!,
      socketPath,
      startedAt:    Date.now(),
      lastSeenAt:   Date.now(),
      process:      child,
      respawnCount: 0
    }

    // Auto-cleanup on exit: remove from map + delete socket file
    child.on('exit', (code) => {
      console.warn(`[SessionManager] UserProcess exited: userId=${userId}, code=${code}`)
      this.processes.delete(userId)
      rm(socketPath, { force: true }).catch(() => {/* ignore cleanup errors */})
    })

    this.processes.set(userId, proc)
    console.log(`[SessionManager] Spawned process: userId=${userId}, pid=${child.pid}, socket=${socketPath}`)
    return proc
  }

  /** Update lastSeenAt for a user process — call on any WS activity */
  touch(userId: string): void {
    const proc = this.processes.get(userId)
    if (proc) proc.lastSeenAt = Date.now()
  }

  /** Get a process by userId (null if not running) */
  getProcess(userId: string): UserProcess | null {
    return this.processes.get(userId) ?? null
  }

  /** Snapshot of all running user processes */
  listProcesses(): readonly UserProcess[] {
    return [...this.processes.values()]
  }

  private sweepIdleProcesses(): void {
    const now = Date.now()
    for (const [userId, proc] of this.processes) {
      if (now - proc.lastSeenAt > this.config.idleTimeoutMs) {
        console.log(`[SessionManager] Idle shutdown: userId=${userId}, idle=${Math.round((now - proc.lastSeenAt) / 60000)}m`)
        this.killUserProcess(userId)
      }
    }
  }

  private killUserProcess(userId: string): void {
    const proc = this.processes.get(userId)
    if (!proc) return
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
    const userIds = [...this.processes.keys()]
    console.log(`[SessionManager] Shutdown: killing ${userIds.length} user process(es)`)
    for (const userId of userIds) {
      this.killUserProcess(userId)
    }
  }
}
