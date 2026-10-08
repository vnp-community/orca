// @vitest-environment happy-dom
import { act } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { mountChart, registerChartCleanup } from '../../quality-charts/__tests__/chart-test-mount'
import type { UseQualityCoverageResult } from '@/hooks/useQualityCoverage'
import { buildCoverageReport } from '../../../test-support/code-intel-quality-visualization-fake-data'

const state: { coverage: UseQualityCoverageResult } = { coverage: {} as UseQualityCoverageResult }
const startQualityRun = vi.fn()
const profile = {
  definition: {
    coverage: { required: true, diffCoverageWarnBelow: 0.8, diffCoverageFailBelow: 0.6 }
  }
}

vi.mock('@/hooks/useQualityCoverage', () => ({ useQualityCoverage: () => state.coverage }))
vi.mock('@/hooks/useQualityProfiles', () => ({
  useQualityProfiles: () => ({ profile, selected: 'full' })
}))
vi.mock('@/store', () => ({ useAppStore: { getState: () => ({ startQualityRun }) } }))

import { QualityCoveragePanel } from './QualityCoveragePanel'

registerChartCleanup()
const refetch = vi.fn()
const base = { support: 'enabled', error: null, refetch, reason: null } as const

function show(
  patch: Partial<UseQualityCoverageResult>,
  onOpenDiff = vi.fn()
): { c: HTMLElement; onOpenDiff: typeof onOpenDiff } {
  state.coverage = {
    ...base,
    status: 'ready',
    report: buildCoverageReport(),
    ...patch
  } as UseQualityCoverageResult
  return {
    c: mountChart(<QualityCoveragePanel worktreeId="wt" onOpenDiff={onOpenDiff} />),
    onOpenDiff
  }
}

describe('QualityCoveragePanel', () => {
  it.each(['measured', 'estimated'] as const)(
    'shows %s coverage in words, with the gauge from covered/total',
    (kind) => {
      const { c } = show({ report: buildCoverageReport(kind) })
      expect(c.querySelector('[data-coverage-source]')!.getAttribute('data-coverage-source')).toBe(
        kind
      )
      expect(c.textContent).toContain(
        kind === 'estimated'
          ? 'Estimated from test edges, not measured coverage.'
          : 'Measured coverage.'
      )
      expect(c.querySelector('[role="meter"]')!.getAttribute('aria-valuenow')).toBe('75')
      if (kind === 'estimated') {
        expect(c.textContent).toContain('Based on test-to-source edges.')
      }
    }
  )

  it('draws thresholds as percent from the 0..1 profile values', () => {
    const { c } = show({})
    expect(c.querySelector('[data-threshold="warn"]')!.getAttribute('style')).toContain('left: 80%')
    expect(c.querySelector('[data-threshold="fail"]')!.getAttribute('style')).toContain('left: 60%')
  })

  it('shows partial scope and the excluded files', () => {
    const { c } = show({ report: buildCoverageReport('partial') })
    expect(c.querySelector('[data-coverage-partial]')).not.toBeNull()
    expect(c.querySelector('[data-coverage-excluded]')!.textContent).toContain(
      'gen/schema.ts: generated file'
    )
  })

  it.each([
    ['diff null', null, 'Diff coverage is not available for this scope.'],
    [
      'no changed lines',
      {
        changedExecutable: 0,
        covered: 0,
        uncovered: 0,
        diffCoverage: null,
        partial: false,
        excludedFiles: []
      },
      'There are no changed executable lines'
    ],
    [
      'reason',
      {
        changedExecutable: 0,
        covered: 0,
        uncovered: 0,
        diffCoverage: null,
        partial: false,
        excludedFiles: [],
        reason: 'base missing'
      },
      'Reason: base missing'
    ]
  ])('never shows 0%% when diff is unusable (%s)', (_n, diff, text) => {
    const { c } = show({ report: { ...buildCoverageReport(), diff } as never })
    expect(c.querySelector('[role="meter"]')).toBeNull()
    expect(c.querySelector('#quality-coverage-diff [data-chart-empty]')!.textContent).toContain(
      text
    )
    expect(c.querySelector('#quality-coverage-diff')!.textContent).not.toContain('0%')
  })

  it('opens the diff of a treemap tile', () => {
    const { c, onOpenDiff } = show({})
    const tile = c.querySelector('[data-treemap] button')!
    act(() => (tile as HTMLButtonElement).click())
    expect(onOpenDiff).toHaveBeenCalledWith(expect.stringMatching(/^src\/area-\d\/file-\d+\.ts$/))
  })

  it('lists uncovered ranges and opens the diff at the first line', () => {
    const { c, onOpenDiff } = show({})
    const button = c.querySelector('[data-open-diff="src/area-0/file-0.ts:3"]') as HTMLButtonElement
    expect(button.textContent).toBe('Lines 3-5')
    act(() => button.click())
    expect(onOpenDiff).toHaveBeenCalledWith('src/area-0/file-0.ts', 3)
    expect(c.querySelector('[data-open-diff="src/area-0/file-0.ts:40"]')!.textContent).toBe(
      'Line 40'
    )
  })

  it('says showing X/Y when the report is truncated', () => {
    const { c } = show({ report: { ...buildCoverageReport(), truncated: true, totalCount: 900 } })
    expect(c.textContent).toContain('Showing 12/900')
  })

  it('explains report:null with the backend reason and offers a run when the profile needs coverage', () => {
    const { c } = show({ status: 'empty', report: null, reason: 'no artifact' })
    expect(c.textContent).toContain('Reason: no artifact')
    expect(c.querySelector('[role="meter"]')).toBeNull()
    const run = [...c.querySelectorAll('button')].find((b) => b.textContent === 'Run check')!
    act(() => run.click())
    expect(startQualityRun).toHaveBeenCalledWith('wt', { profile: 'full', scope: 'changed' })
  })

  it('renders loading, error with retry, unavailable and stale states', () => {
    expect(
      show({ status: 'loading', report: null }).c.querySelector('[data-status="loading"]')
    ).not.toBeNull()
    cleanupMounted()
    const error = show({ status: 'error', report: null, error: { kind: 'timeout' } as never }).c
    expect(error.textContent).toContain('The analysis is taking too long')
    act(() => (error.querySelector('[data-chart-error] button') as HTMLButtonElement).click())
    expect(refetch).toHaveBeenCalled()
    expect(
      show({ status: 'stale' }).c.querySelector('[data-quality-coverage] [data-chart-stale]')
    ).not.toBeNull()
    expect(show({ status: 'unavailable', report: null }).c.textContent).toContain(
      'needs a worktree'
    )
  })

  it('renders nothing when quality is disabled', () => {
    expect(show({ support: 'disabled' }).c.innerHTML).toBe('')
  })
})

function cleanupMounted(): void {
  document.body.replaceChildren()
}
