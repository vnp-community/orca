/**
 * Links from the Outcome Review into the code review — FE-OR-T321, T322
 *
 * Which worktrees the review can open, whether the code review is on, and how to open it. The code review (v7) opens by
 * worktree, not by Request, so the worktrees come from the tasks the review's heads were made from. A file chip can only go to the
 * impact view of that worktree: the code review has no way to be pointed at one file's diff from outside.
 *
 * @module components/request/outcome-review/decision/use-outcome-review-links
 */

import { useCallback, useMemo } from 'react'
import { useAppStore } from '@/store'
import { openReviewFromEntryPoint } from '../../../review-map/entry/open-review-entry'
import { useReviewWorktrees } from '../../../../hooks/useReviewWorktrees'
import { worktreesOf, type CodeReviewLens } from './outcome-code-review-link'
import type { CodeReviewLink } from './OutcomeCodeReviewButton'
import type { OutcomeReview } from '../../../../../../shared/outcome-review-types'
import type { OrcaRequest } from '../../../../../../shared/request-types'

export function useOutcomeReviewLinks(
  request: OrcaRequest,
  review: OutcomeReview | null,
  enabled: boolean
): { link: CodeReviewLink; openFile?: (path: string) => void } {
  const codeReviewEnabled = useAppStore((s) => s.codeIntelSupportState.state === 'enabled')
  const { taskWorktrees } = useReviewWorktrees(request, enabled && review !== null)
  const worktrees = useMemo(
    () => (review ? worktreesOf(review, taskWorktrees) : []),
    [review, taskWorktrees]
  )
  const open = useCallback((worktreeId: string, lens: CodeReviewLens) => {
    openReviewFromEntryPoint(worktreeId, 'request-outcome', { lens })
  }, [])
  const link = useMemo<CodeReviewLink>(
    () => ({ enabled: codeReviewEnabled, worktrees, open }),
    [codeReviewEnabled, worktrees, open]
  )
  const only = worktrees.length === 1 ? worktrees[0] : null
  const openFile = codeReviewEnabled && only ? () => open(only.worktreeId, 'impact') : undefined
  return { link, openFile }
}
