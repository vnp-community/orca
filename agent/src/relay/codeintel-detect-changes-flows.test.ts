import { describe, it, expect, vi, beforeEach } from 'vitest'
import { findAffectedFlows, findAffectedClusters } from './codeintel-detect-changes-flows'

describe('codeintel-detect-changes-flows', () => {
  const fakeCypher = vi.fn()

  beforeEach(() => {
    fakeCypher.mockReset()
  })

  it('aggregates multiple symbols belonging to the same flow so each flow appears only once', async () => {
    fakeCypher.mockResolvedValue({
      rows: [
        { 's.id': 'sym1', 'p.id': 'flow_alpha', 'p.stepCount': 5, 'r.step': 3, 'p.label': 'Alpha Flow' },
        { 's.id': 'sym2', 'p.id': 'flow_alpha', 'p.stepCount': 5, 'r.step': 1, 'p.label': 'Alpha Flow' },
        { 's.id': 'sym3', 'p.id': 'flow_beta', 'p.stepCount': 2, 'r.step': 1, 'p.label': 'Beta Flow' }
      ]
    })

    const flows = await findAffectedFlows(['sym1', 'sym2', 'sym3'], {
      repoRoot: '/fake',
      runCypher: fakeCypher as any
    })

    expect(flows).toHaveLength(2)
    const alpha = flows.find(f => f.id === 'flow_alpha')
    expect(alpha).toBeDefined()
    expect(alpha?.changedSymbols).toEqual(['sym1', 'sym2'])
    expect(alpha?.earliestChangedStep).toBe(1)
    expect(alpha?.stepCount).toBe(5)
    expect(alpha?.label).toBe('Alpha Flow')
  })

  it('batches symbol queries in chunks of 300 and respects max flows cap of 200', async () => {
    const symbolIds = Array.from({ length: 700 }, (_, i) => `sym_${i}`)

    // Create 250 distinct flows
    const rows = Array.from({ length: 250 }, (_, i) => ({
      's.id': `sym_${i}`,
      'p.id': `flow_${i}`,
      'p.stepCount': 3,
      'r.step': 1,
      'p.label': `Flow ${i}`
    }))

    fakeCypher.mockImplementation((template, slots) => {
      // return rows matching ids
      const ids = slots.ids as string[]
      const matching = rows.filter(r => ids.includes(r['s.id']))
      return Promise.resolve({ rows: matching })
    })

    const flows = await findAffectedFlows(symbolIds, {
      repoRoot: '/fake',
      runCypher: fakeCypher as any
    })

    // 700 ids divided into chunks of 300 -> 3 batches
    expect(fakeCypher).toHaveBeenCalledTimes(3)
    // Capped at 200
    expect(flows).toHaveLength(200)
  })

  it('finds affected clusters when withClusters is true', async () => {
    fakeCypher.mockResolvedValue({
      rows: [
        { 's.id': 'sym1', 'c.id': 'community_1' },
        { 's.id': 'sym2', 'c.id': 'community_1' },
        { 's.id': 'sym3', 'c.id': 'community_2' }
      ]
    })

    const clusters = await findAffectedClusters(['sym1', 'sym2', 'sym3'], {
      repoRoot: '/fake',
      withClusters: true,
      runCypher: fakeCypher as any
    })

    expect(clusters).toHaveLength(2)
    const c1 = clusters.find(c => c.id === 'community_1')
    expect(c1?.changedSymbols).toEqual(['sym1', 'sym2'])
  })

  it('returns empty array when withClusters is false or symbols empty', async () => {
    const clusters = await findAffectedClusters(['sym1'], {
      repoRoot: '/fake',
      withClusters: false,
      runCypher: fakeCypher as any
    })
    expect(clusters).toEqual([])
  })
})
