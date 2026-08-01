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

const HEALTH_CHECK_INTERVAL_MS = 15 * 60 * 1000 // 15 minutes

// ── Status change event ───────────────────────────────────────────────────────

export interface ProviderStatusChange {
  accountId:  string
  oldStatus:  string
  newStatus:  string
  checkedAt:  Date
}

// ── ProviderHealthChecker ─────────────────────────────────────────────────────

export class ProviderHealthChecker {
  private timer: ReturnType<typeof setInterval> | null = null

  /**
   * FIX BUG-AIP-003: Optional callback for status change alerts.
   * Wire this to WS push + webhook in server-bootstrap:
   *   checker.onStatusChanged = (e) => { wsServer.broadcast('provider:statusChanged', e); sendWebhook(e) }
   */
  onStatusChanged: ((event: ProviderStatusChange) => void) | null = null

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
      return
    }

    for (const account of accounts) {
      try {
        const oldStatus = account.status
        const result = await service.testConnection(account.id)

        let newStatus: 'active' | 'quota_exceeded' | 'invalid'
        if (result.ok) {
          newStatus = 'active'
        } else if (result.error?.toLowerCase().includes('quota')) {
          newStatus = 'quota_exceeded'
        } else {
          newStatus = 'invalid'
        }

        const checkedAt = new Date()
        await service.updateAccount(account.id, {
          status: newStatus,
          lastHealthCheck: checkedAt,
        })

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
      } catch (err) {
        console.warn(`[ProviderHealthChecker] Failed to check account ${account.id}:`, err)
      }
    }
  }
}
