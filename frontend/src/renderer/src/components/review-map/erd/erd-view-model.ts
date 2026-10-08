/**
 * erd-view-model.ts — FE-CV-TASK-057-02
 *
 * ErdModel + change overlay -> display model for the ERD lens. Pure: no store, no RPC.
 * Free-form strings (defaults, comments, check/RLS expressions) are masked here once.
 */

import type {
  ErdModel,
  ErdRelation,
  ErdTable
} from '../../../../../shared/code-intel-architecture-types'
import { maskSensitiveRecord } from '../sensitive-text-masking'
import {
  ERD_CHANGE_SYMBOL,
  asChangeKind,
  tableNameMatches,
  erdTableKey,
  mergeColumnChanges
} from './erd-column-changes'
import type { ErdChangeKind, ErdColumnView } from './erd-column-changes'

export type ErdTableChange = ErdChangeKind | 'untouched' | 'adjacent'

export type ErdTableView = Omit<ErdTable, 'columns'> & {
  key: string
  columns: ErdColumnView[]
  tableChange: ErdTableChange
  changeSymbol?: string
  accessCount: { read: number; write: number }
  /** Highlighted only because `touchedTables` named it; no column detail exists. */
  fromTouchedOnly?: boolean
  /** Table was dropped: it is no longer in `tables[]`, so this is a minimal ghost. */
  dropped?: boolean
  masked: boolean
}

export type ErdRelationView = ErdRelation & {
  id: string
  /** Drawn dashed: logical links and every cross-service link. */
  dashed: boolean
  /** `fk` kind across services is not expected by the contract; flagged, still drawn. */
  anomaly: boolean
}

export type ErdGhostRef = { service: string; table: string }

export type ErdViewModel = {
  tables: ErdTableView[]
  relations: ErdRelationView[]
  ghosts: ErdGhostRef[]
  degradedToTableLevel: boolean
}

export type ErdOverlayInput = { touchedTables?: readonly unknown[] } | null

function touchedTableNames(overlay: ErdOverlayInput, service: string): string[] {
  const names: string[] = []
  for (const raw of overlay?.touchedTables ?? []) {
    const item = raw as { table?: unknown; service?: unknown } | null
    if (item && typeof item.table === 'string' && item.service === service) {
      names.push(item.table)
    }
  }
  return names
}

function endpointKey(e: { service?: string; table: string; columns: string[] }): string {
  return `${e.service ?? ''}.${e.table}.${e.columns.join(',')}`
}

export function relationId(r: ErdRelation): string {
  return `${endpointKey(r.from)}->${endpointKey(r.to)}`
}

function maskTable(table: ErdTable): {
  checks: ErdTable['checks']
  rls: ErdTable['rls']
  comment?: string
  masked: boolean
} {
  let masked = false
  const checks = table.checks.map((c) => {
    const r = maskSensitiveRecord(c, ['expr'])
    masked ||= r.masked
    return r.value
  })
  const rls = table.rls.map((p) => {
    const r = maskSensitiveRecord(p, ['usingExpr', 'withCheckExpr'])
    masked ||= r.masked
    return r.value
  })
  const c = maskSensitiveRecord({ comment: table.comment }, ['comment'])
  masked ||= c.masked
  return { checks, rls, comment: c.value.comment, masked }
}

function emptyDroppedTable(model: ErdModel, name: string): ErdTableView {
  const schema = name.includes('.') ? name.slice(0, name.indexOf('.')) : model.schema
  const bare = name.includes('.') ? name.slice(name.indexOf('.') + 1) : name
  return {
    name: bare,
    schema,
    columns: [],
    pk: [],
    indexes: [],
    checks: [],
    rls: [],
    rlsState: 'none',
    tenantScoped: false,
    degraded: false,
    firstMigration: '',
    lastMigration: '',
    accessedBy: [],
    key: `${schema}.${bare}`,
    tableChange: 'removed',
    changeSymbol: ERD_CHANGE_SYMBOL.removed,
    accessCount: { read: 0, write: 0 },
    dropped: true,
    masked: false
  }
}

