// @vitest-environment happy-dom
import { act } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { mountChart, registerChartCleanup } from '../../quality-charts/__tests__/chart-test-mount'
import type { UseQualityDependencyMatrixResult } from '@/hooks/useQualityDependencyMatrix'
import { buildStructureGraph } from '../../../test-support/code-intel-quality-visualization-fake-data'

const state: { dsm: UseQualityDependencyMatrixResult } = {
  dsm: {} as UseQualityDependencyMatrixResult
}
vi.mock('@/hooks/useQualityDependencyMatrix', () => ({
  useQualityDependencyMatrix: () => state.dsm
}))

import { QualityDependencyPanel } from './QualityDependencyPanel'

registerChartCleanup()
const refetch = vi.fn()

function show(
  patch: Partial<UseQualityDependencyMatrixResult>,
  onOpenLens: ((id: string) => void) | null = vi.fn()
): HTMLElement {
  state.dsm = {
    support: 'enabled',
    status: 'ready',
    graph: buildStructureGraph('cycle'),
    truncated: false,
    totalCount: 0,
    error: null,
    refetch,
    ...patch
  }
  return mountChart(
    <QualityDependencyPanel
      worktreeId="wt"
      onOpenDiff={vi.fn()}
      onOpenLens={onOpenLens ?? undefined}
    />
  )
}

describe('QualityDependencyPanel', () => {
  it('describes how to read the matrix and marks the cycle with an outline and a triangle', () => {
    const c = show({})
    expect(c.textContent).toContain('Rows are importing modules, columns are imported modules.')
    expect(c.querySelector('[data-cycle-block]')).not.toBeNull()
    expect(c.querySelector('[data-backward]')).not.toBeNull()
  })

  it('draws no cycle block for a DAG', () => {
    const c = show({ graph: buildStructureGraph('dag') })
    expect(c.querySelector('[data-cycle-block]')).toBeNull()
    expect(c.querySelector('[data-matrix]')).not.toBeNull()
  })

  it('shows at most 60 rows with an Other row and says X/Y for 61 or more modules', () => {
    const c = show({ graph: buildStructureGraph('large') })
    expect(c.querySelectorAll('[data-matrix] g > text').length).toBeGreaterThan(0)
    expect(c.textContent).toContain('Other (31)')
    expect(c.textContent).toContain('Showing 59/90')
  })

  it('reports a graph the backend truncated', () => {
    const c = show({ truncated: true, totalCount: 400 })
    expect(c.textContent).toContain('The backend returned 8 of 400 modules.')
  })

  it('lists the edges of a selected cell and opens the Structure lens when possible', () => {
    const onOpenLens = vi.fn()
    const c = show({}, onOpenLens)
    const matrix = c.querySelector('svg[data-matrix]')!
    act(
      () =>
        void matrix.dispatchEvent(
          new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
        )
    )
    const panel = c.querySelector('[data-dependency-edges]')!
    expect(panel.textContent).toMatch(/imports .* \(\d+\)/)
    act(() =>
      (
        [...panel.querySelectorAll('button')].find(
          (b) => b.textContent === 'Open in Structure lens'
        ) as HTMLButtonElement
      ).click()
    )
    expect(onOpenLens).toHaveBeenCalledWith('structure')
  })

  it('has no Structure lens button when the lens cannot be opened', () => {
    const c = show({}, null)
    const matrix = c.querySelector('svg[data-matrix]')!
    act(
      () =>
        void matrix.dispatchEvent(
          new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
        )
    )
    expect(c.querySelector('[data-dependency-edges]')).not.toBeNull()
    expect(
      [...c.querySelectorAll('button')].some((b) => b.textContent === 'Open in Structure lens')
    ).toBe(false)
  })

  it('explains an empty graph and other states', () => {
    expect(
      show({ status: 'empty', graph: { nodes: [], edges: [] } }).querySelector(
        '[data-chart-empty]'
      )!.textContent
    ).toContain('No dependency data yet')
    expect(
      show({ status: 'loading', graph: null }).querySelector('[data-status="loading"]')
    ).not.toBeNull()
    const c = show({ status: 'error', graph: null, error: { kind: 'too-large' } as never })
    act(() => (c.querySelector('[data-chart-error] button') as HTMLButtonElement).click())
    expect(refetch).toHaveBeenCalled()
    expect(
      show({ status: 'stale' }).querySelector('[data-quality-dependency] [data-chart-stale]')
    ).not.toBeNull()
    expect(show({ support: 'unsupported' }).innerHTML).toBe('')
  })
})
