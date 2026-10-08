/**
 * turn-file-identity.ts — FE-CV-TASK-060-06
 *
 * Coarse per-file fingerprint stored in a turn marker. It hashes status, paths, line counts and
 * the head/merge-base, so an edit that keeps the same counts looks unchanged: every consumer must
 * present the result as an estimate. Same idea as the mobile `statusEntryIdentity`.
 *
 * @module components/review-map/turns/turn-file-identity
 */

import type { ReviewTurnMarker } from '../../../../../shared/code-intel-types'

export const TURN_MARKER_MAX_FILES = 500

export type TurnFileEntry = {
  path: string
  oldPath?: string
  status: string
  area?: string
  added?: number
  removed?: number
}

export type TurnFileContext = { headOid?: string | null; mergeBase?: string | null }

export type TurnFileIdentity = ReviewTurnMarker['files'][number]

/** FNV-1a over length-prefixed parts so ["ab","c"] and ["a","bc"] differ. */
function hashParts(parts: readonly string[]): string {
  let hash = 2166136261
  for (const part of parts) {
    hash = Math.imul(hash ^ part.length, 16777619)
    for (let i = 0; i < part.length; i += 1) {
      hash = Math.imul(hash ^ part.charCodeAt(i), 16777619)
    }
  }
  return `h${(hash >>> 0).toString(36)}`
}

export function fileIdentity(entry: TurnFileEntry, ctx: TurnFileContext = {}): TurnFileIdentity {
  return {
    p: entry.path,
    ...(entry.oldPath ? { o: entry.oldPath } : {}),
    h: hashParts([
      entry.area ?? '',
      entry.status,
      entry.oldPath ?? '',
      entry.path,
      String(entry.added ?? ''),
      String(entry.removed ?? ''),
      ctx.headOid ?? '',
      ctx.mergeBase ?? ''
    ])
  }
}

export function fileIdentities(
  entries: readonly TurnFileEntry[],
  ctx: TurnFileContext = {},
  cap: number = TURN_MARKER_MAX_FILES
): { files: TurnFileIdentity[]; truncated: boolean } {
  const files = entries.slice(0, cap).map((e) => fileIdentity(e, ctx))
  return { files, truncated: entries.length > cap }
}
