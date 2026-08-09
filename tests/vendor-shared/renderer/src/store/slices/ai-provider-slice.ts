// ai-provider-slice.ts — AI Provider account state (TDD-FE-13)
import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { AIProviderAccount, AIProviderUsage, AIProviderStatus } from '../../types/ai-provider-types'

// ─── Types ────────────────────────────────────────────────────────────────────

export type AIProviderSlice = {
  /** All AI provider accounts (across servers/projects/users) */
  aiAccounts: AIProviderAccount[]
  /** Today's usage keyed by account id */
  aiUsageByAccount: Record<string, AIProviderUsage>
  /** Loading state for account list */
  isLoadingAIAccounts: boolean

  setAIAccounts: (accounts: AIProviderAccount[]) => void
  addAIAccount: (account: AIProviderAccount) => void
  removeAIAccount: (id: string) => void
  updateAIAccountStatus: (id: string, status: AIProviderStatus) => void
  updateAIAccount: (id: string, patch: Partial<AIProviderAccount>) => void
  setAIUsage: (accountId: string, usage: AIProviderUsage) => void
  setLoadingAIAccounts: (v: boolean) => void
}

// ─── Slice Factory ────────────────────────────────────────────────────────────

export const createAIProviderSlice: StateCreator<AppState, [], [], AIProviderSlice> = (set) => ({
  aiAccounts:          [],
  aiUsageByAccount:    {},
  isLoadingAIAccounts: false,

  setAIAccounts: (accounts) =>
    set(() => ({ aiAccounts: accounts })),

  addAIAccount: (account) =>
    set((state) => ({ aiAccounts: [...state.aiAccounts, account] })),

  removeAIAccount: (id) =>
    set((state) => ({ aiAccounts: state.aiAccounts.filter(a => a.id !== id) })),

  updateAIAccountStatus: (id, status) =>
    set((state) => ({
      aiAccounts: state.aiAccounts.map(a => a.id === id ? { ...a, status } : a)
    })),

  updateAIAccount: (id, patch) =>
    set((state) => ({
      aiAccounts: state.aiAccounts.map(a => a.id === id ? { ...a, ...patch } : a)
    })),

  setAIUsage: (accountId, usage) =>
    set((state) => ({
      aiUsageByAccount: { ...state.aiUsageByAccount, [accountId]: usage }
    })),

  setLoadingAIAccounts: (v) =>
    set(() => ({ isLoadingAIAccounts: v })),
})
