// src/relay/agent-session.ts
// Manages a single WebSocket connection session for the Orca Dev Agent.
//
// Responsibilities:
//   1. Send handshake frame immediately on connect
//   2. Send keepalive frames on AGENT_KEEPALIVE_INTERVAL_MS cadence
//   3. Respond to incoming KeepAlive frames with a keepalive pong
//   4. Gate RPC dispatch behind successful handshake
//   5. Close ws with code 1008 on handshake auth failure
//
// Design:
//   - createSession() returns an AgentSession factory — one per WS connection
//   - WireState is created inside start() — NOT at module level
//   - stop() must be called when ws closes to clear the keepalive interval

import type WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import type { AgentLogger } from './agent-logger'
import {
  createWireState,
  decodeFrame,
  encodeDataFrame,
  encodeKeepaliveFrame,
  parseJsonPayload,
} from './agent-wire'
import { createRpcDispatcher } from './agent-rpc-dispatch'
import type { JsonRpcRequest } from './agent-rpc-dispatch'
import {
  AGENT_HANDSHAKE_METHOD,
  AGENT_KEEPALIVE_INTERVAL_MS,
} from '../shared/agent-wire-protocol'
import { MessageType } from '../main/ssh/relay-protocol'
import { createTracer } from '../shared/trace'
import { cleanupAllPtys } from './agent-spawner'
import { notifyDaemonSessionClosed } from './pty-daemon-client'
import { cleanupAgentWatches } from './fs-agent-extensions'

const sessionTracer = createTracer('agent:session')

export type AgentSession = {
  /** Attach session logic to an already-constructed WebSocket (may or may not be open yet). */
  start(ws: WebSocket): void
  /** Clear keepalive interval. Call from ws 'close' handler. */
  stop(): void
  /** Register a callback that fires once after a successful handshake. */
  onHandshakeOk(callback: () => void): void
}

