// src/relay/agent-token-manager.ts
//
// AgentTokenManager — proactive token renewal for direct-websocket mode.
//
// Design:
//   - Fetches an initial token via POST /api/agent-token at startup.
//   - Schedules a background renewal at TOKEN_RENEW_RATIO (80%) of TTL.
//   - Renewed token is pre-registered on the server and held in memory,
//     so when the current WS drops the agent can reconnect immediately
//     with a fresh token (no delay waiting for HTTP call).
//   - Uses exponential backoff for renewal failures; never lets the
//     token expire without at least 3 retry attempts.
//
// Token lifecycle:
//   startup → fetch(ttl=86400, permanent=true) → schedule renew at 80% TTL
//   → connected  (current token consumed by handshake)
//   → at 80% TTL → fetch NEW token (pre-register on server, hold in .next)
//   → WS drops   → reconnect with .next token → .next becomes .current
//   → schedule next renew at 80% of new TTL

import { createTracer } from '../shared/trace'
import type { AgentLogger } from './agent-logger'

const tokenTracer = createTracer('agent:tokenManager')

// ─── Constants ────────────────────────────────────────────────────────────────

/** Start renewing when this fraction of TTL has elapsed (0.8 = 80%) */
const TOKEN_RENEW_RATIO = 0.8

/** Default TTL to request from server (24h). */
export const AGENT_TOKEN_DEFAULT_TTL_SEC = 86_400

/** Retry delays for fetch failures (ms): 5s → 15s → 30s → 60s */
const RETRY_DELAYS_MS = [5_000, 15_000, 30_000, 60_000]

/** Max renewal attempts before giving up and keeping old token */
const MAX_RENEWAL_ATTEMPTS = 5

// ─── Types ────────────────────────────────────────────────────────────────────

export interface FetchedToken {
  token: string
  /** TTL in seconds as returned by the server */
  ttlSec: number
  fetchedAt: number
}

export interface TokenManagerOptions {
  /** Base HTTP URL of the Orca server, e.g. http://172.20.2.39:6769 */
  orcaHttpUrl: string
  devServerId: string
  name: string
  apiSecret: string
  /** TTL seconds to request. Defaults to AGENT_TOKEN_DEFAULT_TTL_SEC */
  ttlSec?: number
  log: AgentLogger
}

// ─── AgentTokenManager ───────────────────────────────────────────────────────

export class AgentTokenManager {
  private current: FetchedToken | null = null
  private next:    FetchedToken | null = null
  private renewTimer: ReturnType<typeof setTimeout> | null = null
  private disposed = false

  private readonly opts: Required<TokenManagerOptions>

  constructor(opts: TokenManagerOptions) {
    this.opts = { ttlSec: AGENT_TOKEN_DEFAULT_TTL_SEC, ...opts }
  }

  // ── Public API ─────────────────────────────────────────────────────────────

  /**
   * Fetch the initial token. Must be called once before consume().
   * Exits the process if unable to reach the server after MAX_RENEWAL_ATTEMPTS.
   */
  async init(): Promise<void> {
    this.opts.log.info(`Fetching agent token from ${this.opts.orcaHttpUrl} ...`)
    this.current = await this.fetchWithRetry('initial', MAX_RENEWAL_ATTEMPTS)
    this.opts.log.info(
      `Token OK (ttl=${this.current.ttlSec}s). Starting agent (mode=direct-websocket)...`
    )
    tokenTracer.start({ op: 'init', devServerId: this.opts.devServerId, ttl: this.current.ttlSec }).ok()
    this.scheduleRenewal(this.current.ttlSec)
  }

  /**
   * Return the best available token for the next WS connection attempt.
   * If a pre-fetched renewal token is ready, it becomes current.
   * Call this just before each WebSocket connect.
   */
  consume(): string {
    if (this.next) {
      this.opts.log.info('[TokenManager] Using pre-fetched renewal token for reconnect')
      this.current = this.next
      this.next = null
      this.scheduleRenewal(this.current.ttlSec)
    }
    if (!this.current) {
      throw new Error('AgentTokenManager.consume() called before init()')
    }
    return this.current.token
  }

  /** Release timers. Call on process exit. */
  dispose(): void {
    this.disposed = true
    if (this.renewTimer) {
      clearTimeout(this.renewTimer)
      this.renewTimer = null
    }
  }

  /**
   * Force a synchronous renewal right now (bypassing the timer).
   * Useful when the server rejects the current token (e.g. after server restart).
   */
  async forceRenew(): Promise<void> {
    this.opts.log.info('[TokenManager] Force token renewal triggered')
    if (this.renewTimer) {
      clearTimeout(this.renewTimer)
      this.renewTimer = null
    }
    await this.doRenewal()
  }

  // ── Internals ──────────────────────────────────────────────────────────────

