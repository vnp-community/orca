import type { StateCreator } from 'zustand'
import type {
  CodexUsageBreakdownRow,
  CodexUsageDailyPoint,
  CodexUsageRange,
  CodexUsageScanState,
  CodexUsageScope,
  CodexUsageSessionRow,
  CodexUsageSnapshot,
  CodexUsageSummary
} from '../../../../shared/codex-usage-types'
import type { AppState } from '../types'
import {
  getCodexUsageScanState as rpcGetCodexUsageScanState,
  getCodexUsageSnapshot as rpcGetCodexUsageSnapshot,
  refreshCodexUsage as rpcRefreshCodexUsage,
  setCodexUsageEnabled as rpcSetCodexUsageEnabled
} from '../../runtime/runtime-codex-usage-client'

export type CodexUsageSlice = {
  codexUsageScope: CodexUsageScope
  codexUsageRange: CodexUsageRange
  codexUsageScanState: CodexUsageScanState | null
  codexUsageSummary: CodexUsageSummary | null
  codexUsageDaily: CodexUsageDailyPoint[]
  codexUsageModelBreakdown: CodexUsageBreakdownRow[]
  codexUsageProjectBreakdown: CodexUsageBreakdownRow[]
  codexUsageRecentSessions: CodexUsageSessionRow[]
  setCodexUsageEnabled: (enabled: boolean) => Promise<void>
  setCodexUsageScope: (scope: CodexUsageScope) => Promise<void>
  setCodexUsageRange: (range: CodexUsageRange) => Promise<void>
  fetchCodexUsage: (opts?: { forceRefresh?: boolean }) => Promise<void>
  enableCodexUsage: () => Promise<void>
  refreshCodexUsage: () => Promise<void>
}

export const createCodexUsageSlice: StateCreator<AppState, [], [], CodexUsageSlice> = (
  set,
  get
) => ({
  codexUsageScope: 'orca',
  codexUsageRange: '30d',
  codexUsageScanState: null,
  codexUsageSummary: null,
  codexUsageDaily: [],
  codexUsageModelBreakdown: [],
  codexUsageProjectBreakdown: [],
  codexUsageRecentSessions: [],

  setCodexUsageEnabled: async (enabled) => {
    try {
      const nextScanState = (await rpcSetCodexUsageEnabled(enabled)) as CodexUsageScanState
      set({
        codexUsageScanState: enabled
          ? {
              ...nextScanState,
              isScanning: true,
              lastScanCompletedAt: null,
              lastScanError: null
            }
          : nextScanState,
        codexUsageSummary: null,
        codexUsageDaily: [],
        codexUsageModelBreakdown: [],
        codexUsageProjectBreakdown: [],
        codexUsageRecentSessions: []
      })
      if (enabled) {
        await get().fetchCodexUsage({ forceRefresh: true })
      }
    } catch (error) {
      console.error('Failed to update Codex usage setting:', error)
    }
  },

  setCodexUsageScope: async (scope) => {
    set({ codexUsageScope: scope })
    await get().fetchCodexUsage()
  },

  setCodexUsageRange: async (range) => {
    set({ codexUsageRange: range })
    await get().fetchCodexUsage()
  },

  fetchCodexUsage: async (opts) => {
    try {
      const scanState = (await rpcGetCodexUsageScanState()) as CodexUsageScanState
      const currentScanState = get().codexUsageScanState
      const shouldPreserveLoadingState =
        opts?.forceRefresh === true &&
        currentScanState?.enabled === true &&
        get().codexUsageSummary === null
      set({
        codexUsageScanState: shouldPreserveLoadingState
          ? {
              ...scanState,
              isScanning: true,
              lastScanCompletedAt: null,
              lastScanError: null
            }
          : scanState
      })
      if (!scanState.enabled) {
        return
      }

      const { codexUsageScope, codexUsageRange } = get()
      const snapshot = (await rpcGetCodexUsageSnapshot(
        codexUsageScope,
        codexUsageRange,
        10
      )) as CodexUsageSnapshot
      const hasCachedSnapshot =
        snapshot.scanState.lastScanCompletedAt !== null || snapshot.scanState.hasAnyCodexData

      if (hasCachedSnapshot) {
        set({
          codexUsageScanState:
            opts?.forceRefresh === true
              ? { ...snapshot.scanState, isScanning: true }
              : snapshot.scanState,
          codexUsageSummary: snapshot.summary,
          codexUsageDaily: snapshot.daily,
          codexUsageModelBreakdown: snapshot.modelBreakdown,
          codexUsageProjectBreakdown: snapshot.projectBreakdown,
          codexUsageRecentSessions: snapshot.recentSessions
        })
      } else {
        set({
          codexUsageScanState: {
            ...scanState,
            isScanning: true,
            lastScanError: null
          }
        })
      }

      await rpcRefreshCodexUsage(opts?.forceRefresh ?? false)
      const { codexUsageScope: refreshedScope, codexUsageRange: refreshedRange } = get()
      const refreshedSnapshot = (await rpcGetCodexUsageSnapshot(
        refreshedScope,
        refreshedRange,
        10
      )) as CodexUsageSnapshot

      set({
        codexUsageScanState: refreshedSnapshot.scanState,
        codexUsageSummary: refreshedSnapshot.summary,
        codexUsageDaily: refreshedSnapshot.daily,
        codexUsageModelBreakdown: refreshedSnapshot.modelBreakdown,
        codexUsageProjectBreakdown: refreshedSnapshot.projectBreakdown,
        codexUsageRecentSessions: refreshedSnapshot.recentSessions
      })
    } catch (error) {
      console.error('Failed to fetch Codex usage:', error)
    }
  },

  enableCodexUsage: async () => {
    await get().setCodexUsageEnabled(true)
  },

  refreshCodexUsage: async () => {
    await get().fetchCodexUsage({ forceRefresh: true })
  }
})