export function createSession(
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger,
  /** Optional: pre-built capabilities (used in tests to bypass async git/pty checks) */
  _prebuiltCapabilities?: readonly string[],
  /** Optional: token override for renewed tokens (supersedes config.agentToken) */
  tokenOverride?: string
): AgentSession {
  let keepaliveTimer: ReturnType<typeof setInterval> | null = null
  let handshakeDone = false
  const handshakeOkCallbacks: (() => void)[] = []
  const dispatcher = createRpcDispatcher(tools, config, log)

  // ── WT-Issue-2: Dynamic capability detection ─────────────────────────────
  /**
   * checkGitAvailable — Check if git binary is accessible in toolPath or system PATH.
   * Quick check via fs.access first, fallback to spawning git --version.
   */
  async function checkGitAvailable(): Promise<boolean> {
    const { access: fsAccess, constants } = await import('node:fs/promises')
    const { join } = await import('node:path')
    const dirs = (config.toolPath ?? process.env['PATH'] ?? '').split(':').filter(Boolean)
    for (const dir of dirs) {
      try {
        await fsAccess(join(dir, 'git'), constants.X_OK)
        return true
      } catch { /* continue to next dir */ }
    }
    // Fallback: try running git --version (works on Windows too)
    const { execFile } = await import('node:child_process')
    return new Promise<boolean>((resolve) => {
      const child = execFile('git', ['--version'], { timeout: 3000 })
      child.on('close', (code) => resolve(code === 0))
      child.on('error', () => resolve(false))
    })
  }

  /**
   * checkPtyAvailable — Check if node-pty native module loads successfully.
   * Returns false if the native module is missing or incompatible.
   */
  async function checkPtyAvailable(): Promise<boolean> {
    try {
      await import('node-pty')
      return true
    } catch {
      return false
    }
  }

  /**
   * buildCapabilities — Dynamically build the capabilities list based on what is
   * actually installed and functional on this Dev Server.
   * Falls back to a static list if the check takes > 5 seconds.
   */
  async function buildCapabilities(): Promise<readonly string[]> {
    const caps: string[] = [
      'fs',
      'fs.watch',
      'preflight',
      'ai.providers',
      'agent.spawn',
      'agent.exec',
      'agent.sendInput',
      'agent.kill',
    ]

    const [hasGit, hasPty] = await Promise.all([checkGitAvailable(), checkPtyAvailable()])

    log.info(`capability check: git=${hasGit} pty=${hasPty}`)

    if (hasGit) {
      caps.push('git', 'git.exec', 'git.execStream')
      caps.push('worktrees', 'git.worktree.list', 'git.worktree.add', 'git.worktree.remove')
    }
    if (hasPty) {
      caps.push(
        'pty', 'pty.create', 'pty.write', 'pty.resize', 'pty.destroy', 'pty.scrollback',
        'pty.stream', 'pty.attach'
      )
    }

    log.info(`capabilities: [${caps.join(', ')}]`)
    return caps
  }

  const STATIC_CAPABILITIES_FALLBACK = [
    'fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees', 'pty',
  ] as const

  async function sendHandshake(ws: WebSocket, wireState: ReturnType<typeof createWireState>): Promise<void> {
    // WT-Issue-2: Use dynamic capabilities with 5s timeout fallback
    // If _prebuiltCapabilities is provided (e.g. in tests), skip the async check entirely.
    let capabilities: readonly string[]
    if (_prebuiltCapabilities) {
      capabilities = _prebuiltCapabilities
    } else {
      try {
        capabilities = await Promise.race([
          buildCapabilities(),
          new Promise<readonly string[]>((_res, reject) =>
            setTimeout(() => reject(new Error('capability check timeout')), 5000)
          ),
        ])
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err)
        log.warn(`buildCapabilities failed (${msg}) — using static fallback`)
        capabilities = STATIC_CAPABILITIES_FALLBACK
      }
    }

    const rpc = {
      jsonrpc: '2.0' as const,
      id: 1,
      method: AGENT_HANDSHAKE_METHOD,
      params: {
        agentVersion:  '5.0.0',
        platform:      process.platform,
        arch:          process.arch,
        nodeVersion:   process.version,
        capabilities,
        // agentToken is only sent in direct-websocket mode; empty string = omit.
        // tokenOverride takes precedence so renewed tokens are used transparently.
        ...((tokenOverride || config.agentToken) ? { agentToken: tokenOverride ?? config.agentToken } : {}),
        devServerId:   config.devServerId,
        tools:         tools.map(t => t.name),
      },
    }
    ws.send(encodeDataFrame(wireState, JSON.stringify(rpc)))
    log.info(`Handshake sent: devServerId=${config.devServerId} tools=[${tools.map(t => t.name).join(',')}]`)
  }

  function startKeepalive(ws: WebSocket, wireState: ReturnType<typeof createWireState>): void {
    keepaliveTimer = setInterval(() => {
      if (ws.readyState === 1 /* WebSocket.OPEN */) {
        ws.send(encodeKeepaliveFrame(wireState))
      }
    }, AGENT_KEEPALIVE_INTERVAL_MS)
  }

  return {
    start(ws: WebSocket): void {
      // wireState is scoped to this connection — not shared
      const wireState = createWireState()
      const span = sessionTracer.start({ devServerId: config.devServerId })

      // sendHandshake is async (builds dynamic capabilities) — wrap in a local helper
      const doHandshake = (): void => {
        void sendHandshake(ws, wireState).then(() => {
          span.step('handshake-sent')
          startKeepalive(ws, wireState)
        }).catch((err: unknown) => {
          const msg = err instanceof Error ? err.message : String(err)
          log.error(`sendHandshake failed: ${msg}`)
          span.fail(err, { phase: 'handshake' })
          ws.close(1011, 'Handshake error')
        })
      }

      if (ws.readyState === 1 /* WebSocket.OPEN */) {
        doHandshake()
      } else {
        ws.once('open', () => {
          log.info('WebSocket opened')
          doHandshake()
        })
      }

      ws.on('message', (data: Buffer | string) => {
        // Agent protocol uses binary frames only
        if (!Buffer.isBuffer(data)) {return}

        const frame = decodeFrame(wireState, data)
        if (!frame) {
          log.warn('Received malformed frame (too short) — ignoring')
          return
        }

        // Respond to KeepAlive frames immediately to maintain ACK progress
        if (frame.type === MessageType.KeepAlive) {
          if (ws.readyState === 1) {ws.send(encodeKeepaliveFrame(wireState))}
          return
        }

        // Empty data frame — ignore
        if (frame.payload.length === 0) {return}

        const rpc = parseJsonPayload<{
          id: string | number | null
          result?: { ok?: boolean; orcaVersion?: string; sessionId?: string }
          error?: { code: number; message: string }
          method?: string
          params?: Record<string, unknown>
        }>(frame.payload)

        if (!rpc) {
          log.warn('Received non-JSON frame payload — ignoring')
          return
        }

        if (!handshakeDone) {
          // Only process handshake result (id=1) before handshake completes
          if (rpc.result?.ok === true) {
            handshakeDone = true
            const sessionId = rpc.result.sessionId ?? 'unknown'
            const orcaVersion = rpc.result.orcaVersion ?? 'unknown'
            log.info(`Handshake OK: sessionId=${sessionId} orcaVersion=${orcaVersion}`)
            span.step('handshake-ok', { sessionId, orcaVersion })
            handshakeOkCallbacks.forEach(cb => cb())
          } else if (rpc.error) {
            log.error(`Handshake failed: code=${rpc.error.code} message=${rpc.error.message}`)
            span.fail(`handshake: ${rpc.error.message}`, { code: rpc.error.code })
            ws.close(1008, 'Handshake failed')
          }
          return
        }

        // Post-handshake: dispatch JSON-RPC request
        if (typeof rpc.method === 'string') {
          void dispatcher.dispatch(ws, wireState, rpc as JsonRpcRequest)
        }
      })

      ws.on('close', (code: number, reason: Buffer) => {
        this.stop()
        const reasonStr = reason.toString()
        if (code === 1000) {
          span.ok({ code, reason: reasonStr })
        } else {
          span.fail(`ws close code=${code}`, { code, reason: reasonStr })
        }
        log.info(`Session closed code=${code} reason=${reasonStr}`)
      })

      ws.on('error', (err: Error) => {
        span.fail(err, { phase: 'ws-error' })
        log.error(`WebSocket error: ${err.message}`)
      })
    },

    stop(): void {
      if (keepaliveTimer !== null) {
        clearInterval(keepaliveTimer)
        keepaliveTimer = null
      }
      // ORCH-011: Kill any orphaned agent-spawned (agent.spawn) PTYs — a
      // separate PTY population from pty.create terminals, with no reattach
      // concept, so these are still cleaned up immediately.
      cleanupAllPtys(log)
      // Terminal (pty.create) PTYs live in the detached pty-daemon process
      // (pty-daemon-client.ts) — tell it this WS session ended so it can arm
      // grace-period timers itself (see pty-agent-bridge.ts). Best-effort and
      // fire-and-forget: stop() must not block on it, and a daemon that's
      // unreachable has no PTYs left to protect anyway. fs.watch watchers
      // have no reattach concept and are cheap to re-establish, so those
      // still clean up immediately.
      void notifyDaemonSessionClosed(log)
      cleanupAgentWatches()
    },

    onHandshakeOk(callback: () => void): void {
      handshakeOkCallbacks.push(callback)
    },
  }
}
