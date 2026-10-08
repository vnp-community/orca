/**
 * quality-annotation-eligibility.ts — FE-CV-TASK-087-09
 *
 * Decides whether finding line numbers (computed on the worktree during a run) may be drawn on
 * the modified side of a diff. Conclusions from reading the code that builds `modifiedContent`:
 *   unstaged / worktree (Changes mode): modified = working-tree blob            -> eligible
 *   staged:                             modified = index blob                    -> not eligible
 *   commit / combined-commit:           modified = a historical commit blob      -> not eligible
 *   branch / combined-branch:           modified = blob at the compare head      -> eligible only when
 *                                       compareHeadOid equals the run's headCommit
 *   combined-all / combined-uncommitted: decided per section by `sectionArea`
 *     (unstaged|untracked = worktree, staged = index, no area = branch entry of combined-all).
 *
 * @module components/editor/quality-annotations/quality-annotation-eligibility
 */

import type { DiffSource } from '@/store/slices/editor'

/** 'worktree' is the editable Changes mode, which has no DiffSource of its own. */
export type QualityAnnotationSource = DiffSource | 'worktree'

export type QualityAnnotationSectionArea = 'staged' | 'unstaged' | 'untracked' | undefined

export type QualityAnnotationSoftWarning = 'dirty' | 'changed-during-run'

export type QualityAnnotationReason =
  | 'ok'
  | 'no-worktree'
  | 'no-source'
  | 'no-run'
  | 'index-side'
  | 'commit-side'
  | 'unknown-area'
  | 'head-mismatch'
  | 'compare-head-mismatch'

export type QualityAnnotationEligibility = {
  eligible: boolean
  reason: QualityAnnotationReason
  softWarning?: QualityAnnotationSoftWarning
}

export type QualityAnnotationEligibilityInput = {
  diffSource: QualityAnnotationSource | undefined
  sectionArea?: QualityAnnotationSectionArea
  runHeadCommit: string | null | undefined
  currentHead: string | null | undefined
  compareHeadOid?: string | null
  hasWorktreeId: boolean
  runDirty?: boolean
  workTreeChangedDuringRun?: boolean
}

type Side = 'worktree' | 'branch' | QualityAnnotationReason

function resolveSide(source: QualityAnnotationSource, area: QualityAnnotationSectionArea): Side {
  switch (source) {
    case 'worktree':
    case 'unstaged':
      return 'worktree'
    case 'staged':
      return 'index-side'
    case 'commit':
    case 'combined-commit':
      return 'commit-side'
    case 'branch':
    case 'combined-branch':
      return 'branch'
    case 'combined-all':
    case 'combined-uncommitted':
      if (area === 'staged') {
        return 'index-side'
      }
      if (area === 'unstaged' || area === 'untracked') {
        return 'worktree'
      }
      return source === 'combined-all' ? 'branch' : 'unknown-area'
    default:
      return 'no-source'
  }
}

function softWarningOf(
  input: QualityAnnotationEligibilityInput
): QualityAnnotationSoftWarning | undefined {
  if (input.workTreeChangedDuringRun) {
    return 'changed-during-run'
  }
  return input.runDirty ? 'dirty' : undefined
}

const no = (reason: QualityAnnotationReason): QualityAnnotationEligibility => ({
  eligible: false,
  reason
})

export function isAnnotationEligible(
  input: QualityAnnotationEligibilityInput
): QualityAnnotationEligibility {
  if (!input.hasWorktreeId) {
    return no('no-worktree')
  }
  if (!input.diffSource) {
    return no('no-source')
  }
  if (!input.runHeadCommit) {
    return no('no-run')
  }
  const side = resolveSide(input.diffSource, input.sectionArea)
  if (side === 'worktree') {
    return input.currentHead && input.currentHead === input.runHeadCommit
      ? { eligible: true, reason: 'ok', softWarning: softWarningOf(input) }
      : no('head-mismatch')
  }
  if (side === 'branch') {
    return input.compareHeadOid && input.compareHeadOid === input.runHeadCommit
      ? { eligible: true, reason: 'ok', softWarning: softWarningOf(input) }
      : no('compare-head-mismatch')
  }
  return no(side)
}
