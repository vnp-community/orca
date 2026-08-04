/**
 * ProviderResolver — Priority-based AI provider resolution (TDD-16)
 *
 * Resolution priority:
 * 1. User-scope accounts matching modelHint
 * 2. Project-scope accounts matching modelHint
 * 3. Server-scope accounts matching modelHint
 * 4. Repeat for each scope without modelHint filter
 * 5. Throw NO_PROVIDER_AVAILABLE if none found
 *
 * Quota check: account.quotaLimitDay === 0 (unlimited) OR tokens < quotaLimitDay
 *
 * @module main/ai-providers/ProviderResolver
 */

import type { AIProviderService } from './AIProviderService'
import type { AIProviderAccount, AIProviderScope } from '../../shared/ai-provider-types'
import { Tracers } from '../../shared/trace/tracers'

export interface ResolveOptions {
  devServerId: string
  projectId: string
  userId: string
  modelHint?: string
}

export class ProviderResolver {
  constructor(private readonly service: AIProviderService) {}

  /**
   * Resolve the best available AI provider account.
   *
   * Algorithm:
   * 1. Get all accounts for devServerId
   * 2. Filter: status === 'active'
   * 3. Filter: quota check (unlimited OR tokensUsed < quotaLimitDay)
   * 4. Apply priority + modelHint logic
   * 5. Throw NO_PROVIDER_AVAILABLE if nothing matches
   */
  async resolve(options: ResolveOptions): Promise<AIProviderAccount> {
    const { devServerId, projectId, userId, modelHint } = options
    const span = Tracers.aiProviderResolveFlow.start({ devServerId, projectId, userId, modelHint })

    try {
      // Fetch all accounts for this dev server
      const all = await this.service.listAccounts(devServerId)

      // Filter: active only
      const active = all.filter(a => a.status === 'active')

      // Filter: quota check (fetch today's usage in parallel for bounded accounts) —
      // rẽ nhánh quan trọng, có network hop ẩn trong getUsageToday(), đáng đo.
      const quotaCheckPromises = active
        .filter(a => a.quotaLimitDay > 0)
        .map(async (a) => {
          const usage = await this.service.getUsageToday(a.id)
          return { id: a.id, withinQuota: usage.tokens < a.quotaLimitDay }
        })

      const quotaResults = await Promise.all(quotaCheckPromises)
      const overQuotaIds = new Set(quotaResults.filter(r => !r.withinQuota).map(r => r.id))

      const available = active.filter(a =>
        a.quotaLimitDay === 0 || !overQuotaIds.has(a.id)
      )

      span.step('quota-filter', {
        totalAccounts: all.length,
        activeCount: active.length,
        overQuotaCount: overQuotaIds.size,
      })

      if (available.length === 0) {
        span.fail('NO_PROVIDER_AVAILABLE', { reason: 'quota-or-inactive' })
        throw new Error('NO_PROVIDER_AVAILABLE: no active AI provider accounts within quota')
      }

      // Priority resolution matrix: user > project > server, modelHint first then without
      const scopePriority: Array<{ scope: AIProviderScope; scopeRefId?: string }> = [
        { scope: 'user', scopeRefId: userId },
        { scope: 'project', scopeRefId: projectId },
        { scope: 'server' },
      ]

      // Pass 1: with modelHint
      if (modelHint) {
        for (const { scope, scopeRefId } of scopePriority) {
          const match = this.findInScope(available, scope, scopeRefId, modelHint)
          if (match) {
            span.step('scope-match', { matchedScope: scope, usedModelHint: true })
            span.ok({ accountId: match.id, scope })
            return match
          }
        }
      }

      // Pass 2: without modelHint (any model)
      for (const { scope, scopeRefId } of scopePriority) {
        const match = this.findInScope(available, scope, scopeRefId, undefined)
        if (match) {
          span.step('scope-match', { matchedScope: scope, usedModelHint: false })
          span.ok({ accountId: match.id, scope })
          return match
        }
      }

      span.fail('NO_PROVIDER_AVAILABLE', { reason: 'no-scope-match' })
      throw new Error('NO_PROVIDER_AVAILABLE: no matching AI provider account found')
    } catch (err) {
      // fail() đã được gọi ở 2 nhánh throw phía trên; nhánh này chỉ bắt lỗi hạ tầng
      // khác (vd. listAccounts()/getUsageToday() ném exception) — tránh double
      // span.fail() cho cùng 1 lỗi NO_PROVIDER_AVAILABLE đã trace.
      if (!(err instanceof Error) || !err.message.startsWith('NO_PROVIDER_AVAILABLE')) {
        span.fail(err)
      }
      throw err
    }
  }

  // ── private ────────────────────────────────────────────────────────────────

  private findInScope(
    accounts: AIProviderAccount[],
    scope: AIProviderScope,
    scopeRefId: string | undefined,
    modelHint: string | undefined
  ): AIProviderAccount | undefined {
    return accounts.find(a => {
      if (a.scope !== scope) return false
      if (scopeRefId !== undefined && a.scopeRefId !== scopeRefId) return false
      if (modelHint !== undefined && a.model !== modelHint) return false
      return true
    })
  }
}
