/**
 * Outcome Review → code review link — FE-OR-T320, T322
 *
 * Which lens of the code review (v7) goes with a section of the review, and whether the code review can be opened at all for
 * this Request. Pure. The lens ids are the ones the code review registers (see review-lens-registry); a section with no lens
 * (Behavior, Config) has no button rather than a button to somewhere unrelated, and "requirements" is not a registered lens.
 *
 * @module components/request/outcome-review/decision/outcome-code-review-link
 */

import type {
  OutcomeReview,
  ReviewItem,
  ReviewSection
} from '../../../../../../shared/outcome-review-types'

export type CodeReviewLens = 'architecture' | 'dataflow' | 'erd' | 'contract' | 'impact'

export function lensForSection(section: ReviewSection | string): CodeReviewLens | null {
  switch (section) {
    case 'architecture':
      return 'architecture'
    case 'flow':
      return 'dataflow'
    case 'data':
      return 'erd'
    case 'contracts':
      return 'contract'
    default:
      return null
  }
}

/** A row that names a changed file opens the impact of that change; otherwise its section decides. */
export function lensForItem(item: ReviewItem): CodeReviewLens | null {
  return item.evidence.some((e) => e.kind === 'file') ? 'impact' : lensForSection(item.section)
}

/** A container opens the architecture lens: the lens cannot be narrowed to one container, so this is where the person starts. */
export const CONTAINER_LENS: CodeReviewLens = 'architecture'

export type Worktree = { taskId: string; worktreeId: string; branch: string }

/** The worktrees of the tasks the review was made from, once each, in the order the review lists them. */
export function worktreesOf(
  review: Pick<OutcomeReview, 'model'>,
  taskWorktrees: ReadonlyMap<string, string>
): Worktree[] {
  const seen = new Set<string>()
  const out: Worktree[] = []
  for (const head of review.model.subject.heads) {
    const worktreeId = head.taskId ? taskWorktrees.get(head.taskId) : undefined
    if (head.taskId && worktreeId && !seen.has(worktreeId)) {
      seen.add(worktreeId)
      out.push({ taskId: head.taskId, worktreeId, branch: head.branch })
    }
  }
  return out
}

export type CodeReviewAvailability =
  | { state: 'ready'; worktrees: Worktree[] }
  | { state: 'unsupported' }
  | { state: 'no_worktree' }
  | { state: 'no_lens' }

/**
 * Why a button is dimmed is always one of these, and the screen says which: the code review is off, the review has no worktree
 * to open, or this part of the review has no lens. Never a button that silently does nothing.
 */
export function codeReviewAvailability(input: {
  codeReviewEnabled: boolean
  lens: CodeReviewLens | null
  worktrees: Worktree[]
}): CodeReviewAvailability {
  if (!input.codeReviewEnabled) {
    return { state: 'unsupported' }
  }
  if (input.lens === null) {
    return { state: 'no_lens' }
  }
  return input.worktrees.length === 0
    ? { state: 'no_worktree' }
    : { state: 'ready', worktrees: input.worktrees }
}
