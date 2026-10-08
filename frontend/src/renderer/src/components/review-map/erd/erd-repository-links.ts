/**
 * erd-repository-links.ts — FE-CV-TASK-057-03
 *
 * Which code reads/writes a table (a name-scan hint, never proof) and whether a removed
 * or modified column still has accessors the change did not touch.
 */

import type { ErdTableView } from './erd-view-model'

export type ErdAccessor = {
  symbol: ErdTableView['accessedBy'][number]['symbol']
  op: ErdTableView['accessedBy'][number]['op']
  confidence: number
  /** Symbol is part of the change set. */
  changed: boolean
}

export type ErdAccessorGroups = {
  read: ErdAccessor[]
  write: ErdAccessor[]
  readwrite: ErdAccessor[]
  other: ErdAccessor[]
  /** "Hint, may be wrong": only set when overlay data exists. */
  staleAccessWarning: boolean
}

export type ErdChangedSymbolsInput = {
  changedSymbols?: readonly { symbol: { key: string } }[]
} | null

export function groupAccessors(
  table: ErdTableView,
  overlay: ErdChangedSymbolsInput
): ErdAccessorGroups {
  const known = overlay?.changedSymbols !== undefined
  const changedKeys = new Set((overlay?.changedSymbols ?? []).map((c) => c.symbol.key))
  const groups: ErdAccessorGroups = {
    read: [],
    write: [],
    readwrite: [],
    other: [],
    staleAccessWarning: false
  }
  for (const a of table.accessedBy) {
    const item: ErdAccessor = { ...a, changed: changedKeys.has(a.symbol.key) }
    if (a.op === 'read' || a.op === 'write' || a.op === 'readwrite') {
      groups[a.op].push(item)
    } else {
      groups.other.push(item)
    }
  }
  const risky = table.columns.some((c) => c.change === 'removed' || c.change === 'modified')
  groups.staleAccessWarning =
    known && risky && table.accessedBy.some((a) => !changedKeys.has(a.symbol.key))
  return groups
}
