// ─── DevServerRelayBridge ─────────────────────────────────────────────────────
// Wraps the existing SSH relay deploy infrastructure to provide a clean
// interface for DevServerManager. Only relay-ssh mode is implemented in Phase 1.
// relay-websocket and direct-websocket are implemented in Phase 2 (TASK-006, TASK-010).

import EventEmitter from 'node:events'
import type { SshConnectionManager } from '../ssh/ssh-connection-manager'
import type { PersistedDevServer } from '../../shared/dev-server-types'
import { deployAndLaunchRelay } from '../ssh/ssh-relay-deploy'
import type { RelayPlatform } from '../ssh/relay-protocol'
import { SshChannelMultiplexer } from '../ssh/ssh-channel-multiplexer'
import type { AgentDetectionCommand } from '../../shared/agent-detection-commands'
import WebSocket from 'ws'
import { createWebSocketTransport } from './ws-transport'
import { runOrcaInitiatorHandshake } from './ws-handshake'
import { getPlatform } from '../../platform/context'
import type { AgentWebSocketServer } from './agent-ws-server'
import { generateAgentToken, AGENT_WS_PATH } from '../../shared/agent-wire-protocol'
import { createTracer } from '../../shared/trace'

const relayCallTracer = createTracer('relay:agentCall')

export type RelayHandshakeInfo = {
  platform: NodeJS.Platform
  arch: string
  nodeVersion: string
  relayVersion: string
}

/**
 * Maps a RelayPlatform string (e.g. 'darwin-arm64') to NodeJS.Platform
 * and arch string.
 */
function parseRelayPlatform(relayPlatform: RelayPlatform): {
  platform: NodeJS.Platform
  arch: string
} {
  const [os, arch] = relayPlatform.split('-') as [string, string]
  const platform: NodeJS.Platform =
    os === 'linux' ? 'linux' : os === 'darwin' ? 'darwin' : 'win32'
  return { platform, arch: arch ?? 'x64' }
}

export class DevServerRelayBridge extends EventEmitter {
  /** The active relay multiplexer. Exposed so IPC handlers can forward relay calls. */
  session: SshChannelMultiplexer | null = null
  /** Disposer for active direct-websocket slot — cleaned up on disconnect */
  private _directWsDisposer: (() => void) | null = null
  /** Cancel flag — set to false in disconnect() to stop the relay-ws reconnect loop */
  private _relayWsActive = false
  /** Timer handle for relay-ws reconnect delay */
  private _relayWsReconnectTimer: ReturnType<typeof setTimeout> | null = null

  /**
   * Reconnect queue: when the agent WebSocket closes and we're awaiting a new
   * connection, incoming RPC calls are queued here (max RECONNECT_WAIT_MS)
   * instead of failing immediately with 'Connection lost, reconnecting...'.
   * Drained in FIFO order when onSessionRestored() is called.
   */
  private _reconnecting = false
  private readonly _reconnectWaiters: Array<(session: SshChannelMultiplexer | null) => void> = []
  /** Max time (ms) a queued call will wait for session to be restored */
  private static readonly RECONNECT_WAIT_MS = 20_000

  constructor(
    private config: PersistedDevServer,
    private sshManager: SshConnectionManager,
    private agentWsServer: AgentWebSocketServer | null = null
  ) {
    super()
  }

  /**
   * Called internally when a new session is successfully established after
   * a disconnect. Flushes all queued RPC waiters with the new session.
   */
  private onSessionRestored(mux: SshChannelMultiplexer): void {
    this._reconnecting = false
    // Drain waiters in FIFO order
    for (const resolve of this._reconnectWaiters.splice(0)) {
      resolve(mux)
    }
  }

  /**
   * Called internally when a WebSocket session drops.
   * Marks bridge as reconnecting so subsequent RPC calls queue instead of throw.
   */
  private onSessionDropped(): void {
    this._reconnecting = true
  }

