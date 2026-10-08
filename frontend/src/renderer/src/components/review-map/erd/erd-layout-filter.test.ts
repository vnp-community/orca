import { describe, expect, it } from 'vitest'
import { buildErdViewModel } from './erd-view-model'
import { ERD_COLLAPSED_COLUMN_LIMIT, erdNodeHeight, layoutErdTables } from './erd-layout'
import { filterErdTables, defaultErdFilterMode, selectCollapsedColumns } from './erd-table-filter'
import { groupAccessors } from './erd-repository-links'
import { erdColumn, erdTable, sampleErdModel } from './erd-model.fixture'
import type { ErdRect } from './erd-layout'

const noExpanded = new Set<string>()
const overlap = (a: ErdRect, b: ErdRect): boolean =>
  a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height

function chain(n: number, cycle = false) {
  const tables = Array.from({ length: n }, (_, i) =>
    erdTable(`t${i}`, [erdColumn('id', { isPk: true })])
  )
  const relations = tables.slice(1).map((t, i) => ({
    kind: 'fk' as const,
    from: { table: t.name, columns: ['x'] },
    to: { table: tables[i].name, columns: ['id'] },
    crossService: false,
    source: 'ddl' as const,
    confidence: 1
  }))
  if (cycle) {
    relations.push({
      kind: 'fk',
      from: { table: 't0', columns: ['x'] },
      to: { table: `t${n - 1}`, columns: ['id'] },
      crossService: false,
      source: 'ddl',
      confidence: 1
    })
  }
  return buildErdViewModel(
    { ...sampleErdModel({ tables, relations, changes: [], externalRefs: [] }) },
    null
  )
}

describe('layoutErdTables', () => {
  it('is deterministic, overlap-free and layers referenced tables first', () => {
    const vm = buildErdViewModel(sampleErdModel(), null)
    const input = {
      tables: vm.tables,
      relations: vm.relations,
      expandedTables: noExpanded,
      groupBySchema: false
    }
    const a = layoutErdTables(input)
    expect([...layoutErdTables(input).positions]).toEqual([...a.positions])
    const rects = [...a.positions.values()]
    for (let i = 0; i < rects.length; i++) {
      for (let j = i + 1; j < rects.length; j++) {
        expect(overlap(rects[i], rects[j])).toBe(false)
      }
    }
    expect(a.positions.get('infra.dev_servers')!.x).toBeLessThan(a.positions.get('infra.agents')!.x)
    // isolated table goes below the connected component
    expect(a.positions.get('infra.audit')!.y).toBeGreaterThan(a.positions.get('infra.agents')!.y)
  })

  it('terminates on FK cycles and handles 40 tables', () => {
    for (const vm of [chain(5, true), chain(40)]) {
      const r = layoutErdTables({
        tables: vm.tables,
        relations: vm.relations,
        expandedTables: noExpanded,
        groupBySchema: false
      })
      expect(r.positions.size).toBe(vm.tables.length)
    }
  })

  it('expanding a table raises its node height', () => {
    const big = erdTable(
      'big',
      Array.from({ length: 30 }, (_, i) => erdColumn(`c${i}`))
    )
    expect(erdNodeHeight(big, true)).toBeGreaterThan(erdNodeHeight(big, false))
    expect(erdNodeHeight(big, false)).toBeLessThan(erdNodeHeight(big, true))
  })

  it('stacks schema groups without overlap', () => {
    const model = sampleErdModel()
    model.tables[2] = { ...model.tables[2], schema: 'ops' }
    const vm = buildErdViewModel(model, null)
    const r = layoutErdTables({
      tables: vm.tables,
      relations: vm.relations,
      expandedTables: noExpanded,
      groupBySchema: true
    })
    expect(r.groups.map((g) => g.schema)).toEqual(['infra', 'ops'])
    expect(overlap(r.groups[0], r.groups[1])).toBe(false)
  })
})

