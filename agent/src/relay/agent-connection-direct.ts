// src/relay/agent-connection-direct.ts
// Direct-websocket connection mode: Agent connects outbound to Orca Server.
//
// Token lifecycle (with AgentTokenManager):
//   startup → manager.init() fetches initial token (permanent, TTL=24h)
//   → connect WS → handshake OK → session running
//   → at 80% of TTL → manager pre-fetches next token silently in background
//   → WS drops (network blip, server restart, etc.)
//   → reconnect: manager.consume() returns pre-fetched token if ready, else current
//   → backoff delay → new WS → handshake OK → session restored
//
// Exit codes:
//   0 = clean close (code 1000 — server requested graceful shutdown)
//   1 = startup failure or unrecoverable error

import WebSocket from 'ws'
import type { Socket } from 'node:net'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import { createSession } from './agent-session'
import type { AgentLogger } from './agent-logger'
import { createTracer } from '../shared/trace'
import { AgentTokenManager, AGENT_TOKEN_DEFAULT_TTL_SEC } from './agent-token-manager'

const connTracer = createTracer('agent:connection')

/** Reconnect backoff delays (ms): 1s → 2s → 5s → 15s → 30s (max) */
const RECONNECT_DELAYS_MS = [1_000, 2_000, 5_000, 15_000, 30_000]

/**
 * Minimum time a connection must stay handshaked before a later disconnect
 * counts as "recovered" and resets the backoff to its shortest delay.
 *
 * Why: without this, a handshake that succeeds and then collapses again
 * within milliseconds (a flapping WS path — proxy/LB hiccup, transient
 * server-side session-slot race, etc.) resets reconnectAttempt to 0 on
 * every single cycle, since the old code reset it the instant handshake-ok
 * fired rather than once the connection proved it could actually stay up.
 * The backoff then never escalates past its 1s floor, turning a recoverable
 * blip into a sustained ~1/sec reconnect storm that hammers the server
 * indefinitely (observed live: WS churn continued 16+ minutes after the
 * triggering disconnect, every cycle waiting exactly RECONNECT_DELAYS_MS[0]).
 * A connection that stays up for this long is genuinely healthy again and
 * deserves a fresh attempt count; one that doesn't should keep backing off.
 */
const STABLE_CONNECTION_MS = 10_000

/** Sent on an interval to keep the connection past any intermediate proxy's
 *  idle-read timeout (LB, reverse proxy, NAT conntrack) — mirrors the
 *  server's own 30s ping (agent-ws-server.ts) but from this side too, since
 *  a proxy that only resets its idle timer on frames FROM the client would
 *  otherwise still close an outbound-idle connection despite the server's
 *  pings arriving fine. Well under any observed/typical idle-timeout range. */
const KEEPALIVE_PING_MS = 20_000

