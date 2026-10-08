/**
 * turn-compare-model.ts — FE-CV-TASK-060-06
 *
 * "Previous turn vs this turn" labels. Always an estimate: markers hold fingerprints, not code.
 * Symbol labels exist only when both markers carry symbol keys; keys cannot tell "changed" from
 * "unchanged", so symbols are never labelled changed_in_turn.
 *
 * @module components/review-map/turns/turn-compare-model
 */

import type { ReviewTurnMarker } from '../../../../../shared/code-intel-types'

export type TurnChangeLabel = 'new_in_turn' | 'changed_in_turn' | 'unchanged_since' | 'reverted_in_turn'

export type TurnCompareResult = {
  /** Always true: fingerprints can miss edits that keep the same line counts. */
  estimated: true
  files: Record<string, TurnChangeLabel>
  /** null when either side lacks symbol keys (overlay unavailable). */
  symbols: Record<string, TurnChangeLabel> | null
  counts: Record<TurnChangeLabel, number>
}

export function compareTurns(
  prev: Pick<ReviewTurnMarker, 'files' | 'symbolKeys' | 'overlayAvailable'>,
  curr: Pick<ReviewTurnMarker, 'files' | 'symbolKeys' | 'overlayAvailable'>
): TurnCompareResult {
  const prevByPath = new Map(prev.files.map((f) => [f.p, f.h]))
  const currByPath = new Map(curr.files.map((f) => [f.p, f.h]))
  const files: Record<string, TurnChangeLabel> = {}
  for (const [path, hash] of currByPath) {
    const before = prevByPath.get(path)
    files[path] = before === undefined ? 'new_in_turn' : before === hash ? 'unchanged_since' : 'changed_in_turn'
  }
  for (const path of prevByPath.keys()) {
    if (!currByPath.has(path)) {
      files[path] = 'reverted_in_turn'
    }
  }

  let symbols: Record<string, TurnChangeLabel> | null = null
  if (prev.overlayAvailable && curr.overlayAvailable && prev.symbolKeys && curr.symbolKeys) {
    const before = new Set(prev.symbolKeys)
    const now = new Set(curr.symbolKeys)
    symbols = {}
    for (const key of now) {
      symbols[key] = before.has(key) ? 'unchanged_since' : 'new_in_turn'
    }
    for (const key of before) {
      if (!now.has(key)) {
        symbols[key] = 'reverted_in_turn'
      }
    }
  }

  const counts: Record<TurnChangeLabel, number> = {
    new_in_turn: 0,
    changed_in_turn: 0,
    unchanged_since: 0,
    reverted_in_turn: 0
  }
  for (const label of Object.values(files)) {
    counts[label] += 1
  }
  return { estimated: true, files, symbols, counts }
}

export type NoteProgressHint = 'file_changed' | 'file_unchanged' | 'unknown'

/**
 * Hint shown on an already-sent note. Never "resolved": a changed file is only a signal that the
 * agent may have acted. `listTruncated` says the current identity list was capped.
 */
export function noteProgressHint(
  sentNote: { filePath: string; fileIdentityAtSend?: string },
  currentIdentityByPath: Readonly<Record<string, string>>,
  listTruncated = false
): NoteProgressHint {
  if (!sentNote.fileIdentityAtSend) {
    return 'unknown'
  }
  const current = currentIdentityByPath[sentNote.filePath]
  if (current === undefined) {
    // No longer in the change list: edited away or committed, unless the list was capped.
    return listTruncated ? 'unknown' : 'file_changed'
  }
  return current === sentNote.fileIdentityAtSend ? 'file_unchanged' : 'file_changed'
}
