/**
 * use-storage-lens-probe.ts — FE-CV-TASK-058-03
 *
 * Asks the `storage` channel once when Review opens, so a backend that cannot serve it hides the
 * Storage tab before anyone clicks it. Same params as StorageLens's first load, so the answer
 * is the lens's cached data rather than an extra request later.
 *
 * @module components/review-map/shell/use-storage-lens-probe
 */

import { useEffect } from 'react'
import { useAppStore } from '@/store'
import { useCodeIntelStorage } from '../../../hooks/useCodeIntelStorage'
import { setReviewLensUnavailable } from './review-lens-availability'

export function useStorageLensProbe(
  worktreeId: string,
  environmentId: string | null,
  enabled: boolean
): void {
  const env = useAppStore((s) => s.reviewUiByWorktree[worktreeId]?.storageEnv ?? 'dev')
  const probe = useCodeIntelStorage({ worktreeId, environmentId, env, enabled })
  const unavailable = enabled && !probe.available
  useEffect(() => {
    // Only hides: the lens itself re-shows the tab when a later load succeeds.
    if (unavailable) {
      setReviewLensUnavailable(worktreeId, 'storage', true)
    }
  }, [worktreeId, unavailable])
}