  private scheduleRenewal(ttlSec: number): void {
    if (this.renewTimer) {
      clearTimeout(this.renewTimer)
      this.renewTimer = null
    }
    if (this.disposed) return

    const delayMs = Math.floor(ttlSec * TOKEN_RENEW_RATIO * 1000)
    const renewAt = new Date(Date.now() + delayMs).toISOString()
    this.opts.log.info(
      `[TokenManager] Next token renewal in ${Math.round(delayMs / 60_000)}m (at ${renewAt})`
    )

    this.renewTimer = setTimeout(() => {
      this.renewTimer = null
      void this.doRenewal()
    }, delayMs)

    // Don't prevent process exit if only this timer is pending
    if (typeof (this.renewTimer as NodeJS.Timeout).unref === 'function') {
      (this.renewTimer as NodeJS.Timeout).unref()
    }
  }

  private async doRenewal(): Promise<void> {
    if (this.disposed) return
    this.opts.log.info('[TokenManager] Proactive token renewal starting...')
    const span = tokenTracer.start({ op: 'renew', devServerId: this.opts.devServerId })

    try {
      const fetched = await this.fetchWithRetry('renewal', MAX_RENEWAL_ATTEMPTS)
      this.next = fetched
      span.ok({ ttl: fetched.ttlSec })
      this.opts.log.info(
        `[TokenManager] Renewal token ready (ttl=${fetched.ttlSec}s). Will use on next reconnect.`
      )
      this.scheduleRenewal(fetched.ttlSec)
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err)
      span.fail(err, { devServerId: this.opts.devServerId })
      this.opts.log.warn(
        `[TokenManager] Renewal failed: ${msg}. Retrying in 5m with current token.`
      )
      // Schedule a retry soon without consuming the remaining TTL ratio
      this.scheduleRenewal(300) // 5 minutes
    }
  }

  private async fetchWithRetry(label: string, maxAttempts: number): Promise<FetchedToken> {
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
      try {
        return await this.fetchOnce()
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err)
        if (attempt >= maxAttempts) {
          if (label === 'initial') {
            this.opts.log.error(
              `FATAL: Cannot reach Orca Server after ${maxAttempts} attempts. Exit.`
            )
            process.exit(1)
          }
          throw new Error(`Token ${label} failed after ${maxAttempts} attempts: ${msg}`)
        }
        const delayMs = RETRY_DELAYS_MS[Math.min(attempt - 1, RETRY_DELAYS_MS.length - 1)]
        this.opts.log.warn(
          `[TokenManager] Token fetch failed (attempt ${attempt}/${maxAttempts}). ` +
          `Retry in ${delayMs / 1000}s... (${msg})`
        )
        await sleep(delayMs)
      }
    }
    throw new Error('fetchWithRetry: exhausted attempts')
  }

  private async fetchOnce(): Promise<FetchedToken> {
    const url  = `${this.opts.orcaHttpUrl}/api/agent-token`
    const body = JSON.stringify({
      devServerId: this.opts.devServerId,
      name:        this.opts.name,
      ttl:         this.opts.ttlSec,
      permanent:   true,
    })

    const res = await httpPost(url, body, {
      'Content-Type':  'application/json',
      'Authorization': `Bearer ${this.opts.apiSecret}`,
    }, 10_000)

    if (res.status === 401) throw new Error('Unauthorized — check ORCA_AGENT_API_SECRET')
    if (res.status !== 200) throw new Error(`HTTP ${res.status}: ${res.body.slice(0, 200)}`)

    let parsed: Record<string, unknown>
    try {
      parsed = JSON.parse(res.body) as Record<string, unknown>
    } catch {
      throw new Error(`Invalid JSON response: ${res.body.slice(0, 100)}`)
    }

    const token = parsed['token']
    if (typeof token !== 'string' || !token) {
      throw new Error(`No token in response: ${res.body.slice(0, 100)}`)
    }

    const ttlSec = typeof parsed['expiresIn'] === 'number'
      ? (parsed['expiresIn'] as number)
      : this.opts.ttlSec

    return { token, ttlSec, fetchedAt: Date.now() }
  }
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

interface HttpResult { status: number; body: string }

async function httpPost(
  url: string,
  body: string,
  headers: Record<string, string>,
  timeoutMs: number
): Promise<HttpResult> {
  const { request: httpRequest }  = await import('node:http')
  const { request: httpsRequest } = await import('node:https')
  const { URL: NodeURL }          = await import('node:url')

  return new Promise<HttpResult>((resolve, reject) => {
    const parsed    = new NodeURL(url)
    const isHttps   = parsed.protocol === 'https:'
    const doRequest = isHttps ? httpsRequest : httpRequest

    const req = doRequest(
      {
        hostname: parsed.hostname,
        port:     parsed.port || (isHttps ? 443 : 80),
        path:     parsed.pathname + parsed.search,
        method:   'POST',
        headers:  { ...headers, 'Content-Length': Buffer.byteLength(body) },
        rejectUnauthorized: process.env['NODE_TLS_REJECT_UNAUTHORIZED'] !== '0',
      },
      (res) => {
        let data = ''
        res.setEncoding('utf8')
        res.on('data', (chunk: string) => { data += chunk })
        res.on('end', () => resolve({ status: res.statusCode ?? 0, body: data }))
      }
    )

    req.setTimeout(timeoutMs, () => {
      req.destroy(new Error(`HTTP request timed out after ${timeoutMs}ms`))
    })
    req.on('error', reject)
    req.write(body)
    req.end()
  })
}
