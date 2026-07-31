// useAIProviders.ts — Hook for AI provider account management (TDD-FE-13, TASK-FE-009)
// TASK-FE-009: Added getActiveRuntimeTarget, scope/status filtering, backward-compat devServerId string arg
import { useCallback, useEffect, useMemo } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { AIProviderAccount, AIProviderStatus } from '../types/ai-provider-types'

export interface AIProvidersFilter {
  devServerId?: string
  scope?:       'server' | 'project' | 'user'
  status?:      AIProviderStatus
}

// Overloads — support both old string API and new filter object API
export function useAIProviders(devServerIdOrFilter?: string | AIProvidersFilter) {
  // Normalise arg — string shorthand maps to filter.devServerId
  const filter: AIProvidersFilter = typeof devServerIdOrFilter === 'string'
    ? { devServerId: devServerIdOrFilter }
    : devServerIdOrFilter ?? {}

  // Support both old field names (aiAccounts) and new (accounts) for compatibility
  const allAccounts = useAppStore(s =>
    ((s as any).aiAccounts ?? (s as any).accounts ?? []) as AIProviderAccount[]
  )

  const isLoadingAccounts = useAppStore(s =>
    (s as any).isLoadingAIAccounts ?? (s as any).isLoadingAccounts ?? false
  ) as boolean

  // Filter accounts client-side by devServerId, scope, status
  const accounts = useMemo(() => {
    return allAccounts.filter(account => {
      if (filter.devServerId && account.devServerId !== filter.devServerId) return false
      if (filter.scope       && account.scope       !== filter.scope)       return false
      if (filter.status      && account.status      !== filter.status)      return false
      return true
    })
  }, [allAccounts, filter.devServerId, filter.scope, filter.status])

  const refresh = useCallback(async () => {
    const store           = useAppStore.getState() as any
    const setAccounts     = store.setAIAccounts ?? store.setAccounts
    const setLoadingAccts = store.setLoadingAIAccounts ?? store.setLoadingAccounts

    if (setLoadingAccts) setLoadingAccts(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const result = await callRuntimeRpc<AIProviderAccount[]>(
        target, 'aiProvider.list', { devServerId: filter.devServerId }
      )
      if (setAccounts) setAccounts(result)
    } finally {
      if (setLoadingAccts) setLoadingAccts(false)
    }
  }, [filter.devServerId])

  useEffect(() => { refresh() }, [refresh])

  const testConnection = useCallback(async (accountId: string) => {
    const target       = getActiveRuntimeTarget(useAppStore.getState().settings)
    const store        = useAppStore.getState() as any
    const updateStatus = store.updateAIAccountStatus ?? store.updateAccountStatus
    try {
      const result = await callRuntimeRpc<{
        ok: boolean; latencyMs: number; error?: string
      }>(target, 'aiProvider.testConnection', { accountId })

      // TDD-FE-13: use 'healthy' on success (store compatible with 'active' fallback in slice)
      if (updateStatus) updateStatus(accountId, result.ok ? 'healthy' : 'invalid')
      return result
    } catch {
      if (updateStatus) updateStatus(accountId, 'invalid')
      throw new Error('Connection test failed')
    }
  }, [])

  const deleteAccount = useCallback(async (accountId: string) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'aiProvider.delete', { accountId })
    const store  = useAppStore.getState() as any
    const remove = store.removeAIAccount ?? store.removeAccount
    if (remove) remove(accountId)
  }, [])

  const createAccount = useCallback(async (
    payload: Omit<AIProviderAccount, 'id' | 'status' | 'createdAt'>
  ) => {
    const target  = getActiveRuntimeTarget(useAppStore.getState().settings)
    const created = await callRuntimeRpc<AIProviderAccount>(target, 'aiProvider.create', payload)
    const store   = useAppStore.getState() as any
    const addFn   = store.addAIAccount ?? store.addAccount
    if (addFn) addFn(created)
    return created
  }, [])

  const updateAccount = useCallback(async (accountId: string, patch: Partial<AIProviderAccount>) => {
    const target  = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'aiProvider.update', { accountId, ...patch })
    const store   = useAppStore.getState() as any
    const patchFn = store.updateAIAccount ?? store.updateAccount
    if (patchFn) patchFn(accountId, patch)
  }, [])

  return {
    accounts,
    isLoading: isLoadingAccounts,
    refresh,
    testConnection,
    deleteAccount,
    createAccount,
    updateAccount,
  }
}
