import { describe, expect, it } from 'vitest'
import { collapseToLevel, levelForZoom, zoomPresentation } from './graph-zoom-levels'
import { gNode } from './graph-test-fixtures'

describe('levelForZoom', () => {
  it.each([
    [0.49, 'service'], [0.5, 'module'], [1.2, 'module'], [1.21, 'leaf'],
    [Number.NaN, 'module'], [-1, 'module']
  ])('zoom %s is %s', (zoom, level) => expect(levelForZoom(zoom)).toBe(level))
})

describe('collapseToLevel', () => {
  const nodes = [gNode('a', { group: 'api/auth', risk: 'high' }), gNode('b', { group: 'api/users' }), gNode('c', { group: 'web/ui' })]
  it('collapses by service', () => {
    const reps = collapseToLevel(nodes, [], 'service')
    expect(reps.map((r) => [r.label, r.count, r.maxRisk])).toEqual([['api', 2, 'high'], ['web', 1, 'low']])
  })
  it('collapses by module and keeps leaves as-is', () => {
    expect(collapseToLevel(nodes, [], 'module')).toHaveLength(3)
    expect(collapseToLevel(nodes, [], 'leaf').map((r) => r.id)).toEqual(['a', 'b', 'c'])
  })
})

describe('zoomPresentation', () => {
  it('drops detail when zoomed out and keeps edge signs from module level', () => {
    expect(zoomPresentation('service')).toMatchObject({ nodeDetail: 'minimal', showEdgeSigns: false })
    expect(zoomPresentation('module')).toMatchObject({ nodeDetail: 'compact', showEdgeSigns: true })
    expect(zoomPresentation('leaf')).toMatchObject({ nodeDetail: 'full', edgeOpacityFactor: 1 })
  })
})
