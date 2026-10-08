import { describe, expect, it } from 'vitest'
import { parseGraphChange, parseGraphPayload, parseGraphRisk } from './graph-wire-parsers'

describe('graph wire parsers', () => {
  it('maps missing or unknown risk to unknown, never low', () => {
    expect(parseGraphRisk(undefined)).toBe('unknown')
    expect(parseGraphRisk('severe')).toBe('unknown')
    expect(parseGraphRisk('high')).toBe('high')
    expect(parseGraphChange(undefined)).toBe('unchanged')
    expect(parseGraphChange('removed')).toBe('removed')
  })

  it('parses a valid payload, keeping unknown kind/status verbatim', () => {
    const p = parseGraphPayload(
      {
        lens: 'data',
        nodes: [
          { id: 'a', kind: 'service', label: 'A', group: 'g', risk: 'high', status: 'weird-status' },
          { id: 'b', kind: 'zzz', label: 'B' }
        ],
        edges: [{ from: 'a', to: 'b', kind: 'calls', change: 'added' }],
        totalNodes: 10,
        truncated: true,
        assessedAt: '2026-10-07T00:00:00Z',
        tool: 'codegraph',
        stale: true
      },
      'architecture'
    )
    expect(p.lens).toBe('architecture')
    expect(p.nodes[0]).toMatchObject({ status: 'weird-status', risk: 'high', group: 'g' })
    expect(p.nodes[1]).toMatchObject({ kind: 'zzz', risk: 'unknown', group: null, status: null })
    expect(p.edges[0].change).toBe('added')
    expect(p).toMatchObject({ totalNodes: 10, truncated: true, stale: true, tool: 'codegraph' })
  })

  it('drops orphan edges and duplicate nodes, counting dropped edges', () => {
    const p = parseGraphPayload(
      {
        nodes: [
          { id: 'a', label: 'A' },
          { id: 'a', label: 'A2' },
          { label: 'no id' }
        ],
        edges: [
          { from: 'a', to: 'ghost', kind: 'x' },
          { from: 'a', to: 'a', kind: 'self' }
        ]
      },
      'flow'
    )
    expect(p.nodes).toHaveLength(1)
    expect(p.nodes[0].label).toBe('A')
    expect(p.edges).toHaveLength(1)
    expect(p.droppedEdges).toBe(1)
  })

  it('converts meta.finding_ids and clamps totalNodes', () => {
    const p = parseGraphPayload(
      { nodes: [{ id: 'a', label: 'A', meta: { finding_ids: ['f1', 2] } }], totalNodes: 0 },
      'impact'
    )
    expect(p.nodes[0].meta?.findingIds).toEqual(['f1'])
    expect(p.totalNodes).toBe(1)
  })

  it('never throws on arbitrary input and treats only boolean true as truncated', () => {
    for (const raw of [null, undefined, [], 42, 'x', {}]) {
      const p = parseGraphPayload(raw, 'plan')
      expect(p.nodes).toEqual([])
      expect(p.lens).toBe('plan')
    }
    expect(parseGraphPayload({ truncated: 'true', stale: 1 }, 'plan')).toMatchObject({ truncated: false, stale: false })
  })
})