describe('filterErdTables', () => {
  const vm = buildErdViewModel(sampleErdModel(), null)
  const base = {
    tables: vm.tables,
    relations: vm.relations,
    query: '',
    schemas: null,
    mode: 'all' as const
  }

  it('focus mode keeps changed + adjacent and pulls in search matches', () => {
    expect(
      filterErdTables({ ...base, mode: 'focus' })
        .visible.map((t) => t.name)
        .sort()
    ).toEqual(['agents', 'dev_servers'])
    expect(
      filterErdTables({ ...base, mode: 'focus', query: 'AUDIT' })
        .visible.map((t) => t.name)
        .sort()
    ).toEqual(['agents', 'audit', 'dev_servers'])
  })

  it('searches accent-insensitively across columns, types and symbols', () => {
    expect(filterErdTables({ ...base, query: 'vault_ssh' }).matchCount).toBe(1)
    expect(filterErdTables({ ...base, query: 'uuid' }).matchCount).toBe(1)
    expect(filterErdTables({ ...base, query: 'SaveAgent' }).matchCount).toBe(1)
    expect(filterErdTables({ ...base, query: 'Ágents' }).matchCount).toBe(1)
  })

  it('dims non-matches in all mode', () => {
    const r = filterErdTables({ ...base, query: 'audit' })
    expect(r.visible).toHaveLength(3)
    expect([...r.dimmed].sort()).toEqual(['infra.agents', 'infra.dev_servers'])
  })

  it('caps by priority and always reports truncation', () => {
    const r = filterErdTables({ ...base, limit: 2 })
    expect(r.truncated).toEqual({ shown: 2, total: 3, capped: true })
    expect(r.visible.map((t) => t.name).sort()).toEqual(['agents', 'dev_servers'])
    expect(filterErdTables(base).truncated).toEqual({ shown: 3, total: 3, capped: false })
  })

  it('filters by schema and picks the default mode by size', () => {
    expect(filterErdTables({ ...base, schemas: new Set(['ops']) }).visible).toHaveLength(0)
    expect(defaultErdFilterMode(26)).toBe('focus')
    expect(defaultErdFilterMode(25)).toBe('all')
  })
})

describe('selectCollapsedColumns', () => {
  it('orders PK, FK, changed, match, rest and reports hidden', () => {
    const cols = Array.from({ length: 20 }, (_, i) => erdColumn(`c${i}`))
    const views = cols.map((c, i) => ({
      ...c,
      masked: false,
      ...(i === 19 ? { isPk: true } : {}),
      ...(i === 18 ? { isFk: true } : {}),
      ...(i === 17 ? { change: 'added' as const } : {})
    }))
    const r = selectCollapsedColumns(views, 'c16', false)
    expect(r.shown.slice(0, 4).map((c) => c.name)).toEqual(['c19', 'c18', 'c17', 'c16'])
    expect(r.shown).toHaveLength(ERD_COLLAPSED_COLUMN_LIMIT)
    expect(r.hidden).toBe(8)
    expect(selectCollapsedColumns(views, '', true).shown).toHaveLength(20)
  })
})

describe('groupAccessors', () => {
  const vm = buildErdViewModel(sampleErdModel(), null)
  it('groups by op and flags changed symbols', () => {
    const g = groupAccessors(vm.tables[1], { changedSymbols: [{ symbol: { key: 'sym:a' } }] })
    expect(g.read.map((a) => [a.symbol.name, a.changed])).toEqual([['ListAgents', true]])
    expect(g.readwrite).toHaveLength(1)
  })
  it('warns only when a risky column change has untouched accessors and overlay data exists', () => {
    const risky = { ...vm.tables[0], accessedBy: vm.tables[1].accessedBy }
    expect(groupAccessors(risky, { changedSymbols: [] }).staleAccessWarning).toBe(true)
    expect(groupAccessors(risky, null).staleAccessWarning).toBe(false)
    expect(groupAccessors(vm.tables[1], { changedSymbols: [] }).staleAccessWarning).toBe(false)
  })
})
