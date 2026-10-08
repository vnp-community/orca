/**
 * use-review-node-note-counts.ts — FE-CV-TASK-060-03
 *
 * Unsent review-note counts per graph node key, for the badges on xyflow nodes. Read-only over
 * the store (comments + the anchors on the ReviewState row): the canvas must not mount the
 * notes persistence hook, which also owns saving.
 *
 * @module components/review-map/notes/use-review-node-note-counts
 */

import { useMemo } from 'react'
import { useAppStore } from '@/store'
import { readNotes } from './review-state-merge'
import { anchoredNotes, noteCountByNodeKey } from './review-note-selectors'

export function useReviewNodeNoteCounts(
  worktreeId: string | null
): Readonly<Record<string, number>> {
  const comments = useAppStore((s) => s.getDiffComments(worktreeId))
  const rawNotes = useAppStore((s) =>
    worktreeId ? s.reviewProgressByWorktree[worktreeId]?.serverState?.notes : undefined
  )
  return useMemo(
    () => noteCountByNodeKey(anchoredNotes(comments, readNotes(rawNotes).anchors)),
    [comments, rawNotes]
  )
}
