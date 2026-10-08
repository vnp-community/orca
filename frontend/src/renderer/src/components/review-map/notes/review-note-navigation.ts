/**
 * review-note-navigation.ts — FE-CV-TASK-060-03
 *
 * "Jump to anchor" from the notes panel: switches lens, selects the node when the lens has a
 * selection notion, and opens the diff at the note's line. Pure over injected store actions.
 *
 * @module components/review-map/notes/review-note-navigation
 */

import type { ReviewNoteAnchor } from '../../../../../shared/code-intel-types'
import { resolveGraphNodeCommentTarget } from './review-note-anchor'

export type NoteNavigationActions = {
  setReviewLens: (worktreeId: string, lens: string) => void
  selectReviewSymbol: (worktreeId: string, symbolKey: string | null) => void
  selectErdTable: (worktreeId: string, tableKey: string | null) => void
  selectStorageNode: (worktreeId: string, nodeId: string | null) => void
  setReviewDataFlowId: (worktreeId: string, flowId: string | null) => void
  openDiff: (path: string, line?: number) => void
}

const SYMBOL_LENSES = new Set(['impact', 'architecture', 'structure'])

export function jumpToNoteAnchor(
  worktreeId: string,
  anchor: ReviewNoteAnchor,
  actions: NoteNavigationActions
): void {
  if (anchor.kind === 'graph-node') {
    actions.setReviewLens(worktreeId, anchor.lens)
    if (anchor.lens === 'erd') {
      actions.selectErdTable(worktreeId, anchor.nodeKey)
    } else if (anchor.lens === 'storage') {
      actions.selectStorageNode(worktreeId, anchor.nodeKey)
    } else if (anchor.lens === 'dataflow') {
      actions.setReviewDataFlowId(worktreeId, anchor.nodeKey)
    } else if (SYMBOL_LENSES.has(anchor.lens)) {
      actions.selectReviewSymbol(worktreeId, anchor.nodeKey)
    }
  }
  const target = resolveGraphNodeCommentTarget(anchor)
  if (target) {
    actions.openDiff(target.filePath, target.lineNumber > 0 ? target.lineNumber : undefined)
  }
}
