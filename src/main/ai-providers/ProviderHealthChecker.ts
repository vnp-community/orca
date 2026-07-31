/**
 * ProviderHealthChecker — Background cron for AI provider health checks (TDD-16)
 *
 * Runs immediately when started, then every 15 minutes.
 * Checks all accounts via service.testConnection() and updates status.
 * Non-fatal: errors are logged, never thrown.
 *
 * Status mapping:
 * - ok: true → 'active'
 * - error contains 'quota' → 'quota_exceeded'
 * - otherwise → 'invalid'
 *
 * @module main/ai-providers/ProviderHealthChecker
 */

import type { AIProviderService } from './AIProviderService'
import type { RelayConnectionPool } from '../dev-server/relay-connection-pool'

const HEALTH_CHECK_INTERVAL_MS = 15 * 60 * 1000 // 15 minutes

export class ProviderHealthChecker {
  private timer: ReturnType<typeof setInterval> | null = null

  /**
   * Start the health checker.
   * Runs one check immediately, then every 15 minutes.
   */
  start(service: AIProviderService, relayPool: RelayConnectionPool): void {
    // Run immediately
    void this.runCheck(service, relayPool)
    // Then on interval
    this.timer = setInterval(
      () => void this.runCheck(service, relayPool),
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
   */
  private async runCheck(service: AIProviderService, _relayPool: RelayConnectionPool): Promise<void> {
    let accounts
    try {
      accounts = await service.getAllAccounts()
    } catch (err) {
      console.warn('[ProviderHealthChecker] Failed to fetch accounts:', err)
      return
    }

    for (const account of accounts) {
      try {
        const result = await service.testConnection(account.id)

        let newStatus: 'active' | 'quota_exceeded' | 'invalid'
        if (result.ok) {
          newStatus = 'active'
        } else if (result.error?.toLowerCase().includes('quota')) {
          newStatus = 'quota_exceeded'
        } else {
          newStatus = 'invalid'
        }

        await service.updateAccount(account.id, {
          status: newStatus,
          lastHealthCheck: new Date(),
        })
      } catch (err) {
        console.warn(`[ProviderHealthChecker] Failed to check account ${account.id}:`, err)
      }
    }
  }
}
