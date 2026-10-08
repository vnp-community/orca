/**
 * review-notes-delivery.ts — FE-CV-TASK-060-04
 *
 * What happens after notes reach an agent, in a fixed order: (1) record the sent batch,
 * (2) clear the delivered DiffComments, (3) tell annotation-service (only for the full
 * "all unsent" scope). Recording comes first and never blocks: clearing deletes the notes, so the
 * batch is the only history. A failed record is reported to the caller, the send is not undone.
 *
 * @module components/review-map/notes/review-notes-delivery
 */

import type { ReviewNoteAnchor, ReviewSentBatch } from '../../../../../shared/code-intel-types'
import type { DiffComment } from '../../../../../shared/types'
import { buildSentBatch } from './review-sent-batch'
import type { NotesPayload } from './review-sent-batch'

export type DeliveryDeps = {
  /** Applies an update to the persisted notes; false when the row is not loaded. */
  updateNotes: (update: (prev: NotesPayload) => NotesPayload) => boolean
  clearDelivered: (worktreeId: string, notes: readonly DiffComment[]) => Promise<unknown>
  markAnnotationsSent: (ids: readonly string[]) => void
  now: () => number
  newBatchId: () => string
}

export type DeliveryInput = {
  worktreeId: string
  delivered: readonly DiffComment[]
  anchors: Readonly<Record<string, ReviewNoteAnchor>>
  /** True when `delivered` is exactly the "all unsent notes" set. */
  isAllScope: boolean
  annotationIds: readonly string[]
  fileIdentityByPath?: Readonly<Record<string, string>>
  turn?: { turnId: string | null; targetPaneKey: string | null; agentType: string | null }
}

export type DeliveryOutcome = { batch: ReviewSentBatch; recorded: boolean }

export async function handleNotesDelivered(
  deps: DeliveryDeps,
  input: DeliveryInput
): Promise<DeliveryOutcome> {
  const batch = buildSentBatch({
    batchId: deps.newBatchId(),
    sentAt: deps.now(),
    turnId: input.turn?.turnId ?? null,
    targetPaneKey: input.turn?.targetPaneKey ?? null,
    agentType: input.turn?.agentType ?? null,
    comments: input.delivered,
    anchors: input.anchors,
    fileIdentityByPath: input.fileIdentityByPath
  })
  let recorded = false
  try {
    recorded = deps.updateNotes((prev) => ({ ...prev, sentBatches: [...prev.sentBatches, batch] }))
  } catch {
    recorded = false
  }
  await deps.clearDelivered(input.worktreeId, input.delivered)
  if (input.isAllScope) {
    deps.markAnnotationsSent(input.annotationIds)
  }
  return { batch, recorded }
}
