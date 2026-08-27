import type { StateCreator } from 'zustand'
import type { RateLimitRuntimeTarget, RateLimitState } from '../../../../shared/rate-limit-types'
import type { AppState } from '../types'
import {
  consumeCodexRateLimitResetCredit as rpcConsumeCodexRateLimitResetCredit,
  fetchInactiveClaudeRateLimitAccounts as rpcFetchInactiveClaudeRateLimitAccounts,
  fetchInactiveCodexRateLimitAccounts as rpcFetchInactiveCodexRateLimitAccounts,
  getRateLimitState as rpcGetRateLimitState,
  refreshClaudeRateLimitsForTarget as rpcRefreshClaudeRateLimitsForTarget,
  refreshCodexRateLimitsForTarget as rpcRefreshCodexRateLimitsForTarget,
  refreshGrokRateLimits as rpcRefreshGrokRateLimits,
  refreshRateLimits as rpcRefreshRateLimits
} from '../../runtime/runtime-rate-limits-client'

export type RateLimitSlice = {
  rateLimits: RateLimitState
  fetchRateLimits: () => Promise<void>
  refreshRateLimits: () => Promise<void>
  refreshGrokRateLimits: () => Promise<void>
  refreshClaudeRateLimitsForTarget: (target: RateLimitRuntimeTarget) => Promise<void>
  refreshCodexRateLimitsForTarget: (target: RateLimitRuntimeTarget) => Promise<void>
  consumeCodexRateLimitResetCredit: () => Promise<void>
  fetchInactiveClaudeAccountUsage: () => Promise<void>
  fetchInactiveCodexAccountUsage: () => Promise<void>
  setRateLimitsFromPush: (state: RateLimitState) => void
}

export const createRateLimitSlice: StateCreator<AppState, [], [], RateLimitSlice> = (set, get) => ({
  rateLimits: {
    claude: null,
    codex: null,
    gemini: null,
    opencodeGo: null,
    kimi: null,
    antigravity: null,
    minimax: null,
    grok: null,
    minimaxCookieConfigured: false,
    grokAuthConfigured: false,
    claudeTarget: { runtime: 'host', wslDistro: null },
    codexTarget: { runtime: 'host', wslDistro: null },
    inactiveClaudeAccounts: [],
    inactiveCodexAccounts: []
  },

  fetchRateLimits: async () => {
    try {
      const state = await rpcGetRateLimitState()
      set({ rateLimits: state })
    } catch (error) {
      console.error('Failed to fetch rate limits:', error)
    }
  },

  refreshRateLimits: async () => {
    try {
      const state = await rpcRefreshRateLimits()
      set({ rateLimits: state })
    } catch (error) {
      console.error('Failed to refresh rate limits:', error)
    }
  },

  refreshGrokRateLimits: async () => {
    try {
      const state = await rpcRefreshGrokRateLimits()
      set({ rateLimits: state })
    } catch (error) {
      console.error('Failed to refresh Grok usage:', error)
    }
  },

  refreshClaudeRateLimitsForTarget: async (target) => {
    const current = get().rateLimits
    const targetChanged =
      current.claudeTarget.runtime !== target.runtime ||
      current.claudeTarget.wslDistro !== target.wslDistro
    set({
      rateLimits: {
        ...current,
        claudeTarget: target,
        claude:
          current.claude && !targetChanged
            ? { ...current.claude, status: 'fetching' }
            : {
                provider: 'claude',
                session: null,
                weekly: null,
                updatedAt: 0,
                error: null,
                status: 'fetching'
              }
      }
    })
    try {
      const state = await rpcRefreshClaudeRateLimitsForTarget(target)
      set({ rateLimits: state })
    } catch (error) {
      console.error('Failed to refresh Claude usage for runtime:', error)
    }
  },

  refreshCodexRateLimitsForTarget: async (target) => {
    const current = get().rateLimits
    const targetChanged =
      current.codexTarget.runtime !== target.runtime ||
      current.codexTarget.wslDistro !== target.wslDistro
    set({
      rateLimits: {
        ...current,
        codexTarget: target,
        codex:
          current.codex && !targetChanged
            ? { ...current.codex, status: 'fetching' }
            : {
                provider: 'codex',
                session: null,
                weekly: null,
                updatedAt: 0,
                error: null,
                status: 'fetching'
              }
      }
    })
    try {
      const state = await rpcRefreshCodexRateLimitsForTarget(target)
      set({ rateLimits: state })
    } catch (error) {
      console.error('Failed to refresh Codex usage for runtime:', error)
    }
  },

  consumeCodexRateLimitResetCredit: async () => {
    try {
      const result = await rpcConsumeCodexRateLimitResetCredit()
      set({ rateLimits: result.state })
    } catch (error) {
      console.error('Failed to consume Codex rate-limit reset:', error)
      throw error
    }
  },

  fetchInactiveClaudeAccountUsage: async () => {
    try {
      await rpcFetchInactiveClaudeRateLimitAccounts()
    } catch (error) {
      console.error('Failed to fetch inactive Claude account usage:', error)
    }
  },

  fetchInactiveCodexAccountUsage: async () => {
    try {
      await rpcFetchInactiveCodexRateLimitAccounts()
    } catch (error) {
      console.error('Failed to fetch inactive Codex account usage:', error)
    }
  },

  setRateLimitsFromPush: (state) => {
    set({ rateLimits: state })
  }
})
