import { describe, expect, it } from 'vitest'
import type { QualityGate, QualityRun } from '../../../../../shared/code-intel-quality-types'
import { indexDiffersFromHead, isGateStale } from './quality-stale-model'

const gate = (basedOn: Partial<QualityGate['basedOn']> = {}): QualityGate => ({
  verdict: 'pass',
  reasons: [],
  mode: 'inform',
  profile: 'full@repo/v1',
  basedOn: { runIds: ['r1'], indexCommit: 'idx', stale: false, ...basedOn }
})
const run = (headCommit: string): QualityRun => ({ id: 'r1', headCommit }) as QualityRun

describe('isGateStale', () => {
  it('is not stale for a fresh gate at the current HEAD', () => {
    expect(isGateStale({ gate: gate({ headCommit: 'h' }), currentHead: 'h' })).toEqual({
      stale: false,
      reasons: []
    })
  })

  it('trusts basedOn.stale', () => {
    expect(isGateStale({ gate: gate({ stale: true }), currentHead: 'h' }).reasons).toEqual([
      'backend'
    ])
  })

  it('detects HEAD movement from gate.headCommit or the run behind it', () => {
    expect(isGateStale({ gate: gate({ headCommit: 'old' }), currentHead: 'new' }).reasons).toEqual([
      'head-moved'
    ])
    expect(isGateStale({ gate: gate(), runs: [run('old')], currentHead: 'new' }).reasons).toEqual([
      'head-moved'
    ])
    expect(isGateStale({ gate: gate(), runs: [run('x')], currentHead: null }).stale).toBe(false)
  })

  it('reports a stale cache and combines reasons', () => {
    const s = isGateStale({
      gate: gate({ stale: true, headCommit: 'a' }),
      currentHead: 'b',
      cacheStale: true
    })
    expect(s.reasons).toEqual(['backend', 'head-moved', 'cache'])
  })

  it('no gate is never stale', () => {
    expect(isGateStale({ gate: null, cacheStale: true }).stale).toBe(false)
  })
})

describe('indexDiffersFromHead', () => {
  it('is informational and false without data', () => {
    expect(indexDiffersFromHead(gate({ indexCommit: 'a' }), 'b')).toBe(true)
    expect(indexDiffersFromHead(gate({ indexCommit: 'a' }), 'a')).toBe(false)
    expect(indexDiffersFromHead(null, 'a')).toBe(false)
    expect(indexDiffersFromHead(gate({ indexCommit: '' }), 'a')).toBe(false)
  })
})
