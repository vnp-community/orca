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
import { WebCredentialStore } from '../credentials/web-credential-store'
import { createTracer } from '../../shared/trace'

const sessionTracer = createTracer('session:spawn')

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

    // Broadcast DevServer events to all active user processes
    const broadcastEvent = (event: string, ...args: any[]) => {
      for (const proc of this.processes.values()) {
        proc.process.send({ type: 'devServer:event', event, args })
      }
    }

    this.config.devServerManager.on('devServer:added', (id: string) => broadcastEvent('devServer:added', id))
    this.config.devServerManager.on('devServer:removed', (id: string) => broadcastEvent('devServer:removed', id))
    this.config.devServerManager.on('devServer:statusChanged', (id: string, status: string, err?: Error) => broadcastEvent('devServer:statusChanged', id, status, err))

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
    const span = sessionTracer.start({ userId })

    // Create per-user directory with restricted permissions (700)
    await mkdir(userDataPath, { recursive: true, mode: 0o700 })

    // Load integration credentials from WebCredentialStore and inject into
    // the child process environment. This is the mechanism by which Bitbucket,
    // Azure DevOps and Gitea tokens reach the integration clients without calling
    // Electron's safeStorage (which is unavailable in headless Node.js).
    //
    // Why env vars: the child process can read them synchronously at start-up
    // without needing async I/O or RPC round-trips. Each user gets their own
    // isolated env because spawnUserProcess is always called per-userId.
    const credentialEnv = await this.buildCredentialEnv(userId)

    const child = fork(this.config.userProcessEntry, [], {
      env: {
        ...process.env,
        ...credentialEnv,               // ORCA_BITBUCKET_*, ORCA_AZURE_*, etc.
        ORCA_USER_DATA_PATH: userDataPath,
        ORCA_USER_ID:        userId,
        ORCA_SOCKET_PATH:    socketPath,
        NODE_OPTIONS:        '--max-old-space-size=512'
      },
      stdio: ['ignore', 'pipe', 'pipe', 'ipc'],
      detached: false
    })

    span.step('forked', { pid: child.pid ?? 0 })

    // Prefix stdout/stderr logs with userId for easy filtering
    child.stdout?.on('data', (d: Buffer) => process.stdout.write(`[user:${userId}] ${d}`))
    child.stderr?.on('data', (d: Buffer) => process.stderr.write(`[user:${userId}] ${d}`))

    // Wait for 'ready' IPC message — timeout 30s
    let rpcAuthToken = ''
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => {
        child.kill('SIGKILL')
        span.fail(`spawn timeout ${SPAWN_TIMEOUT_MS}ms`, { userId })
        reject(new Error(`UserProcess spawn timeout (30s): userId=${userId}`))
      }, SPAWN_TIMEOUT_MS)

      child.on('message', (msg: unknown) => {
        if (msg && typeof msg === 'object') {
          const m = msg as Record<string, unknown>
          if (m['type'] === 'ready') {
            // Why: rpcAuthToken is generated by OrcaRuntimeRpcServer and sent
            // back via IPC so WsSessionRouter can inject it into proxied
            // WebSocket messages. Without it, the Unix socket rejects all
            // requests with "Invalid auth token".
            if (typeof m['rpcAuthToken'] === 'string') {
              rpcAuthToken = m['rpcAuthToken']
            }
            clearTimeout(timer)
            resolve()
          } else if (m['type'] === 'devServer:proxyRequest') {
            const req = m as { requestId: string, method: string, args: any[] }
            this.handleDevServerProxyRequest(child, req)
          }
        }
      })
      child.on('error', (err) => { clearTimeout(timer); span.fail(err, { userId, phase: 'fork' }); reject(err) })
    })

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

    this.processes.set(userId, proc)
    console.log(`[SessionManager] Spawned process: userId=${userId}, pid=${child.pid}, socket=${socketPath}`)
    span.ok({ userId, pid: child.pid ?? 0 })
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

  private async handleDevServerProxyRequest(
    child: import('node:child_process').ChildProcess,
    req: { requestId: string; method: string; args: any[] }
  ): Promise<void> {
    try {
      const dm = this.config.devServerManager
      let result: any
      if (req.method === 'list') {
        result = await dm.list()
      } else if (req.method === 'add') {
        result = await dm.add(req.args[0])
      } else if (req.method === 'remove') {
        result = await dm.remove(req.args[0])
      } else if (req.method === 'connect') {
        result = await dm.connect(req.args[0])
      } else if (req.method === 'disconnect') {
        result = await dm.disconnect(req.args[0])
      } else if (req.method === 'testConnection') {
        result = await dm.testConnection(req.args[0])
      } else if (req.method === 'generateAgentToken') {
        result = await dm.generateAgentToken(req.args[0])
      } else if (req.method === 'relayCall') {
        // relayCall(id, method, params, timeoutMs)
        const [id, rpcMethod, params, timeoutMs] = req.args
        const relay = dm.getRelay(id)
        if (!relay) {
          throw new Error(`Dev server '${id}' relay is not connected.`)
        }
        const { Tracers } = await import('../../shared/trace/tracers')
        const span = Tracers.ipcProxyFlow.start({ devServerId: id, method: rpcMethod })
        try {
          result = await relay.call(rpcMethod, params, timeoutMs)
          span.ok()
        } catch (err) {
          span.fail(err, { devServerId: id, method: rpcMethod })
          throw err
        }
      } else {
        throw new Error(`Unsupported devServer proxy method: ${req.method}`)
      }
      child.send({ type: 'devServer:proxyResponse', requestId: req.requestId, result })
    } catch (err) {
      child.send({
        type: 'devServer:proxyResponse',
        requestId: req.requestId,
        error: err instanceof Error ? err.message : String(err)
      })
    }
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

  /**
   * Read credentials from WebCredentialStore for userId and build the set of
   * environment variables to inject into the child process.
   *
   * Failures are non-fatal: a warning is logged but spawn continues without creds.
   * The user can set tokens later via the credentials.set RPC method and reconnect.
   */
  private async buildCredentialEnv(userId: string): Promise<Record<string, string>> {
    if (!this.config.serverSecret) {
      return {}
    }
    const env: Record<string, string> = {}
    try {
      const store = new WebCredentialStore(this.config.baseDataPath, userId, this.config.serverSecret)

      // Bitbucket — App Password auth
      const bitbucketToken = await store.getToken('bitbucket')
      const bitbucketConfig = await store.getConfig('bitbucket')
      if (bitbucketToken) env['ORCA_BITBUCKET_ACCESS_TOKEN'] = bitbucketToken
      if (bitbucketConfig?.email) env['ORCA_BITBUCKET_EMAIL'] = bitbucketConfig.email
      if (bitbucketConfig?.apiBaseUrl) env['ORCA_BITBUCKET_API_BASE_URL'] = bitbucketConfig.apiBaseUrl

      // Azure DevOps — Personal Access Token
      const azureToken = await store.getToken('azure-devops')
      const azureConfig = await store.getConfig('azure-devops')
      if (azureToken) env['ORCA_AZURE_DEVOPS_TOKEN'] = azureToken
      if (azureConfig?.apiBaseUrl) env['ORCA_AZURE_DEVOPS_API_BASE_URL'] = azureConfig.apiBaseUrl
      if (azureConfig?.username) env['ORCA_AZURE_DEVOPS_USERNAME'] = azureConfig.username

      // Gitea — API token
      const giteaToken = await store.getToken('gitea')
      const giteaConfig = await store.getConfig('gitea')
      if (giteaToken) env['ORCA_GITEA_TOKEN'] = giteaToken
      if (giteaConfig?.apiBaseUrl) env['ORCA_GITEA_API_BASE_URL'] = giteaConfig.apiBaseUrl
    } catch (err) {
      // Non-fatal: integration tokens not yet configured or store not initialised
      console.warn(`[SessionManager] Could not load credentials for user ${userId} (non-fatal):`, (err as Error)?.message)
    }
    return env
  }
}