  async connect(opts: { testOnly?: boolean } = {}): Promise<RelayHandshakeInfo> {
    if (this.config.connectionType === 'relay-ssh') {
      const sshTargetId = this.config.sshTargetId
      if (!sshTargetId) {
        throw new Error(
          `DevServer '${this.config.name}' (${this.config.id}) has no sshTargetId set`
        )
      }

      const conn = this.sshManager.getConnection(sshTargetId)
      if (!conn) {
        throw new Error(
          `SSH connection for target '${sshTargetId}' not found. ` +
            `Connect to the SSH target before connecting the dev server.`
        )
      }

      // Deploy (or reuse) the relay on the remote host.
      // deployAndLaunchRelay returns a RelayDeployResult with transport + platform.
      const result = await deployAndLaunchRelay(conn, undefined, undefined, undefined)

      const { platform: nodePlatform, arch } = parseRelayPlatform(result.platform)

      // Store transport as session for downstream relay calls (TASK-013+).
      // The actual session object from the existing infrastructure is the transport.
      this.session = result.transport

      // Close immediately if this is a test-only probe.
      if (opts.testOnly) {
        await this.disconnect()
      }

      return {
        platform: nodePlatform,
        arch,
        // nodeVersion and relayVersion are not part of RelayDeployResult yet
        // (extended in TASK-006). Use placeholder until handshake extension lands.
        nodeVersion: 'unknown',
        relayVersion: 'unknown'
      }
    }

    // ─── Phase 2: relay-websocket ────────────────────────────────────────────
    if (this.config.connectionType === 'relay-websocket') {
      const wsUrl = this.config.wsUrl
      if (!wsUrl) {
        throw new Error(
          `DevServer '${this.config.name}' has no wsUrl configured. ` +
          `Set wsUrl to ws://host:port/orca-relay?token=<secret> for relay-websocket mode.`
        )
      }
      return this.connectRelayWebSocket(wsUrl, opts)
    }

    // ─── Phase 2: direct-websocket ───────────────────────────────────────────
    // Implemented in TASK-010 after AgentWebSocketServer is available
    if (this.config.connectionType === 'direct-websocket') {
      return this.connectDirectWebSocket(opts)
    }

    throw new Error(
      `Connection type '${this.config.connectionType}' is not supported. ` +
      `Supported types: relay-ssh, relay-websocket, direct-websocket`
    )
  }

  async disconnect(): Promise<void> {
    // Stop reconnect loop and queue
    this._relayWsActive = false
    this._reconnecting = false
    if (this._relayWsReconnectTimer) {
      clearTimeout(this._relayWsReconnectTimer)
      this._relayWsReconnectTimer = null
    }
    // Drain queued waiters with null (permanent disconnect)
    for (const resolve of this._reconnectWaiters.splice(0)) {
      resolve(null)
    }
    // Cancel direct-websocket slot if still pending
    if (this._directWsDisposer) {
      this._directWsDisposer()
      this._directWsDisposer = null
    }
    if (this.session && typeof this.session.close === 'function') {
      await this.session.close()
    } else if (this.session && typeof (this.session as unknown as { destroy?(): void }).destroy === 'function') {
      (this.session as unknown as { destroy(): void }).destroy()
    }
    this.session = null
  }

  /**
   * Returns true when this bridge has an active session (relay-ssh, relay-ws,
   * or direct-websocket). Used by RelayConnectionPool to determine whether
   * an existing connection can be reused before requesting a new one.
   */
  isAlive(): boolean {
    return this.session !== null
  }