export function buildErdViewModel(model: ErdModel, overlay: ErdOverlayInput): ErdViewModel {
  const touched = new Set(touchedTableNames(overlay, model.service))
  const tableLevel = new Map<string, ErdChangeKind>()
  for (const change of model.changes) {
    const kind = asChangeKind(change.kind)
    if (!change.column && kind) {
      tableLevel.set(change.table, kind)
    }
  }
  const hasChanges = model.changes.length > 0

  const tables: ErdTableView[] = model.tables.map((table) => {
    const key = erdTableKey(table)
    const columns = mergeColumnChanges(table, model.changes)
    const directKind = [...tableLevel.entries()].find(([name]) =>
      tableNameMatches(name, table)
    )?.[1]
    const hasColumnChange = columns.some((c) => c.change)
    let tableChange: ErdTableChange = directKind ?? (hasColumnChange ? 'modified' : 'untouched')
    let fromTouchedOnly = false
    if (
      tableChange === 'untouched' &&
      !hasChanges &&
      (touched.has(table.name) || touched.has(key))
    ) {
      tableChange = 'modified'
      fromTouchedOnly = true
    }
    const masked = maskTable(table)
    const accessCount = { read: 0, write: 0 }
    for (const a of table.accessedBy) {
      if (a.op === 'read' || a.op === 'readwrite') {
        accessCount.read += 1
      }
      if (a.op === 'write' || a.op === 'readwrite') {
        accessCount.write += 1
      }
    }
    return {
      ...table,
      checks: masked.checks,
      rls: masked.rls,
      comment: masked.comment,
      key,
      columns,
      tableChange,
      changeSymbol:
        tableChange in ERD_CHANGE_SYMBOL
          ? ERD_CHANGE_SYMBOL[tableChange as ErdChangeKind]
          : undefined,
      accessCount,
      fromTouchedOnly: fromTouchedOnly || undefined,
      masked: masked.masked || columns.some((c) => c.masked)
    }
  })

  for (const [name, kind] of tableLevel) {
    const exists = model.tables.some((t) => tableNameMatches(name, t))
    if (kind === 'removed' && !exists) {
      tables.push(emptyDroppedTable(model, name))
    }
  }

  const relations: ErdRelationView[] = model.relations.map((r) => ({
    ...r,
    id: relationId(r),
    dashed: r.kind !== 'fk' || r.crossService,
    anomaly: r.kind === 'fk' && r.crossService
  }))

  // Neighbours of changed tables are shown as context ("adjacent"), never as changed.
  const changedKeys = new Set(
    tables.filter((t) => t.tableChange !== 'untouched').map((t) => t.name)
  )
  const adjacent = new Set<string>()
  for (const r of model.relations) {
    if (r.from.service && r.from.service !== model.service) {
      continue
    }
    if (r.to.service && r.to.service !== model.service) {
      continue
    }
    if (changedKeys.has(r.from.table)) {
      adjacent.add(r.to.table)
    }
    if (changedKeys.has(r.to.table)) {
      adjacent.add(r.from.table)
    }
  }
  for (const t of tables) {
    if (t.tableChange === 'untouched' && adjacent.has(t.name)) {
      t.tableChange = 'adjacent'
    }
  }

  const ghostMap = new Map<string, ErdGhostRef>()
  const addGhost = (service: string | undefined, table: string): void => {
    if (service && service !== model.service) {
      ghostMap.set(`${service}.${table}`, { service, table })
    }
  }
  for (const ref of model.externalRefs) {
    addGhost(ref.service, ref.table)
  }
  for (const r of model.relations) {
    if (r.crossService) {
      addGhost(r.from.service, r.from.table)
      addGhost(r.to.service, r.to.table)
    }
  }

  return {
    tables,
    relations,
    ghosts: [...ghostMap.values()],
    degradedToTableLevel: !hasChanges && tables.some((t) => t.fromTouchedOnly)
  }
}
