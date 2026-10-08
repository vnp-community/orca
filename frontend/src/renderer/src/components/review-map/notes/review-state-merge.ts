/**
 * review-state-merge.ts — FE-CV-TASK-060-05
 *
 * Merge rules after a CODEINTEL_VERSION_CONFLICT on reviewState.save: nothing either side wrote
 * is lost. Local wins ties because it is the edit the user just made.
 *
 * @module components/review-map/notes/review-state-merge
 */

import type { ReviewNoteAnchor, ReviewSentBatch, ReviewTurnMarker } from '../../../../../shared/code-intel-types'
import type { NotesPayload } from './review-sent-batch'

export const MAX_TURN_MARKERS = 5

export function emptyNotes(): NotesPayload {
  return { anchors: {}, sentBatches: [] }
}

/** Tolerant read of `ReviewState.notes` (typed `unknown` on the wire view). */
export function readNotes(raw: unknown): NotesPayload {
  const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Partial<NotesPayload>
  const anchors =
    typeof r.anchors === 'object' && r.anchors !== null ? (r.anchors as Record<string, ReviewNoteAnchor>) : {}
  return { anchors, sentBatches: Array.isArray(r.sentBatches) ? (r.sentBatches as ReviewSentBatch[]) : [] }
}

export function readTurnMarkers(raw: unknown): ReviewTurnMarker[] {
  return Array.isArray(raw) ? (raw as ReviewTurnMarker[]) : []
}

export function mergeReviewNotes(local: NotesPayload, remote: NotesPayload): NotesPayload {
  const batches = new Map<string, ReviewSentBatch>()
  for (const batch of remote.sentBatches) {
    batches.set(batch.batchId, batch)
  }
  for (const batch of local.sentBatches) {
    batches.set(batch.batchId, batch)
  }
  return {
    anchors: { ...remote.anchors, ...local.anchors },
    sentBatches: [...batches.values()].sort((a, b) => a.sentAt - b.sentAt)
  }
}

/** Union by turnId (local wins), newest MAX_TURN_MARKERS by endedAt, oldest first. */
export function mergeTurnMarkers(
  local: readonly ReviewTurnMarker[],
  remote: readonly ReviewTurnMarker[]
): ReviewTurnMarker[] {
  const byId = new Map<string, ReviewTurnMarker>()
  for (const m of remote) {
    byId.set(m.turnId, m)
  }
  for (const m of local) {
    byId.set(m.turnId, m)
  }
  return [...byId.values()].sort((a, b) => a.endedAt - b.endedAt).slice(-MAX_TURN_MARKERS)
}

export function notesEqual(a: NotesPayload, b: NotesPayload): boolean {
  return JSON.stringify(a) === JSON.stringify(b)
}