  /**
   * direct-websocket mode: Orca is WS SERVER; agent will connect inbound.
   *
   * Flow:
   *   1. Generate unique agentToken
   *   2. Register slot in AgentWebSocketServer (60s expiry)
   *   3. Emit 'agentTokenGenerated' event → UI can display token + setup command
   *   4. Wait for agent to connect and handshake (max 60s via slot timer)
   *   5. On success: session = mux, resolve RelayHandshakeInfo
   *   6. On timeout: reject with setup instructions
   */
  private connectDirectWebSocket(opts: { testOnly?: boolean }): Promise<RelayHandshakeInfo> {
    if (!this.agentWsServer) {
      return Promise.reject(
        new Error(
          'direct-websocket mode requires AgentWebSocketServer to be initialized. ' +
          'Ensure server-bootstrap.ts creates and passes AgentWebSocketServer to DevServerManager.'
        )
      )
    }

    const agentToken = generateAgentToken(this.config.id)

    return new Promise<RelayHandshakeInfo>((resolve, reject) => {
      const disposer = this.agentWsServer!.registerSlot(
        agentToken,
        // onConnected: agent successfully connected and handshaked
        (mux, info) => {
          this._directWsDisposer = null
          this.session = mux

          if (opts.testOnly) {
            void this.disconnect()
          }

          resolve({
            platform: (info.platform as NodeJS.Platform) ?? 'linux',
            arch: info.arch,
            nodeVersion: info.nodeVersion,
            relayVersion: info.agentVersion,
          })
        },
        // onExpired: agent did not connect within 60s
        (reason) => {
          this._directWsDisposer = null
          reject(new Error(reason))
        }
      )

      this._directWsDisposer = disposer

      // Notify UI so user can configure and start the agent.
      // TASK-DS-006 (BUG-DS-003): resolve Orca WS URL from env instead of literal placeholder.
      // Priority:
      //   1. ORCA_AGENT_WS_URL — full override (e.g. wss://b15.openledger.vn/agent)
      //   2. ws://{ORCA_ADVERTISED_HOST}:{ORCA_HTTP_PORT}/agent
      //   3. ws://localhost:6769/agent (dev fallback)
      const orcaWsUrl =
        process.env['ORCA_AGENT_WS_URL'] ??
        (() => {
          const host = process.env['ORCA_ADVERTISED_HOST'] ?? 'localhost'
          const port = process.env['ORCA_HTTP_PORT'] ?? '6769'
          return `ws://${host}:${port}${AGENT_WS_PATH}`
        })()
      this.emit('agentTokenGenerated', {
        devServerId: this.config.id,
        agentToken,
        orcaUrl: orcaWsUrl,
      })
    })
  }

  /**
   * Daemon-initiated connection: wire a PRE-REGISTERED token into this bridge's session.
   *
   * Used by POST /api/agent-token → DevServerManager.connectDaemonAgent() when the
   * daemon was started with a token pre-registered via the REST API (not via the UI
   * "Add Dev Server" flow). The token is already in AgentWebSocketServer.pendingSlots.
   *
   * Unlike connectDirectWebSocket(), this does NOT generate a new token — it attaches
   * to the slot that was registered by the HTTP API handler.
   *
   * TTL is specified at slot-registration time; callers pass it here only for logging.
   */
  connectWithExternalToken(token: string): Promise<RelayHandshakeInfo> {
    if (!this.agentWsServer) {
      return Promise.reject(
        new Error(
          'direct-websocket mode requires AgentWebSocketServer to be initialized.'
        )
      )
    }

    // Cancel any pending direct-ws slot from a previous connectWithExternalToken call.
    if (this._directWsDisposer) {
      this._directWsDisposer()
      this._directWsDisposer = null
    }

    return new Promise<RelayHandshakeInfo>((resolve, reject) => {
      const disposer = this.agentWsServer!.registerSlot(
        token,
        (mux, info) => {
          this._directWsDisposer = null
          this.session = mux

          // CRITICAL: clear session when the agent WebSocket closes.
          // Without this, bridge.session remains non-null with a dead mux, causing
          // connectDaemonAgent to always take the "already connected" path, which
          // calls connectWithExternalToken again and cancels the NEW token's slot
          // before the new agent can connect — producing the 20s handshake timeout loop.
          mux.onDispose(() => {
            if (this.session === mux) {
              console.log(`[DevServerRelayBridge] Agent WS closed — clearing session (direct-ws mode)`)
              this.session = null
              this.onSessionDropped()  // mark reconnecting → queue subsequent calls
            }
          })

          // If this is a reconnect (waiters are queued), flush them.
          if (this._reconnecting || this._reconnectWaiters.length > 0) {
            console.log(`[DevServerRelayBridge] Session restored — flushing ${this._reconnectWaiters.length} queued call(s)`)
            this.onSessionRestored(mux)
          } else {
            this._reconnecting = false
          }

          resolve({
            platform: (info.platform as NodeJS.Platform) ?? 'linux',
            arch: info.arch,
            nodeVersion: info.nodeVersion,
            relayVersion: info.agentVersion,
          })
        },
        (reason) => {
          this._directWsDisposer = null
          reject(new Error(reason))
        }
      )
      this._directWsDisposer = disposer
    })
  }


