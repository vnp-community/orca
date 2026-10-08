// @vitest-environment happy-dom
import { act } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { mountChart, registerChartCleanup } from '../../quality-charts/__tests__/chart-test-mount'
import type { UseQualityHotspotsResult } from '@/hooks/useQualityHotspots'
import type { Finding } from '../../../../../shared/code-intel-findings-types'
import { buildHotspotFindings } from '../../../test-support/code-intel-quality-visualization-fake-data'

const state: { hotspots: UseQualityHotspotsResult } = { hotspots: {} as UseQualityHotspotsResult }
vi.mock('@/hooks/useQualityHotspots', () => ({ useQualityHotspots: () => state.hotspots }))

import { QualityHotspotPanel } from './QualityHotspotPanel'

registerChartCleanup()
const refetch = vi.fn()

function show(
  patch: Partial<UseQualityHotspotsResult>,
  onOpenDiff = vi.fn()
): { c: HTMLElement; onOpenDiff: typeof onOpenDiff } {
  const findings = patch.findings ?? buildHotspotFindings(6)
  state.hotspots = {
    support: 'enabled',
    status: 'ready',
    findings,
    totalCount: findings.length,
    truncated: false,
    error: null,
    refetch,
    ...patch
  }
  return {
    c: mountChart(<QualityHotspotPanel worktreeId="wt" onOpenDiff={onOpenDiff} />),
    onOpenDiff
  }
}

const headers = (c: HTMLElement): string[] =>
  [...c.querySelectorAll('[role="columnheader"]')].map((h) => h.textContent!)

describe('QualityHotspotPanel', () => {
  it('derives the columns from the data and has no complexity column', () => {
    const { c } = show({})
    expect(headers(c)).toEqual(['Label', 'authors', 'churn', 'recentFixes'])
    expect(c.textContent!.toLowerCase()).not.toContain('complexity')
  })

  it('shows a dash for a missing number and no total score', () => {
    const { c } = show({})
    const row = [...c.querySelectorAll('[role="row"]')].find((r) =>
      r.textContent!.includes('file-1.ts')
    )!
    expect([...row.querySelectorAll('[role="gridcell"]')].map((x) => x.textContent)).toEqual([
      '2',
      '29',
      '—'
    ])
    expect(c.textContent!.toLowerCase()).not.toContain('score:')
  })

  it('states that the source is structure findings, not a quality score', () => {
    expect(show({}).c.textContent).toContain('This is not a quality score.')
  })

  it('cuts 41 rows to 40 and says 40/41', () => {
    const { c } = show({ findings: buildHotspotFindings(41) })
    expect(c.textContent).toContain('Showing 40/41')
    expect(c.querySelectorAll('[data-heatmap] [role="row"]')).toHaveLength(41)
  })

  it('notes files that exist on the backend but were not loaded', () => {
    expect(show({ totalCount: 130 }).c.textContent).toContain('124 more files exist on the backend')
  })

  it('explains an empty result', () => {
    const { c } = show({ status: 'empty', findings: [] })
    expect(c.querySelector('[data-chart-empty]')!.textContent).toContain('No hotspot data yet.')
  })

  it('selects a row, shows plain-text owner and opens the diff', () => {
    const findings = buildHotspotFindings(2)
    // Why: rows are ordered by the first metric column, so file-1 (authors 2) is the first row.
    findings[1] = {
      ...findings[1],
      owner: { source: 'codeowners', names: ['<b>@evil</b>'] }
    } as Finding
    const { c, onOpenDiff } = show({ findings })
    const grid = c.querySelector('[role="grid"]')!
    act(
      () =>
        void grid.dispatchEvent(
          new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
        )
    )
    const selected = c.querySelector('[data-hotspot-selected]')!
    expect(selected.textContent).toContain('Owner: <b>@evil</b>')
    expect(selected.querySelector('b')).toBeNull()
    act(() => (selected.querySelector('button') as HTMLButtonElement).click())
    expect(onOpenDiff).toHaveBeenCalledWith('src/area/file-1.ts')
  })

  it('keeps metrics beyond six columns in a full table', () => {
    const metrics = Object.fromEntries(Array.from({ length: 8 }, (_, i) => [`m${i}`, i]))
    const findings = [{ ...buildHotspotFindings(1)[0], metrics }] as Finding[]
    const { c } = show({ findings })
    expect(headers(c)).toHaveLength(7)
    const all = c.querySelector('[data-hotspot-all-metrics]')!
    expect(all.textContent).toContain('All metrics (8)')
    expect(all.querySelectorAll('thead th')).toHaveLength(9)
  })

  it('renders loading, error with retry and renders nothing when disabled', () => {
    expect(
      show({ status: 'loading', findings: [] }).c.querySelector('[data-status="loading"]')
    ).not.toBeNull()
    const { c } = show({ status: 'error', findings: [], error: { kind: 'timeout' } as never })
    act(() => (c.querySelector('[data-chart-error] button') as HTMLButtonElement).click())
    expect(refetch).toHaveBeenCalled()
    expect(show({ support: 'disabled' }).c.innerHTML).toBe('')
  })
})
