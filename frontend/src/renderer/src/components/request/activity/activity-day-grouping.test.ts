import { describe, expect, it } from 'vitest'
import type { RequestActivityEntry } from '../../../../../shared/request-execution-types'
import { dayKeyOf, groupActivityByDay, UNDATED_DAY_KEY } from './activity-day-grouping'

const at = (id: string, occurredAt: string): RequestActivityEntry => ({
  id,
  kind: 'task.started',
  occurredAt,
  actorKind: 'system',
  summary: id,
  severity: 'info',
  details: {}
})

describe('dayKeyOf', () => {
  it('uses the viewer time zone for the day boundary', () => {
    // 23:30 UTC is already the next day in Tokyo and still the same day in New York.
    expect(dayKeyOf('2026-10-15T23:30:00Z', 'UTC')).toBe('2026-10-15')
    expect(dayKeyOf('2026-10-15T23:30:00Z', 'Asia/Tokyo')).toBe('2026-10-16')
    expect(dayKeyOf('2026-10-15T23:30:00Z', 'America/New_York')).toBe('2026-10-15')
  })

  it('returns a marker for an unparsable timestamp', () => {
    expect(dayKeyOf('nope')).toBe(UNDATED_DAY_KEY)
  })
})

describe('groupActivityByDay', () => {
  const now = Date.parse('2026-10-16T10:00:00Z')

  it('labels today and yesterday, orders newest first, and splits on the day boundary', () => {
    const groups = groupActivityByDay(
      [
        at('old', '2026-10-10T08:00:00Z'),
        at('t1', '2026-10-16T09:00:00Z'),
        at('y1', '2026-10-15T23:59:00Z'),
        at('t2', '2026-10-16T00:00:00Z'),
        at('y2', '2026-10-15T00:00:00Z')
      ],
      now,
      'UTC'
    )
    expect(groups.map((g) => [g.dayKey, g.relative, g.entries.map((e) => e.id)])).toEqual([
      ['2026-10-16', 'today', ['t1', 't2']],
      ['2026-10-15', 'yesterday', ['y1', 'y2']],
      ['2026-10-10', null, ['old']]
    ])
  })

  it('applies the time zone to which group a line falls in', () => {
    const groups = groupActivityByDay([at('a', '2026-10-15T23:30:00Z')], now, 'Asia/Tokyo')
    expect(groups[0]).toMatchObject({ dayKey: '2026-10-16', relative: 'today' })
  })

  it('steps over a month boundary for yesterday', () => {
    const groups = groupActivityByDay(
      [at('a', '2026-09-30T12:00:00Z')],
      Date.parse('2026-10-01T08:00:00Z'),
      'UTC'
    )
    expect(groups[0]?.relative).toBe('yesterday')
  })

  it('keeps server order for equal timestamps and puts undated lines last', () => {
    const groups = groupActivityByDay(
      [at('bad', 'nope'), at('a', '2026-10-16T09:00:00Z'), at('b', '2026-10-16T09:00:00Z')],
      now,
      'UTC'
    )
    expect(groups.map((g) => g.entries.map((e) => e.id))).toEqual([['a', 'b'], ['bad']])
    expect(groups[1]?.dayKey).toBe(UNDATED_DAY_KEY)
  })

  it('returns no groups for no entries', () => {
    expect(groupActivityByDay([], now, 'UTC')).toEqual([])
  })
})
