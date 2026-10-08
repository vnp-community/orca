/**
 * quality-finding-open.ts — FE-CV-TASK-087-13
 *
 * Opens a check finding in the editor. The path is validated against the worktree root first
 * (a finding path such as "../x" must never leave it). In-scope findings go to the diff through
 * `onOpenDiff` when the host provides it; otherwise, and for findings outside the change set,
 * the file is opened at the line.
 *
 * @module components/review-map/quality/findings/quality-finding-open
 */

import type { RefObject } from 'react'
import { useAppStore } from '@/store'
import { findWorktreeById } from '@/store/slices/worktree-helpers'
import { resolveAnnotationPathInsideWorktree } from '../../../editor/check-annotation-path'
import { openAnnotationLocation } from '../../../editor/check-annotation-open'
import type { QualityFinding } from '../../../../../../shared/code-intel-quality-types'

export type QualityFindingOpenDiff = (path: string, line: number) => void

export type QualityFindingOpenArgs = {
  worktreeId: string
  finding: Pick<QualityFinding, 'file' | 'line' | 'inScope'>
  onOpenDiff?: QualityFindingOpenDiff
  revealRafRef: RefObject<number | null>
  revealInnerRafRef: RefObject<number | null>
}

/** Returns false when the path was rejected or no worktree is known. */
export function openQualityFinding(args: QualityFindingOpenArgs): boolean {
  const { worktreeId, finding } = args
  const worktree = findWorktreeById(useAppStore.getState().worktreesByRepo, worktreeId)
  if (!worktree) {
    return false
  }
  const resolved = resolveAnnotationPathInsideWorktree(worktree.path, finding.file)
  if (!resolved) {
    return false
  }
  const line = finding.line > 0 ? finding.line : 1
  if (finding.inScope && args.onOpenDiff) {
    args.onOpenDiff(resolved.relativePath, line)
    return true
  }
  openAnnotationLocation({
    worktreeId,
    path: resolved.relativePath,
    line,
    revealRafRef: args.revealRafRef,
    revealInnerRafRef: args.revealInnerRafRef
  })
  return true
}
