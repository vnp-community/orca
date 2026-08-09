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
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import { createSession } from './agent-session'
import type { AgentLogger } from './agent-logger'
import { createTracer } from '../shared/trace'
import { AgentTokenManager, AGENT_TOKEN_DEFAULT_TTL_SEC } from './agent-token-manager'

const connTracer = createTracer('agent:connection')

/** Reconnect backoff delays (ms): 1s → 2s → 5s → 15s → 30s (max) */
const RECONNECT_DELAYS_MS = [1_000, 2_000, 5_000, 15_000, 30_000]

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
      name:        config.devServerId,
      apiSecret:   config.apiSecret,
      ttlSec:      AGENT_TOKEN_DEFAULT_TTL_SEC,
      log,
    })
    await tokenManager.init()

    // Graceful cleanup on shutdown
    process.on('SIGINT',  () => { tokenManager?.dispose(); process.exit(0) })
    process.on('SIGTERM', () => { tokenManager?.dispose(); process.exit(0) })
  } else {
    // Legacy: AGENT_TOKEN from env, no renewal
    if (!config.agentToken) {
      log.error('Neither ORCA_AGENT_API_SECRET nor AGENT_TOKEN is set.')
      log.error('Set ORCA_AGENT_API_SECRET to enable automatic token renewal.')
      process.exit(1)
    }
    log.warn('[TokenManager] ORCA_AGENT_API_SECRET not set — using static AGENT_TOKEN (no renewal).')
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
        rejectUnauthorized: config.tlsRejectUnauthorized,
      })

      const session = createSession(config, tools, log, undefined, token)
      session.onHandshakeOk(() => {
        lastHandshakeOk = true
        reconnectAttempt = 0   // reset backoff on successful auth
        connSpan.step('handshake-ok')
        log.info('Connection established and authenticated.')
      })
      session.start(ws)

      ws.once('close', (code: number) => {
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
          return
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
          return
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
        log.error(`Failed to force renew token: ${err instanceof Error ? err.message : String(err)}`)
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
  return new Promise(resolve => setTimeout(resolve, ms))
}
