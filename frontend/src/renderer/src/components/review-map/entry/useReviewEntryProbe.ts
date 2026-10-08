import { useAppStore } from '@/store'
import { useCodeIntelSelector } from '@/lib/code-intel-worktree-selector'
import { useCodeIntelSupport } from '@/hooks/useCodeIntelSupport'

/** Mounted once (right sidebar) so entry points know the flag without any Review tab open. */
export function useReviewEntryProbe(): void {
  const activeWorktreeId = useAppStore((s) => s.activeWorktreeId)
  const selector = useCodeIntelSelector(activeWorktreeId ?? '')
  useCodeIntelSupport({
    worktreeId: activeWorktreeId && selector.state === 'ready' ? selector.worktreeId : null,
    environmentId: selector.state === 'ready' ? selector.environmentId : null,
    isReviewTabOpen: true
  })
}