  /**
   * relay-websocket mode: Orca acts as WS CLIENT connecting to agent's WS server.
   *
   * URL format: ws://host:port/path?token=<secret>
   *   Token is stripped from URL and sent as Authorization: Bearer <token> header.
   *
   * Flow:
   *   1. TCP connect to agent WS server (10s timeout)
   *   2. Run agent.handshake — Orca initiator (20s timeout)
   *   3. Wire SshChannelMultiplexer on transport
   *   4. Return RelayHandshakeInfo
   */
  private connectRelayWebSocket(
    rawUrl: string,
    opts: { testOnly?: boolean }
  ): Promise<RelayHandshakeInfo> {
    // Parse token from URL: ws://host:port/path?token=<secret>
    // Strip it before creating WS — send as Authorization header instead.
    const url = new URL(rawUrl)
    const token = url.searchParams.get('token') ?? ''
    url.searchParams.delete('token')
    const cleanUrl = url.toString()

    const orcaVersion = getPlatform().app.getVersion()

    // TASK-DS-008 (BUG-DS-005): enable reconnect loop for non-testOnly connections.
    this._relayWsActive = !opts.testOnly

    return new Promise<RelayHandshakeInfo>((resolve, reject) => {
      let initialResolved = false

      const attempt = () => {
        if (!this._relayWsActive) return  // disconnect() was called — stop loop

        const ws = new WebSocket(cleanUrl, {
          headers: token ? { Authorization: `Bearer ${token}` } : {},
        })
        ;(ws as unknown as { binaryType: string }).binaryType = 'nodebuffer'

        const connectionTimeout = setTimeout(() => {
          ws.terminate()
          const timeoutMsg =
            `relay-websocket: TCP connection timed out after 10s to ${cleanUrl}. ` +
            `Verify the agent is running and the address is reachable.`
          if (!initialResolved) {
            reject(new Error(timeoutMsg))
          } else {
            console.warn(`[RelayBridge] ${timeoutMsg} Retry in 15s.`)
          }
        }, 10_000)

        ws.on('error', (err: Error) => {
          clearTimeout(connectionTimeout)
          if (!initialResolved) {
            reject(new Error(
              `relay-websocket: WebSocket error connecting to ${cleanUrl}: ${err.message}`
            ))
          } else {
            console.warn(`[RelayBridge] relay-ws error: ${err.message}`)
            // 'close' event fires next — retry handled there
          }
        })

        ws.on('open', () => {
          clearTimeout(connectionTimeout)

          runOrcaInitiatorHandshake(ws, orcaVersion)
            .then((info) => {
              const transport = createWebSocketTransport(ws)
              this.session = new SshChannelMultiplexer(transport, {
                connectionLostMessage: 'Connection lost, reconnecting...'
              })

              if (opts.testOnly) {
                void this.disconnect()
              } else {
                // TASK-DS-008: monitor close → trigger reconnect
                ws.on('close', () => {
                  if (this.session) {
                    console.log('[RelayBridge] relay-ws disconnected — clearing session')
                    this.session = null
                    this.onSessionDropped()  // queue subsequent RPC calls
                  }
                  if (this._relayWsActive) {
                    console.log('[RelayBridge] relay-ws will reconnect in 15s...')
                    this._relayWsReconnectTimer = setTimeout(attempt, 15_000)
                  }
                })
              }

              if (!initialResolved) {
                initialResolved = true
                resolve({
                  platform: (info.platform as NodeJS.Platform) ?? 'linux',
                  arch: info.arch,
                  nodeVersion: info.nodeVersion,
                  relayVersion: info.agentVersion,
                })
              } else {
                console.log('[RelayBridge] relay-ws reconnected successfully')
                // Flush any calls that were queued during the reconnect gap
                if (this._reconnectWaiters.length > 0) {
                  console.log(`[RelayBridge] Flushing ${this._reconnectWaiters.length} queued call(s) after relay-ws reconnect`)
                  this.onSessionRestored(this.session!)
                } else {
                  this._reconnecting = false
                }
              }
            })
            .catch((err: Error) => {
              ws.close()
              if (!initialResolved) {
                reject(err)
              } else {
                console.warn(`[RelayBridge] relay-ws handshake failed: ${err.message} — retry in 15s`)
                if (this._relayWsActive) {
                  this._relayWsReconnectTimer = setTimeout(attempt, 15_000)
                }
              }
            })
        })
      }

      attempt()
    })
  }

