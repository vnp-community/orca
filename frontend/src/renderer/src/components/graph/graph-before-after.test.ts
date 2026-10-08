import { describe, expect, it } from 'vitest'
import { applyChangeView, hasChangeAxis } from './graph-before-after'
import { gEdge, gNode, gPayload } from './graph-test-fixtures'

const payload = gPayload(
  [gNode('a'), gNode('b'), gNode('c', { status: 'added' })],
  [gEdge('a', 'b', { change: 'removed' }), gEdge('a', 'c', { change: 'added' })]
)

describe('applyChangeView', () => {
  it('after keeps every edge and dims removed ones', () => {
    const v = applyChangeView(payload, 'after')
    expect(v.edges).toHaveLength(2)
    expect(v.edges.find((e) => e.to === 'b')?.dim).toBe(true)
  })
  it('before hides added edges/nodes and draws removed edges as normal', () => {
    const v = applyChangeView(payload, 'before')
    expect(v.edges).toEqual([{ from: 'a', to: 'b', kind: 'calls', change: 'unchanged' }])
    expect(v.nodes.map((n) => n.id)).toEqual(['a', 'b'])
  })
  it('does not mutate the input and passes through graphs without a change axis', () => {
    const snapshot = JSON.stringify(payload)
    applyChangeView(payload, 'before')
    expect(JSON.stringify(payload)).toBe(snapshot)
    const plain = gPayload([gNode('a')], [gEdge('a', 'a')])
    expect(hasChangeAxis(plain)).toBe(false)
    expect(applyChangeView(plain, 'before')).toBe(plain)
  })
})
