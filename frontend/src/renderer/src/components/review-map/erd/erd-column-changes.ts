/**
 * erd-column-changes.ts — FE-CV-TASK-057-02
 *
 * Folds backend `ErdChange` rows into columns. Why only from `changes`: the lens never
 * invents a column the backend did not report; `removed` is the one case that adds a
 * "ghost" column (it no longer exists in the final `columns[]`).
 */

import type {
  ErdChange,
  ErdColumn,
  ErdTable
} from '../../../../../shared/code-intel-architecture-types'
import { maskSensitiveRecord } from '../sensitive-text-masking'

export type ErdChangeKind = 'added' | 'removed' | 'modified'

/** Symbols travel with the data so components never rely on colour alone. */
export const ERD_CHANGE_SYMBOL: Record<ErdChangeKind, string> = {
  added: '+',
  modified: '~',
  removed: '−'
}

export type ErdColumnView = ErdColumn & {
  change?: ErdChangeKind
  changeSymbol?: string
  before?: ErdChange['before']
  after?: ErdChange['after']
  ghost?: boolean
  masked: boolean
}

export function erdTableKey(table: { schema: string; name: string }): string {
  return `${table.schema}.${table.name}`
}

export function asChangeKind(kind: string): ErdChangeKind | null {
  return kind === 'added' || kind === 'removed' || kind === 'modified' ? kind : null
}

export function tableNameMatches(name: string, table: { schema: string; name: string }): boolean {
  return name === table.name || name === erdTableKey(table)
}

export function changeMatchesTable(
  change: ErdChange,
  table: { schema: string; name: string }
): boolean {
  return tableNameMatches(change.table, table)
}

export function maskErdColumn(column: ErdColumn): { column: ErdColumn; masked: boolean } {
  const { value, masked } = maskSensitiveRecord(column, ['defaultExpr', 'comment'])
  return { column: value, masked }
}

function ghostColumn(change: ErdChange): ErdColumnView {
  const before = change.before
  return {
    name: change.column ?? '',
    type: before?.type ?? '',
    canonicalType: before?.type ?? '',
    nullable: before?.nullable ?? true,
    defaultExpr: before?.default,
    isPk: false,
    isFk: false,
    generated: false,
    change: 'removed',
    changeSymbol: ERD_CHANGE_SYMBOL.removed,
    before,
    ghost: true,
    masked: false
  }
}

/** Column-level changes of one table folded into its columns (input is not mutated). */
export function mergeColumnChanges(
  table: ErdTable,
  changes: readonly ErdChange[]
): ErdColumnView[] {
  const columnChanges = changes.filter((c) => c.column && changeMatchesTable(c, table))
  const byColumn = new Map<string, ErdChange>()
  for (const change of columnChanges) {
    byColumn.set(change.column as string, change)
  }
  const views: ErdColumnView[] = table.columns.map((raw) => {
    const { column, masked } = maskErdColumn(raw)
    const change = byColumn.get(column.name)
    const kind = change ? asChangeKind(change.kind) : null
    if (!change || !kind || kind === 'removed') {
      return { ...column, masked }
    }
    return {
      ...column,
      masked,
      change: kind,
      changeSymbol: ERD_CHANGE_SYMBOL[kind],
      before: change.before,
      after: change.after
    }
  })
  const present = new Set(table.columns.map((c) => c.name))
  for (const change of columnChanges) {
    if (asChangeKind(change.kind) === 'removed' && !present.has(change.column as string)) {
      views.push(ghostColumn(change))
    }
  }
  return views
}
