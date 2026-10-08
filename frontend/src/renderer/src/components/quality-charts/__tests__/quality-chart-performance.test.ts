// @vitest-environment happy-dom

import { createElement } from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  buildDependencyGraph,
  buildHeatmapRows,
  buildSeverityCounts,
  buildTreemapItems,
  buildTrendSeries
} from '../../../test-support/quality-chart-fixtures'
import { orderByStrongComponents } from '../dependency-matrix-ordering'
import { DependencyMatrix } from '../DependencyMatrix'
import { HotspotHeatmap } from '../HotspotHeatmap'
import { MetricTreemap } from '../MetricTreemap'
import { squarify } from '../treemap-squarified-layout'

globalThis.IS_REACT_ACT_ENVIRONMENT = true

// Why: budgets are x5 the design targets because happy-dom/CI timing is noisy; the goal is to
// catch large regressions, not to benchmark. Measured values are logged for the PR record.
const SLACK = 5

function time<T>(run: () => T): { ms: number; value: T } {
  const start = performance.now()
  const value = run()
  return { ms: performance.now() - start, value }
}

function renderMs(node: React.ReactNode): number {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  const start = performance.now()
  act(() => root.render(node))
  const ms = performance.now() - start
  act(() => root.unmount())
  container.remove()
  return ms
}

beforeEach(() => {
  vi.stubGlobal('IntersectionObserver', undefined)
})
afterEach(() => {
  vi.unstubAllGlobals()
})

describe('fixtures', () => {
  it('are deterministic', () => {
    expect(buildTreemapItems()).toEqual(buildTreemapItems())
    expect(buildHeatmapRows()).toEqual(buildHeatmapRows())
    expect(buildDependencyGraph(150)).toEqual(buildDependencyGraph(150))
    expect(buildTrendSeries()).toEqual(buildTrendSeries())
    expect(buildSeverityCounts()).toEqual(buildSeverityCounts())
  })

  it('hit the budget ceilings', () => {
    expect(buildTreemapItems()).toHaveLength(400)
    expect(buildHeatmapRows()).toHaveLength(40)
    expect(buildHeatmapRows()[0].values).toHaveLength(6)
    expect(buildDependencyGraph(60).nodes).toHaveLength(60)
    expect(buildTrendSeries().series).toHaveLength(4)
  })
})

describe('chart performance budgets', () => {
  it('squarify(400) stays within the budget', () => {
    const items = buildTreemapItems(400).map((i) => ({ id: i.id, size: i.size }))
    const { ms } = time(() => squarify(items, { x: 0, y: 0, width: 1000, height: 600 }))
    console.info(`[perf] squarify(400) ${ms.toFixed(2)} ms (budget 16 ms, gate ${16 * SLACK} ms)`)
    expect(ms).toBeLessThanOrEqual(16 * SLACK)
  })

  it('orderByStrongComponents(150) stays within the budget', () => {
    const g = buildDependencyGraph(150, 0.05, 6)
    const { ms } = time(() =>
      orderByStrongComponents(
        g.nodes.map((n) => n.id),
        g.edges
      )
    )
    console.info(
      `[perf] orderByStrongComponents(150) ${ms.toFixed(2)} ms (budget 50 ms, gate ${50 * SLACK} ms)`
    )
    expect(ms).toBeLessThanOrEqual(50 * SLACK)
  })

  it('first render of the three grid charts stays within the budget', () => {
    const frame = (id: string) => ({ id, title: id })
    const treemap = renderMs(
      createElement(MetricTreemap, {
        frame: frame('t'),
        items: buildTreemapItems(400),
        sizeLabel: 'lines',
        intensityLabel: 'findings'
      })
    )
    const heatmap = renderMs(
      createElement(HotspotHeatmap, {
        frame: frame('h'),
        rows: buildHeatmapRows(40, 6),
        columns: Array.from({ length: 6 }, (_, i) => ({ key: `c${i}`, label: `C${i}` }))
      })
    )
    const g = buildDependencyGraph(60, 0.12, 4)
    const matrix = renderMs(
      createElement(DependencyMatrix, { frame: frame('d'), nodes: g.nodes, edges: g.edges })
    )
    console.info(
      `[perf] first render treemap(400) ${treemap.toFixed(1)} ms, heatmap(40x6) ${heatmap.toFixed(1)} ms, matrix(60, ${g.edges.length} edges) ${matrix.toFixed(1)} ms (budget 250 ms, gate ${250 * SLACK} ms)`
    )
    for (const ms of [treemap, heatmap, matrix]) {
      expect(ms).toBeLessThanOrEqual(250 * SLACK)
    }
  })
})
