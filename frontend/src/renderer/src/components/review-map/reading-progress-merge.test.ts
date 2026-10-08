import { describe, expect, it } from 'vitest'
import {
  countSeen,
  measureReadingProgressBytes,
  mergeReadingProgress,
  pruneReadingProgress
} from './reading-progress-merge'
import type { ReadingProgress } from './review-wire-types'

const p = (entries: ReadingProgress['entries'], last: string | null = null): ReadingProgress => ({
  version: 1,
  entries,
  lastFocusedKey: last
})

describe('mergeReadingProgress', () => {
  it('per key picks the larger at; newer unseen beats older seen (no resurrection)', () => {
    const local = p({ a: { state: 'unseen', at: 20 }, b: { state: 'seen', at: 5 } })
    const remote = p({
      a: { state: 'seen', at: 10 },
      b: { state: 'unseen', at: 9 },
      c: { state: 'seen', at: 1 }
    })
    const merged = mergeReadingProgress(local, remote)
    expect(merged.entries).toEqual({
      a: { state: 'unseen', at: 20 },
      b: { state: 'unseen', at: 9 },
      c: { state: 'seen', at: 1 }
    })
  })
  it('does not mutate inputs', () => {
    const local = p({ a: { state: 'seen', at: 1 } })
    const remote = p({ a: { state: 'unseen', at: 2 } })
    const snap = JSON.stringify([local, remote])
    mergeReadingProgress(local, remote)
    expect(JSON.stringify([local, remote])).toBe(snap)
  })
  it('lastFocusedKey comes from the side with the newer focused entry', () => {
    const local = p({ a: { state: 'seen', at: 1 } }, 'a')
    const remote = p({ b: { state: 'seen', at: 9 } }, 'b')
    expect(mergeReadingProgress(local, remote).lastFocusedKey).toBe('b')
  })
})

describe('pruneReadingProgress', () => {
  const big = (): ReadingProgress => {
    const entries: ReadingProgress['entries'] = {}
    for (let i = 0; i < 400; i++) {
      entries[`step-key-${'x'.repeat(60)}-${i}`] = { state: i % 2 ? 'seen' : 'unseen', at: i }
    }
    return p(entries)
  }
  it('is a no-op when it fits', () => {
    const small = p({ a: { state: 'seen', at: 1 } })
    expect(pruneReadingProgress(small)).toEqual({
      progress: small,
      droppedUnseen: 0,
      stillTooLarge: false
    })
  })
  it('drops oldest unseen tombstones first and keeps every seen', () => {
    const progress = big()
    const budget = measureReadingProgressBytes(progress) - 4000
    const res = pruneReadingProgress(progress, budget)
    expect(res.droppedUnseen).toBeGreaterThan(0)
    expect(res.stillTooLarge).toBe(false)
    expect(measureReadingProgressBytes(res.progress)).toBeLessThanOrEqual(budget)
    const seenBefore = Object.values(progress.entries).filter((e) => e.state === 'seen').length
    expect(Object.values(res.progress.entries).filter((e) => e.state === 'seen')).toHaveLength(
      seenBefore
    )
    expect(Object.keys(progress.entries)).toHaveLength(400)
  })
  it('reports stillTooLarge when seen alone does not fit', () => {
    expect(pruneReadingProgress(big(), 100).stillTooLarge).toBe(true)
  })
  it('measures multi-byte characters by bytes', () => {
    const one = p({ あ: { state: 'seen', at: 1 } })
    expect(measureReadingProgressBytes(one)).toBeGreaterThan(JSON.stringify(one).length)
  })
})

describe('countSeen', () => {
  it('counts only listed keys', () => {
    expect(
      countSeen(p({ a: { state: 'seen', at: 1 }, b: { state: 'unseen', at: 1 } }), ['a', 'b', 'c'])
    ).toBe(1)
  })
})
