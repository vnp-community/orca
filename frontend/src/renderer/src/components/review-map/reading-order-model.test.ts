import { describe, expect, it } from 'vitest'
import {
  buildReadingOrderItems,
  buildReadingOrderRows,
  countReadingOrderTotals,
  OTHER_GROUP_ID
} from './reading-order-model'
import { EMPTY_PROGRESS, makeStep } from './review-test-data'

const group = (id: string, label: string, stepKeys: string[]) => ({
  componentId: id,
  label,
  files: stepKeys.length,
  symbols: 0,
  added: 0,
  removed: 0,
  riskPoints: 0,
  stepKeys
})
const none = new Set<string>()

describe('buildReadingOrderItems', () => {
  it('sorts by n, keeps first duplicate stepKey, maps file status, unknown enum -> unknown', () => {
    const items = buildReadingOrderItems(
      [makeStep(3), makeStep(1), makeStep(1, { file: 'dup.ts' })],
      [group('c1', 'API', ['s1'])],
      [
        { path: 'src/f1.ts', status: 'added' },
        { path: 'src/f3.ts', status: 'weird' as never }
      ]
    )
    expect(items.map((i) => i.stepKey)).toEqual(['s1', 's3'])
    expect(items[0].file).toBe('src/f1.ts')
    expect(items[0].fileStatus).toBe('added')
    expect(items[0].component).toEqual({ id: 'c1', label: 'API' })
    expect(items[1].fileStatus).toBe('unknown')
  })
  it('a step listed in two groups goes to the first', () => {
    const items = buildReadingOrderItems(
      [makeStep(1)],
      [group('a', 'A', ['s1']), group('b', 'B', ['s1'])],
      []
    )
    expect(items[0].component?.id).toBe('a')
  })
})

describe('buildReadingOrderRows', () => {
  const items = buildReadingOrderItems(
    [makeStep(1), makeStep(2), makeStep(3), makeStep(4)],
    [group('late', 'Late', ['s3', 's4']), group('early', 'Early', ['s1'])],
    []
  )
  it('orders groups by smallest n, ungrouped step goes to the trailing Other group', () => {
    const rows = buildReadingOrderRows(items, { collapsedGroupIds: none, filter: null })
    const groups = rows.filter((r) => r.type === 'group').map((r) => r.id)
    expect(groups).toEqual(['early', 'late', OTHER_GROUP_ID])
    expect(rows.filter((r) => r.type === 'step')).toHaveLength(4)
  })
  it('collapsed group hides steps but keeps totals', () => {
    const rows = buildReadingOrderRows(items, {
      collapsedGroupIds: new Set(['late']),
      filter: null
    })
    const late = rows.find((r) => r.type === 'group' && r.id === 'late')
    expect(late).toMatchObject({ collapsed: true, total: 2 })
    expect(rows.some((r) => r.type === 'step' && r.groupId === 'late')).toBe(false)
  })
  it('filter hides rows and empty groups, totals unchanged', () => {
    const rows = buildReadingOrderRows(items, {
      collapsedGroupIds: none,
      filter: new Set(['src/f3.ts'])
    })
    expect(rows.map((r) => r.id)).toEqual(['late', 's3'])
    expect(rows[0]).toMatchObject({ total: 2 })
    expect(countReadingOrderTotals(items, EMPTY_PROGRESS).total).toBe(4)
  })
  it('group seen count comes from progress', () => {
    const rows = buildReadingOrderRows(items, {
      collapsedGroupIds: none,
      filter: null,
      progress: { version: 1, lastFocusedKey: null, entries: { s3: { state: 'seen', at: 1 } } }
    })
    expect(rows.find((r) => r.id === 'late')).toMatchObject({ seen: 1, total: 2 })
  })
  it('overflow marker is not counted as a reviewable file', () => {
    const withOverflow = buildReadingOrderItems(
      [makeStep(1), makeStep(2, { reason: 'overflow' })],
      [],
      []
    )
    expect(countReadingOrderTotals(withOverflow, EMPTY_PROGRESS).total).toBe(1)
  })
})
