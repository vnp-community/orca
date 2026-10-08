/**
 * review-sent-batch.ts — FE-CV-TASK-060-04
 *
 * Sent-batch bookkeeping. `clearDeliveredDiffComments` deletes delivered notes, so a batch is
 * the only record that they were sent. Bodies are masked and cut: they are history for display,
 * the prompt that reached the agent was built from the originals.
 *
 * @module components/review-map/notes/review-sent-batch
 */

import type { ReviewNoteAnchor, ReviewSentBatch } from '../../../../../shared/code-intel-types'
import type { DiffComment } from '../../../../../shared/types'
import { maskSensitiveText } from '../sensitive-text-masking'

/** Contract §4.6: notes <= 500 entries and 256 KiB; stay under with headroom for the envelope. */
export const REVIEW_NOTES_MAX_ENTRIES = 500
export const REVIEW_NOTES_MAX_BYTES = 200 * 1024
export const SENT_NOTE_BODY_MAX = 2000
export const SEND_PREVIEW_WARN_COUNT = 50

export type NotesPayload = { anchors: Record<string, ReviewNoteAnchor>; sentBatches: ReviewSentBatch[] }

export function capBody(body: string): string {
  const masked = maskSensitiveText(body).text
  return masked.length > SENT_NOTE_BODY_MAX ? `${masked.slice(0, SENT_NOTE_BODY_MAX - 1)}…` : masked
}

export function buildSentBatch(input: {
  batchId: string
  sentAt: number
  turnId: string | null
  targetPaneKey: string | null
  agentType: string | null
  comments: readonly Pick<DiffComment, 'id' | 'filePath' | 'startLine' | 'lineNumber' | 'body'>[]
  anchors: Readonly<Record<string, ReviewNoteAnchor>>
  /** filePath -> identity string computed when sending (estimate, see turn-file-identity). */
  fileIdentityByPath?: Readonly<Record<string, string>>
}): ReviewSentBatch {
  return {
    batchId: input.batchId,
    sentAt: input.sentAt,
    turnId: input.turnId,
    targetPaneKey: input.targetPaneKey,
    agentType: input.agentType,
    notes: input.comments.map((c) => ({
      commentId: c.id,
      anchor:
        input.anchors[c.id] ??
        ({
          kind: 'diff-line',
          filePath: c.filePath,
          ...(c.startLine !== undefined ? { startLine: c.startLine } : {}),
          lineNumber: c.lineNumber
        } satisfies ReviewNoteAnchor),
      filePath: c.filePath,
      ...(c.startLine !== undefined ? { startLine: c.startLine } : {}),
      lineNumber: c.lineNumber,
      body: capBody(c.body),
      ...(input.fileIdentityByPath?.[c.filePath]
        ? { fileIdentityAtSend: input.fileIdentityByPath[c.filePath] }
        : {})
    }))
  }
}

function measure(payload: NotesPayload): number {
  return new TextEncoder().encode(JSON.stringify(payload)).length
}

function entryCount(payload: NotesPayload): number {
  return payload.sentBatches.reduce((n, b) => n + b.notes.length, 0) + Object.keys(payload.anchors).length
}

export type TrimmedNotes = { payload: NotesPayload; droppedBatches: number }

/**
 * Drops the oldest batches until entries and bytes fit. The newest batch is always kept; if it
 * alone is too large its notes are cut from the end rather than losing the send record entirely.
 */
export function trimSentBatches(
  payload: NotesPayload,
  limits: { maxEntries?: number; maxBytes?: number } = {}
): TrimmedNotes {
  const maxEntries = limits.maxEntries ?? REVIEW_NOTES_MAX_ENTRIES
  const maxBytes = limits.maxBytes ?? REVIEW_NOTES_MAX_BYTES
  let batches = [...payload.sentBatches].sort((a, b) => a.sentAt - b.sentAt)
  let dropped = 0
  const fits = (b: ReviewSentBatch[]): boolean => {
    const candidate = { anchors: payload.anchors, sentBatches: b }
    return entryCount(candidate) <= maxEntries && measure(candidate) <= maxBytes
  }
  while (batches.length > 1 && !fits(batches)) {
    batches = batches.slice(1)
    dropped += 1
  }
  if (batches.length === 1 && !fits(batches)) {
    let notes = batches[0].notes
    while (notes.length > 1 && !fits([{ ...batches[0], notes }])) {
      notes = notes.slice(0, Math.max(1, Math.floor(notes.length * 0.8)))
    }
    batches = [{ ...batches[0], notes }]
  }
  return { payload: { anchors: payload.anchors, sentBatches: batches }, droppedBatches: dropped }
}