  /**
   * Forward agent detection to the relay process.
   * The relay runs the probe commands and returns which agents are installed
   * along with the dev server's platform.
   *
   * @throws Error('Not connected') when the relay session is not established.
   */
  async detectAgents(commands: AgentDetectionCommand[]): Promise<{
    agents: string[]
    platform: NodeJS.Platform
  }> {
    if (!this.session) throw new Error('Not connected')

    // Timeout: agent detection is bounded to 15 seconds (relay probes PATH).
    const result = await this.callWithTimeout<{
      agents: string[]
      platform: NodeJS.Platform
    }>('preflight.detectAgents', { commands }, 15_000)
    return { agents: result.agents, platform: result.platform }
  }

  /**
   * Forward an arbitrary relay RPC call.
   * Used by onboarding-ipc handlers that need to invoke relay methods
   * (preflight.check, preflight.setGitIdentity, preflight.detectGhosttyConfig, etc.)
   * without exposing the raw SshChannelMultiplexer session.
   *
   * @throws Error('Not connected') when the relay session is not established.
   */
  async call<T = unknown>(
    method: string,
    params: Record<string, unknown> = {},
    timeoutMs = 30_000
  ): Promise<T> {
    if (!this.session) throw new Error('Not connected')
    return this.callWithTimeout<T>(method, params, timeoutMs)
  }


  /**
   * Wraps SshChannelMultiplexer.request() with an explicit timeout guard.
   * Why: the multiplexer has its own 30s default but agent detection should fail
   * faster (15s) so the UI remains responsive if the relay stalls.
   *
   * Reconnect queue: if session is null but bridge is reconnecting (agent WS just
   * dropped), this method waits up to RECONNECT_WAIT_MS for the session to be
   * restored before failing. This prevents the 'Connection lost, reconnecting...'
   * error from propagating to the user during the ~15s agent restart window.
   */
  private async callWithTimeout<T>(
    method: string,
    params: Record<string, unknown>,
    timeoutMs: number
  ): Promise<T> {
    // If session is available, call immediately
    let session = this.session

    // If session is null but we're reconnecting, wait for it to be restored
    if (!session && this._reconnecting) {
      console.log(`[DevServerRelayBridge] Session reconnecting — queuing call '${method}' (wait up to ${DevServerRelayBridge.RECONNECT_WAIT_MS}ms)`)
      session = await new Promise<SshChannelMultiplexer | null>((resolve) => {
        const timeout = setTimeout(() => {
          // Remove this waiter from the queue on timeout
          const idx = this._reconnectWaiters.indexOf(resolve)
          if (idx !== -1) this._reconnectWaiters.splice(idx, 1)
          resolve(null)
        }, DevServerRelayBridge.RECONNECT_WAIT_MS)

        this._reconnectWaiters.push((mux) => {
          clearTimeout(timeout)
          resolve(mux)
        })
      })
    }

    if (!session) {
      const span = relayCallTracer.start({ devServerId: this.config.id, method })
      span.fail('Not connected', { method, devServerId: this.config.id })
      throw new Error('Not connected')
    }

    const span = relayCallTracer.start({ devServerId: this.config.id, method })
    return new Promise<T>((resolve, reject) => {
      const timer = setTimeout(
        () => {
          span.fail(`timed out after ${timeoutMs}ms`, { method, devServerId: this.config.id })
          reject(new Error(`Relay call '${method}' timed out after ${timeoutMs}ms`))
        },
        timeoutMs
      )
      session!.request(method, params)
        .then((result: unknown) => {
          clearTimeout(timer)
          span.ok({ method })
          resolve(result as T)
        })
        .catch((err: unknown) => {
          clearTimeout(timer)
          span.fail(err, { method, devServerId: this.config.id })
          reject(err)
        })
    })
  }
}
