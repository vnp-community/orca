import { describe, expect, it } from 'vitest'
import { GRAPH_LENSES, getLensConfig, pickDefaultLens } from './graph-lens-registry'
import { emptyGraphPayload } from '../../../../shared/graph-wire-parsers'
import type { GraphLens, GraphPayload, GraphRisk } from '../../../../shared/graph-types'

function payload(lens: GraphLens, risks: GraphRisk[]): GraphPayload {
  return {
    ...emptyGraphPayload(lens),
    nodes: risks.map((risk, i) => ({ id: `${lens}${i}`, kind: 'k', label: 'n', group: null, risk, status: null }))
  }
}

describe('graph lens registry', () => {
  it('has seven unique lenses with label key and icon', () => {
    expect(new Set(GRAPH_LENSES.map((l) => l.id)).size).toBe(7)
    for (const l of GRAPH_LENSES) {
      expect(l.labelKey).toMatch(/^auto\.components\.graph\.lens\./)
      expect(l.Icon).toBeTruthy()
    }
    expect(getLensConfig('plan').source).toBe('client')
  })

  it('picks the backend lens with the highest risk', () => {
    expect(pickDefaultLens({ data: payload('data', ['low']), impact: payload('impact', ['critical']) })).toBe('impact')
  })

  it('falls back to flow when nothing is assessed', () => {
    expect(pickDefaultLens({})).toBe('flow')
    expect(pickDefaultLens({ data: payload('data', ['unknown']) })).toBe('flow')
  })

  it('breaks ties by registry order', () => {
    expect(pickDefaultLens({ impact: payload('impact', ['high']), contract: payload('contract', ['high']) })).toBe('contract')
  })
})
