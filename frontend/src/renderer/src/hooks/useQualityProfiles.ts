/**
 * useQualityProfiles.ts — FE-CV-TASK-087-03
 *
 * Runnable profiles and the selected profile of a worktree. The selection lives in the quality
 * UI state; `selectDefaultProfile` keeps it valid when the runnable list changes.
 *
 * @module hooks/useQualityProfiles
 */

import { useCallback, useEffect, useMemo } from 'react'
import { useAppStore } from '@/store'
import { selectDefaultProfile } from '../components/review-map/quality/quality-profile-selection'
import type { RunnableProfile, QualityProfile } from '../../../shared/code-intel-quality-types'
import { useQualitySupport } from './useQualitySupport'

const NO_PROFILES: readonly RunnableProfile[] = []

export type UseQualityProfilesResult = {
  runnable: readonly RunnableProfile[]
  profile: QualityProfile | null
  selected: string | null
  selectedProfile: RunnableProfile | null
  select: (name: string) => void
  refetch: () => void
  status: 'idle' | 'loading' | 'ready' | 'error'
}

export function useQualityProfiles(
  worktreeId: string | null | undefined,
  gateProfileRef: string | null = null
): UseQualityProfilesResult {
  const support = useQualitySupport(worktreeId)
  const enabled = support === 'enabled' && Boolean(worktreeId)
  const entry = useAppStore((s) =>
    worktreeId ? s.codeIntelQualityByWorktree[worktreeId]?.profiles : null
  )
  const uiProfile = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.ui.profile ?? null) : null
  )
  const loading = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.loading.profiles ?? false) : false
  )
  const failed = useAppStore((s) =>
    worktreeId ? Boolean(s.codeIntelQualityByWorktree[worktreeId]?.errors.profiles) : false
  )

  useEffect(() => {
    if (enabled && worktreeId) {
      void useAppStore.getState().loadQualityProfiles(worktreeId)
    }
  }, [enabled, worktreeId])

  const runnable = enabled ? (entry?.data.runnableProfiles ?? NO_PROFILES) : NO_PROFILES
  const profile = enabled ? (entry?.data.profile ?? null) : null
  const selected = useMemo(
    () =>
      selectDefaultProfile({
        uiProfile,
        gateProfileRef,
        configuredName: profile?.name ?? null,
        runnable
      }),
    [uiProfile, gateProfileRef, profile, runnable]
  )
  const selectedProfile = useMemo(
    () => runnable.find((p) => p.id === selected) ?? null,
    [runnable, selected]
  )

  const select = useCallback(
    (name: string) => {
      if (worktreeId) {
        useAppStore.getState().setQualityUi(worktreeId, { profile: name })
      }
    },
    [worktreeId]
  )
  const refetch = useCallback(() => {
    if (enabled && worktreeId) {
      void useAppStore.getState().loadQualityProfiles(worktreeId, { force: true })
    }
  }, [enabled, worktreeId])

  let status: UseQualityProfilesResult['status'] = 'idle'
  if (enabled) {
    status = entry ? 'ready' : failed ? 'error' : loading ? 'loading' : 'idle'
  }
  return { runnable, profile, selected, selectedProfile, select, refetch, status }
}
