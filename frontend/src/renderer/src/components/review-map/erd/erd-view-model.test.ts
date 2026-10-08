import { describe, expect, it } from 'vitest'
import { buildErdViewModel } from './erd-view-model'
import { mergeColumnChanges } from './erd-column-changes'
import { erdColumn, erdTable, sampleErdModel } from './erd-model.fixture'

describe('mergeColumnChanges', () => {
  const model = sampleErdModel()
  it('marks added/modified columns and appends a ghost column for removed ones', () => {
    const cols = mergeColumnChanges(model.tables[0], model.changes)
    expect(cols.find((c) => c.name === 'vault_ssh_role')).toMatchObject({
      change: 'added',
      changeSymbol: '+'
    })
    expect(cols.find((c) => c.name === 'status')).toMatchObject({
      change: 'modified',
      changeSymbol: '~'
    })
    expect(cols.find((c) => c.name === 'status')?.before?.type).toBe('varchar(32)')
    expect(cols.find((c) => c.name === 'old_flag')).toMatchObject({
      change: 'removed',
      ghost: true,
      changeSymbol: '−'
    })
    expect(cols.filter((c) => !c.change && !c.ghost).map((c) => c.name)).toEqual(['id'])
  })

  it('never invents a column and does not mutate its input', () => {
    const table = erdTable('t', [erdColumn('a')])
    const snapshot = JSON.stringify(table)
    const cols = mergeColumnChanges(table, [
      { table: 't', column: 'zzz', kind: 'modified', migrationFile: 'm' }
    ])
    expect(cols.map((c) => c.name)).toEqual(['a'])
    expect(JSON.stringify(table)).toBe(snapshot)
  })

  it('ignores unknown change kinds', () => {
    const table = erdTable('t', [erdColumn('a')])
    const cols = mergeColumnChanges(table, [
      { table: 't', column: 'a', kind: 'unknown', migrationFile: 'm' }
    ])
    expect(cols[0].change).toBeUndefined()
  })
})

describe('buildErdViewModel', () => {
  it('derives table state, adjacency, access counts and ghosts', () => {
    const vm = buildErdViewModel(sampleErdModel(), null)
    const by = Object.fromEntries(vm.tables.map((t) => [t.name, t]))
    expect(by.dev_servers.tableChange).toBe('modified')
    expect(by.agents.tableChange).toBe('adjacent')
    expect(by.audit.tableChange).toBe('untouched')
    expect(by.agents.accessCount).toEqual({ read: 2, write: 1 })
    expect(vm.ghosts).toEqual([{ service: 'auth', table: 'tenants' }])
    expect(vm.degradedToTableLevel).toBe(false)
  })

  it('masks secrets in default expressions', () => {
    const vm = buildErdViewModel(sampleErdModel(), null)
    const status = vm.tables[0].columns.find((c) => c.name === 'status')
    expect(status?.defaultExpr).not.toContain('hunter2')
    expect(status?.masked).toBe(true)
    expect(vm.tables[0].masked).toBe(true)
  })

  it('builds stable relation ids and dashes logical/cross-service links', () => {
    const vm = buildErdViewModel(sampleErdModel(), null)
    expect(vm.relations[0].id).toBe('.agents.dev_server_id->.dev_servers.id')
    expect(vm.relations.map((r) => r.dashed)).toEqual([false, true])
    expect(vm.relations.every((r) => !r.anomaly)).toBe(true)
  })

  it('flags an fk across services as an anomaly but still draws it dashed', () => {
    const model = sampleErdModel()
    model.relations[1] = { ...model.relations[1], kind: 'fk' }
    const rel = buildErdViewModel(model, null).relations[1]
    expect(rel).toMatchObject({ dashed: true, anomaly: true })
  })

  it('adds a minimal ghost for a dropped table', () => {
    const model = sampleErdModel({
      changes: [{ table: 'old_t', kind: 'removed', migrationFile: 'm' }]
    })
    const vm = buildErdViewModel(model, null)
    expect(vm.tables.find((t) => t.name === 'old_t')).toMatchObject({
      dropped: true,
      tableChange: 'removed',
      changeSymbol: '−'
    })
  })

  it('marks a created table as added', () => {
    const model = sampleErdModel({
      changes: [{ table: 'audit', kind: 'added', migrationFile: 'm' }]
    })
    expect(buildErdViewModel(model, null).tables.find((t) => t.name === 'audit')?.tableChange).toBe(
      'added'
    )
  })

  it('degrades to table level from touchedTables when changes is empty', () => {
    const model = sampleErdModel({ changes: [] })
    const vm = buildErdViewModel(model, {
      touchedTables: [
        { table: 'audit', service: 'infra-fleet' },
        { table: 'x', service: 'other' }
      ]
    })
    const audit = vm.tables.find((t) => t.name === 'audit')
    expect(audit).toMatchObject({ tableChange: 'modified', fromTouchedOnly: true })
    expect(audit?.columns.every((c) => !c.change)).toBe(true)
    expect(vm.degradedToTableLevel).toBe(true)
  })

  it('does not mutate the model', () => {
    const model = sampleErdModel()
    const snapshot = JSON.stringify(model)
    buildErdViewModel(model, null)
    expect(JSON.stringify(model)).toBe(snapshot)
  })
})
