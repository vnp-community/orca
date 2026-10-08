import { useCallback } from 'react'
import { useReviewEntryAvailability } from '@/components/review-map/entry/useReviewEntryAvailability'
import { useAgentTurnCompletions } from '@/components/review-map/entry/useAgentTurnCompletions'
import { openReviewFromEntryPoint } from '@/components/review-map/entry/open-review-entry'
import { useReviewedTurnId } from '@/components/review-map/entry/reviewed-turn-memory'
import { hasUnreviewedCompletion } from '@/components/review-map/entry/agent-turn-completion'

export type SourceControlReviewEntry = {
  visible: boolean
  open: () => void
  hasUnreviewed: boolean
}

/** Review entry for the Source Control header; the Source Control filter is deliberately not forwarded. */
export function useSourceControlReviewEntry({
  worktreeId
}: {
  worktreeId: string | null | undefined
}): SourceControlReviewEntry {
  const availability = useReviewEntryAvailability(worktreeId)
  const completions = useAgentTurnCompletions(worktreeId ?? null)
  const reviewedTurnId = useReviewedTurnId(worktreeId ?? null)
  const open = useCallback(() => {
    if (worktreeId) {
      openReviewFromEntryPoint(worktreeId, 'source-control')
    }
  }, [worktreeId])
  return {
    visible: availability.visible,
    open,
    hasUnreviewed: hasUnreviewedCompletion(completions, reviewedTurnId)
  }
}
