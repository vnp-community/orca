/**
 * ProviderHealthChecker — Background cron for AI provider health checks (TDD-16)
 *
 * Runs immediately when started, then every 15 minutes.
 * Checks all accounts via service.testConnection() and updates status.
 * Non-fatal: errors are logged, never thrown.
 *
 * FIX BUG-AIP-003: Emits 'statusChanged' event when account status changes.
 *   Wire onStatusChanged callback to send WS push / webhook alerts.
 * FIX BUG-AIP-004: Remove unused _relayPool parameter — service already
 *   manages relay internally via AIProviderService.
 *
 * Status mapping:
 * - ok: true → 'active'
 * - error contains 'quota' → 'quota_exceeded'
 * - otherwise → 'invalid'
 *
 * @module main/ai-providers/ProviderHealthChecker
 */

import type { AIProviderService } from './AIProviderService'
import { Tracers } from '../../shared/trace/tracers'

const HEALTH_CHECK_INTERVAL_MS = 15 * 60 * 1000 // 15 minutes
const QUOTA_ALERT_THRESHOLD_RATIO = 0.8 // BUG-BE-HLD-015: warn at 80% of quotaLimitDay

// ── Status change event ───────────────────────────────────────────────────────

export interface ProviderStatusChange {
  accountId:  string
  oldStatus:  string
  newStatus:  string
  checkedAt:  Date
}

// BUG-BE-HLD-015: emitted the first time an account crosses 80% of its daily quota.
export interface ProviderQuotaWarning {
  accountId:    string
  tokensUsed:   number
  quotaLimitDay: number
  ratio:        number
  checkedAt:    Date
}

// ── ProviderHealthChecker ─────────────────────────────────────────────────────

export class ProviderHealthChecker {
  private timer: ReturnType<typeof setInterval> | null = null
  // BUG-BE-HLD-015: debounce — one warning per account per calendar day so the
  // 15-minute cron doesn't re-alert every cycle while still above threshold.
  private readonly quotaWarnedOn = new Map<string, string>() // accountId -> 'YYYY-MM-DD'

  /**
   * FIX BUG-AIP-003: Optional callback for status change alerts.
   * Wire this to WS push + webhook in server-bootstrap:
   *   checker.onStatusChanged = (e) => { wsServer.broadcast('provider:statusChanged', e); sendWebhook(e) }
   */
  onStatusChanged: ((event: ProviderStatusChange) => void) | null = null

  /**
   * BUG-BE-HLD-015: optional callback for early quota warnings.
   * Wire this in server-bootstrap next to onStatusChanged:
   *   checker.onQuotaWarning = (e) => { wsServer.broadcast('provider:quotaWarning', e); sendWebhook(e) }
   */
  onQuotaWarning: ((event: ProviderQuotaWarning) => void) | null = null

  /**
   * Start the health checker.
   * FIX BUG-AIP-004: Removed unused relayPool param — service manages relay internally.
   * Runs one check immediately, then every 15 minutes.
   */
  start(service: AIProviderService): void {
    // Run immediately
    void this.runCheck(service)
    // Then on interval
    this.timer = setInterval(
      () => void this.runCheck(service),
      HEALTH_CHECK_INTERVAL_MS
    )
  }

  /**
   * Stop the health checker and clear the interval.
   */
  stop(): void {
    if (this.timer) {
      clearInterval(this.timer)
      this.timer = null
    }
  }

  /**
   * Run a health check on all accounts.
   * Non-fatal — errors per-account are caught and logged.
   * FIX BUG-AIP-003: Detects status transitions and calls onStatusChanged.
   */
  private async runCheck(service: AIProviderService): Promise<void> {
    let accounts
    try {
      accounts = await service.getAllAccounts()
    } catch (err) {
      console.warn('[ProviderHealthChecker] Failed to fetch accounts:', err)
      return // no span — nothing to measure if the account list itself never loaded
    }

    const span = Tracers.aiProviderHealthFlow.start({ accountCount: accounts.length })
    let activeCount = 0
    let quotaExceededCount = 0
    let invalidCount = 0
    let errorCount = 0

    for (const account of accounts) {
      try {
        // BUG-BE-HLD-014: an account mid key-rotation keeps its real credential
        // until completeRotation() commits — a normal connectivity ping here
        // would still succeed (old key) and must NOT flip status away from
        // 'rotating'. Use this cron cycle only as crash-recovery: if the
        // grace period already elapsed (rotateKey()'s setTimeout was lost to
        // a restart), finish the commit now.
        if (account.status === 'rotating') {
          if (account.rotationGraceUntil && account.rotationGraceUntil.getTime() <= Date.now()) {
            span.step('rotation-recovery', { accountId: account.id })
            await service.completeRotation(account.id).catch((err) =>
              console.warn(`[ProviderHealthChecker] completeRotation recovery failed for ${account.id}:`, err)
            )
          }
          continue
        }

        const oldStatus = account.status
        span.step('ping-account', { accountId: account.id, provider: account.provider })
        const result = await service.testConnection(account.id)

        let newStatus: 'active' | 'quota_exceeded' | 'invalid'
        if (result.ok) {
          newStatus = 'active'
        } else if (result.error?.toLowerCase().includes('quota')) {
          newStatus = 'quota_exceeded'
        } else {
          newStatus = 'invalid'
        }
        span.step('ping-result', {
          accountId: account.id,
          ok: result.ok,
          latencyMs: result.latencyMs,
          newStatus,
        })

        const checkedAt = new Date()
        await service.updateAccount(account.id, {
          status: newStatus,
          lastHealthCheck: checkedAt,
        })

        if (newStatus === 'active') activeCount++
        else if (newStatus === 'quota_exceeded') quotaExceededCount++
        else invalidCount++

        // FIX BUG-AIP-003: Emit status change event when status transitions
        if (oldStatus !== newStatus && this.onStatusChanged) {
          console.log(`[ProviderHealthChecker] Account ${account.id}: ${oldStatus} → ${newStatus}`)
          this.onStatusChanged({
            accountId: account.id,
            oldStatus,
            newStatus,
            checkedAt,
          })
        }

        // BUG-BE-HLD-015: proactive 80% quota warning, independent of the
        // reactive quota_exceeded status above (which only fires once the
        // provider itself has already started rejecting requests).
        if (account.quotaLimitDay > 0) {
          const usage = await service.getUsageToday(account.id)
          const ratio = usage.tokens / account.quotaLimitDay
          const today = checkedAt.toISOString().slice(0, 10)
          if (ratio >= QUOTA_ALERT_THRESHOLD_RATIO && this.quotaWarnedOn.get(account.id) !== today) {
            this.quotaWarnedOn.set(account.id, today)
            span.step('quota-warning', { accountId: account.id, ratio, tokensUsed: usage.tokens })
            this.onQuotaWarning?.({
              accountId: account.id,
              tokensUsed: usage.tokens,
              quotaLimitDay: account.quotaLimitDay,
              ratio,
              checkedAt,
            })
          } else if (ratio < QUOTA_ALERT_THRESHOLD_RATIO) {
            this.quotaWarnedOn.delete(account.id) // usage dropped (new day) — allow re-alert later
          }
        }
      } catch (err) {
        // Per-account failure — does NOT fail the whole cycle (existing behavior,
        // see CR-TRACE-016 §4). Counted separately from provider-reported invalid.
        errorCount++
        console.warn(`[ProviderHealthChecker] Failed to check account ${account.id}:`, err)
      }
    }

    span.ok({ activeCount, quotaExceededCount, invalidCount, errorCount })
  }
}
