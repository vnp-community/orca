/**
 * reading-progress-merge.ts — FE-CV-TASK-052-02
 *
 * Last-writer-wins merge of ReadingProgress (per stepKey, by `at`) and the 64 KiB guard.
 * Pure; never mutates inputs.
 */

import type { ReadingProgress, ReadingProgressEntry } from './review-wire-types'

/** Backend cap is 64 KiB; prune at 56 KiB to leave room for the envelope. */
export const READING_PROGRESS_PRUNE_BYTES = 57344

export function mergeReadingProgress(
  local: ReadingProgress,
  remote: ReadingProgress
): ReadingProgress {
  const entries: Record<string, ReadingProgressEntry> = { ...remote.entries }
  for (const [key, entry] of Object.entries(local.entries)) {
    const other = entries[key]
    // Tie goes to local so a just-made toggle is not lost on equal clocks.
    if (!other || entry.at >= other.at) {
      entries[key] = entry
    }
  }
  const newest = (p: ReadingProgress): number =>
    p.lastFocusedKey ? (p.entries[p.lastFocusedKey]?.at ?? 0) : -1
  const lastFocusedKey =
    newest(local) >= newest(remote) ? local.lastFocusedKey : remote.lastFocusedKey
  return {
    version: 1,
    entries,
    lastFocusedKey: lastFocusedKey ?? local.lastFocusedKey ?? remote.lastFocusedKey
  }
}

export function measureReadingProgressBytes(progress: ReadingProgress): number {
  return new TextEncoder().encode(JSON.stringify(progress)).length
}

export type PrunedReadingProgress = {
  progress: ReadingProgress
  droppedUnseen: number
  stillTooLarge: boolean
}

/** Keeps every `seen`; drops oldest `unseen` tombstones until the payload fits. */
export function pruneReadingProgress(
  progress: ReadingProgress,
  maxBytes: number = READING_PROGRESS_PRUNE_BYTES
): PrunedReadingProgress {
  if (measureReadingProgressBytes(progress) <= maxBytes) {
    return { progress, droppedUnseen: 0, stillTooLarge: false }
  }
  const tombstones = Object.entries(progress.entries)
    .filter(([, e]) => e.state === 'unseen')
    .sort((a, b) => a[1].at - b[1].at)
  const entries = { ...progress.entries }
  let dropped = 0
  let next: ReadingProgress = { ...progress, entries }
  for (const [key] of tombstones) {
    delete entries[key]
    dropped += 1
    next = { ...progress, entries }
    if (measureReadingProgressBytes(next) <= maxBytes) {
      return { progress: next, droppedUnseen: dropped, stillTooLarge: false }
    }
  }
  return { progress: next, droppedUnseen: dropped, stillTooLarge: true }
}

export function countSeen(progress: ReadingProgress, stepKeys: readonly string[]): number {
  let n = 0
  for (const k of stepKeys) {
    if (progress.entries[k]?.state === 'seen') {
      n += 1
    }
  }
  return n
}
