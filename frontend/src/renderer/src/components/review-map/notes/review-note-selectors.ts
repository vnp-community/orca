/**
 * review-note-selectors.ts — FE-CV-TASK-060-03
 *
 * Pure views over DiffComments + anchors: per-node unsent counts for graph badges and grouping
 * for the notes panel. Only comments that have a graph/finding anchor count as review-map notes.
 *
 * @module components/review-map/notes/review-note-selectors
 */

import type { ReviewNoteAnchor } from '../../../../../shared/code-intel-types'
import type { DiffComment } from '../../../../../shared/types'
import { anchorLensId, isGraphAnchor } from './review-note-anchor'

export type AnchoredNote = {
  comment: DiffComment
  anchor: Extract<ReviewNoteAnchor, { kind: 'graph-node' | 'finding' }>
}

export function anchoredNotes(
  comments: readonly DiffComment[],
  anchors: Readonly<Record<string, ReviewNoteAnchor>>
): AnchoredNote[] {
  const out: AnchoredNote[] = []
  for (const comment of comments) {
    const anchor = anchors[comment.id]
    if (isGraphAnchor(anchor)) {
      out.push({ comment, anchor })
    }
  }
  return out
}

/** Node key the badge sits on: graph nodes use nodeKey, findings use findingKey. */
export function anchorNodeKey(anchor: AnchoredNote['anchor']): string {
  return anchor.kind === 'graph-node' ? anchor.nodeKey : anchor.findingKey
}

/** Unsent notes per node key (sent ones were already handed to the agent). */
export function noteCountByNodeKey(notes: readonly AnchoredNote[]): Record<string, number> {
  const counts: Record<string, number> = {}
  for (const { comment, anchor } of notes) {
    if (comment.sentAt) {
      continue
    }
    const key = anchorNodeKey(anchor)
    counts[key] = (counts[key] ?? 0) + 1
  }
  return counts
}

export type NoteLensGroup = { lens: string; notes: AnchoredNote[] }

export function groupNotesByLens(notes: readonly AnchoredNote[]): NoteLensGroup[] {
  const groups = new Map<string, AnchoredNote[]>()
  for (const note of notes) {
    const lens = anchorLensId(note.anchor)
    const list = groups.get(lens)
    if (list) {
      list.push(note)
    } else {
      groups.set(lens, [note])
    }
  }
  return [...groups.entries()]
    .map(([lens, list]) => ({
      lens,
      notes: [...list].sort((a, b) => a.comment.createdAt - b.comment.createdAt)
    }))
    .sort((a, b) => (a.lens < b.lens ? -1 : 1))
}
