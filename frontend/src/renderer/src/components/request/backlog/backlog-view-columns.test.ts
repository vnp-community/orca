import { describe, expect, it } from 'vitest'
import {
  BACKLOG_COLUMNS, EMPTY_BACKLOG_FILTERS, filterGroups, filterRequestRows, hasActiveBacklogFilters
} from './backlog-view-columns'
import type { BacklogGroupData, RequestBacklogRowData } from '../../../../../shared/request-backlog-types'

const row = (over: Partial<RequestBacklogRowData> & { requestId: string }): RequestBacklogRowData => ({
  number: 1, title: 'Title', type: 'bug', sourceProvider: 'github', returnedFromStage: 'plan',
  returnedCategory: 'infeasible', parentRequestIds: [], ...over
})

describe('backlog-view-columns', () => {
  it('declares 9 / 3 / 5 columns with a sticky first column', () => {
    expect(BACKLOG_COLUMNS.requests).toHaveLength(9)
    expect(BACKLOG_COLUMNS.tasks).toHaveLength(3)
    expect(BACKLOG_COLUMNS.execute).toHaveLength(5)
    for (const cols of Object.values(BACKLOG_COLUMNS)) {expect(cols[0].sticky).toBe(true)}
  })

  it('filters request rows by title, #number, type and category', () => {
    const rows = [
      row({ requestId: 'a', title: 'Login broken', number: 42 }),
      row({ requestId: 'b', title: 'Docs', number: 7, type: 'docs', returnedCategory: 'missing_info' })
    ]
    const f = EMPTY_BACKLOG_FILTERS
    expect(filterRequestRows(rows, { ...f, q: 'login' }).map((r) => r.requestId)).toEqual(['a'])
    expect(filterRequestRows(rows, { ...f, q: '#7' }).map((r) => r.requestId)).toEqual(['b'])
    expect(filterRequestRows(rows, { ...f, type: 'docs' }).map((r) => r.requestId)).toEqual(['b'])
    expect(filterRequestRows(rows, { ...f, category: 'infeasible' }).map((r) => r.requestId)).toEqual(['a'])
    expect(filterRequestRows(rows, f)).toHaveLength(2)
  })

  it('filters groups by plan/phase title or task title', () => {
    const task = (taskId: string, title: string) => ({
      taskId, title, status: 'open', estimatedHours: null, blockedByTaskIds: [], failedAttempts: 0
    })
    const groups: BacklogGroupData[] = [
      { requestId: 'r1', planTitle: 'Auth plan', gateStatus: 'approved', tasks: [task('t1', 'Write tests'), task('t2', 'Deploy')] },
      { requestId: 'r2', planTitle: 'Other', gateStatus: 'none', tasks: [task('t3', 'Deploy docs')] }
    ]
    expect(filterGroups(groups, { q: 'auth' })[0].tasks).toHaveLength(2)
    const hit = filterGroups(groups, { q: 'deploy' })
    expect(hit.map((g) => g.tasks.length)).toEqual([1, 1])
    expect(filterGroups(groups, { q: 'zzz' })).toEqual([])
    expect(filterGroups(groups, { q: '' })).toBe(groups)
  })

  it('detects active filters', () => {
    expect(hasActiveBacklogFilters(EMPTY_BACKLOG_FILTERS)).toBe(false)
    expect(hasActiveBacklogFilters({ ...EMPTY_BACKLOG_FILTERS, q: ' x' })).toBe(true)
  })
})