export async function connectDirect(
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger
): Promise<never> {
  // ── Token manager: handles initial fetch + proactive renewal ──────────────
  // Requires ORCA_AGENT_API_SECRET. If absent, falls back to AGENT_TOKEN env
  // var (legacy one-shot mode — no renewal, systemd restart on disconnect).
  let tokenManager: AgentTokenManager | null = null

  if (config.apiSecret) {
    tokenManager = new AgentTokenManager({
      orcaHttpUrl: config.orcaHttpUrl,
      devServerId: config.devServerId,
      name: config.devServerId,
      apiSecret: config.apiSecret,
      ttlSec: AGENT_TOKEN_DEFAULT_TTL_SEC,
      log
    })
    await tokenManager.init()

    // Graceful cleanup on shutdown. dispose() is wrapped defensively — an
    // exception here must never prevent process.exit() (agent-entry.ts's
    // own SIGTERM listener already logs and exits; this one exists so
    // token-renewal state is cleaned up too, not as the sole exit path).
    const disposeAndExit = (): void => {
      try {
        tokenManager?.dispose()
      } catch {
        /* best effort — still exit */
      }
      process.exit(0)
    }
    process.on('SIGINT', disposeAndExit)
    process.on('SIGTERM', disposeAndExit)
  } else {
    // Legacy: AGENT_TOKEN from env, no renewal
    if (!config.agentToken) {
      log.error('Neither ORCA_AGENT_API_SECRET nor AGENT_TOKEN is set.')
      log.error('Set ORCA_AGENT_API_SECRET to enable automatic token renewal.')
      process.exit(1)
    }
    log.warn(
      '[TokenManager] ORCA_AGENT_API_SECRET not set — using static AGENT_TOKEN (no renewal).'
    )
  }

  // ── Reconnect loop ────────────────────────────────────────────────────────
  let reconnectAttempt = 0

  const runConnection = (): Promise<'reconnect-renew' | 'reconnect-auth-failed' | 'exit'> =>
    new Promise((resolve) => {
      const token = tokenManager ? tokenManager.consume() : config.agentToken
      const connSpan = connTracer.start({ url: config.orcaUrl, attempt: reconnectAttempt })
      let lastHandshakeOk = false

      log.info(`Connecting to ${config.orcaUrl} ...`)

      const ws = new WebSocket(config.orcaUrl, {
        headers: { 'User-Agent': 'orca-dev-agent/2.1.0' },
        rejectUnauthorized: config.tlsRejectUnauthorized
      })

      // TEMP DIAG BUG-FE-PTY-001: no application code was found calling
      // ws.close()/terminate() around the observed disconnect — wrap both so
      // ANY call (ours, or from a library path we haven't traced) logs a
      // stack trace instead of silently firing.
      const diagOrigClose = ws.close.bind(ws)
      const diagOrigTerminate = ws.terminate.bind(ws)
      ws.close = ((...args: Parameters<typeof diagOrigClose>) => {
        log.error(
          `[DIAG BUG-FE-PTY-001] ws.close() called args=${JSON.stringify(args)} readyState=${ws.readyState}\n${new Error('close call site').stack}`
        )
        return diagOrigClose(...args)
      }) as typeof ws.close
      ws.terminate = (() => {
        log.error(
          `[DIAG BUG-FE-PTY-001] ws.terminate() called readyState=${ws.readyState}\n${new Error('terminate call site').stack}`
        )
        return diagOrigTerminate()
      }) as typeof ws.terminate

      // TEMP DIAG BUG-FE-PTY-001: raw socket-level visibility — logged once,
      // right when the underlying TCP socket itself signals it's ending, to
      // see which side/reason the 'ws' library attributes it to before any
      // of our own 'close' handling runs.
      ws.once('open', () => {
        const sock = (ws as unknown as { _socket?: Socket })._socket
        sock?.once('end', () => log.error('[DIAG BUG-FE-PTY-001] raw socket "end" (peer sent FIN)'))
        sock?.once('close', (hadError: boolean) =>
          log.error(`[DIAG BUG-FE-PTY-001] raw socket "close" hadError=${hadError}`)
        )
        sock?.once('error', (err: Error) =>
          log.error(`[DIAG BUG-FE-PTY-001] raw socket "error": ${err.stack ?? err.message}`)
        )
      })

      let handshakeOkAt: number | null = null
      // Why this side pings too (not just relying on the server's own 30s
      // ping in agent-ws-server.ts): defends against ANY intermediate hop
      // (LB, reverse proxy, NAT conntrack) that only resets its idle timer
      // on frames it sees FROM this side — the server pinging outbound
      // wouldn't reset that half of the timeout.
      let keepAliveTimer: ReturnType<typeof setInterval> | null = null

      const session = createSession(config, tools, log, undefined, token)
      session.onHandshakeOk(() => {
        lastHandshakeOk = true
        handshakeOkAt = Date.now()
        connSpan.step('handshake-ok')
        log.info('Connection established and authenticated.')
        keepAliveTimer = setInterval(() => {
          try {
            ws.ping()
          } catch {
            /* socket already closing — 'close' handles cleanup */
          }
        }, KEEPALIVE_PING_MS)
      })
      session.start(ws)

      ws.once('close', (code: number) => {
        // TEMP DIAG BUG-FE-PTY-001
        log.error(`[DIAG BUG-FE-PTY-001] ws 'close' event fired code=${code} t=${Date.now()}`)
        if (keepAliveTimer) {
          clearInterval(keepAliveTimer)
          keepAliveTimer = null
        }
        session.stop()

        if (code === 1000) {
          connSpan.ok({ code })
          log.info('Connection closed cleanly (code=1000). Shutting down.')
          resolve('exit')
          return
        }

        if (lastHandshakeOk) {
          connSpan.fail(`connection dropped after handshake`, { code })
          log.warn(`Connection dropped (code=${code}). Reconnecting...`)
          // Why reset backoff here (not at handshake-ok, see STABLE_CONNECTION_MS
          // above): only a connection that actually stayed up counts as
          // recovered. One that handshakes and immediately collapses again
          // must keep backing off, or a flapping path turns into a sustained
          // ~1/sec reconnect storm that never gives the path time to settle.
          const stableMs = handshakeOkAt !== null ? Date.now() - handshakeOkAt : 0
          if (stableMs >= STABLE_CONNECTION_MS) {
            reconnectAttempt = 0
          }
          // Why renew here too, not just on outright rejection: the server
          // consumes an agent token the moment it first connects successfully
          // (AgentWebSocketServer.registerSlot — single-use by design), so a
          // token that already handshaked once is guaranteed stale for the
          // NEXT reconnect. The token manager's own 80%-of-TTL background
          // renewal won't help here — the server grants ~30-day TTLs, so that
          // timer doesn't fire again for weeks. Renewing immediately means
          // the very next connection attempt uses a fresh token instead of
          // failing once first (see BUG-DS-AWS below for the same fix on the
          // outright-rejected path).
          resolve('reconnect-renew')
        } else {
          connSpan.fail(`closed before handshake`, { code })
          log.warn(`Connection closed before handshake (code=${code}). Reconnecting...`)
          // FIX BUG-DS-AWS: treat ANY close before a successful handshake as a
          // token problem, not just code 1008. A rejected/unregistered token
          // (e.g. after an Orca Server restart wipes in-memory pending slots)
          // is the overwhelmingly common cause of a pre-handshake close, and the
          // exact wire code isn't reliable across ws versions/proxies (a bare
          // ws.close() with no code surfaces as 1005, not 1008). Without this,
          // the agent retries the same dead token forever instead of renewing it.
          resolve('reconnect-auth-failed')
        }
      })

      ws.once('error', (err) => {
        connSpan.fail(err)
        log.warn(`WebSocket error: ${err.message}. Reconnecting...`)
        // 'close' will fire after 'error' — let it resolve the promise
      })
    })

  // Main loop
  while (true) {
    const result = await runConnection()

    if (result === 'exit') {
      tokenManager?.dispose()
      setTimeout(() => process.exit(0), 100)
      return new Promise<never>(() => {})
    }

    if (tokenManager) {
      log.warn(
        result === 'reconnect-auth-failed'
          ? 'Handshake rejected (likely unregistered token). Forcing proactive token renewal...'
          : 'Connection dropped after a successful handshake — its token is now stale server-side. Forcing proactive token renewal...'
      )
      try {
        await tokenManager.forceRenew()
      } catch (err) {
        log.error(
          `Failed to force renew token: ${err instanceof Error ? err.message : String(err)}`
        )
      }
    }

    // Reconnect with backoff
    const delay = RECONNECT_DELAYS_MS[Math.min(reconnectAttempt, RECONNECT_DELAYS_MS.length - 1)]
    reconnectAttempt += 1
    log.info(`Reconnect in ${delay / 1000}s (attempt ${reconnectAttempt})...`)
    await sleep(delay)
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}
