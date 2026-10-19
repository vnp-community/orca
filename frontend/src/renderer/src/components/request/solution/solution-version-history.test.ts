import { describe, expect, it } from 'vitest'
import type { Solution } from '../../../../../shared/request-types'
import {
  VERSION_NOTE_MAX_LENGTH,
  currentSolutionOf,
  orderSolutionsNewestFirst,
  solutionVersionNumbers,
  versionBadge,
  versionNote,
  versionPageCount,
  versionPageOf
} from './solution-version-history'

const sol = (over: Partial<Solution> & { id: string }): Solution => ({
  requestId: 'r1',
  kind: 'solution',
  status: 'ready',
  ...over
})

describe('orderSolutionsNewestFirst', () => {
  it('orders by seq when both carry one', () => {
    const out = orderSolutionsNewestFirst([
      sol({ id: 'a', seq: 1 }),
      sol({ id: 'c', seq: 3 }),
      sol({ id: 'b', seq: 2 })
    ])
    expect(out.map((s) => s.id)).toEqual(['c', 'b', 'a'])
  })

  it('falls back to version, then generation time, without seq', () => {
    const out = orderSolutionsNewestFirst([
      sol({ id: 'a', generatedAt: '2026-01-01T00:00:00Z' }),
      sol({ id: 'c', generatedAt: '2026-01-03T00:00:00Z' }),
      sol({ id: 'b', generatedAt: '2026-01-02T00:00:00Z' })
    ])
    expect(out.map((s) => s.id)).toEqual(['c', 'b', 'a'])
    const byVersion = orderSolutionsNewestFirst([
      sol({ id: 'a', version: 1 }),
      sol({ id: 'b', version: 2 })
    ])
    expect(byVersion.map((s) => s.id)).toEqual(['b', 'a'])
  })

  it('does not mutate its input', () => {
    const input = [sol({ id: 'a', seq: 1 }), sol({ id: 'b', seq: 2 })]
    orderSolutionsNewestFirst(input)
    expect(input.map((s) => s.id)).toEqual(['a', 'b'])
  })
})

describe('currentSolutionOf', () => {
  it('is the newest version that is not superseded, else the newest', () => {
    const list = [
      sol({ id: 'c', status: 'superseded' }),
      sol({ id: 'b', status: 'rejected' }),
      sol({ id: 'a', status: 'superseded' })
    ]
    expect(currentSolutionOf(list)?.id).toBe('b')
    expect(currentSolutionOf([sol({ id: 'x', status: 'superseded' })])?.id).toBe('x')
    expect(currentSolutionOf([])).toBeUndefined()
  })
})

describe('solutionVersionNumbers', () => {
  it('uses seq, and the place in the order when there is none', () => {
    const withSeq = solutionVersionNumbers([sol({ id: 'b', seq: 7 }), sol({ id: 'a', seq: 6 })])
    expect(withSeq.get('b')).toBe(7)
    const without = solutionVersionNumbers([sol({ id: 'c' }), sol({ id: 'b' }), sol({ id: 'a' })])
    expect([without.get('a'), without.get('b'), without.get('c')]).toEqual([1, 2, 3])
  })
})

describe('versionNote', () => {
  it('is null for the first proposal and for blank feedback', () => {
    expect(versionNote(undefined)).toBeNull()
    expect(versionNote('')).toBeNull()
    expect(versionNote('  \n\t ')).toBeNull()
  })

  it('flattens whitespace and keeps short text whole', () => {
    expect(versionNote('Add a\n\nrollback   plan')).toBe('Add a rollback plan')
  })

  it('cuts at 80 characters with an ellipsis', () => {
    const note = versionNote('x'.repeat(200)) as string
    expect([...note]).toHaveLength(VERSION_NOTE_MAX_LENGTH)
    expect(note.endsWith('…')).toBe(true)
    expect(versionNote('y'.repeat(80))).toBe('y'.repeat(80))
  })

  it('counts characters, so an emoji is never split', () => {
    const note = versionNote('😀'.repeat(100)) as string
    expect([...note]).toHaveLength(VERSION_NOTE_MAX_LENGTH)
    expect(note).not.toContain('�')
  })
})

describe('versionBadge', () => {
  it('maps status to a text badge', () => {
    expect(versionBadge(sol({ id: 'a', status: 'superseded' }), false)).toBe('superseded')
    expect(versionBadge(sol({ id: 'a', status: 'rejected' }), true)).toBe('rejected')
    expect(versionBadge(sol({ id: 'a', status: 'chosen' }), true)).toBe('approved')
    expect(versionBadge(sol({ id: 'a', status: 'ready' }), true)).toBe('current')
    expect(versionBadge(sol({ id: 'a', status: 'ready' }), false)).toBeNull()
  })
})

describe('pages', () => {
  it('counts pages of 20 and finds the page of a version', () => {
    expect(versionPageCount(0)).toBe(1)
    expect(versionPageCount(20)).toBe(1)
    expect(versionPageCount(21)).toBe(2)
    expect(versionPageCount(100)).toBe(5)
    const list = Array.from({ length: 45 }, (_, i) => sol({ id: `s${i}` }))
    expect(versionPageOf(list, 's0')).toBe(0)
    expect(versionPageOf(list, 's19')).toBe(0)
    expect(versionPageOf(list, 's20')).toBe(1)
    expect(versionPageOf(list, 's44')).toBe(2)
    expect(versionPageOf(list, 'missing')).toBe(0)
  })
})
