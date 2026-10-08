// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { QualityCall } from '../../../store/slices/code-intel-quality-slice-context'
import type { ReviewLensProps } from '../review-lens-registry'

const call = vi.fn<QualityCall>()
vi.mock('@/store', async () => {
  const { createCodeIntelQualityTestStore } =
    await import('../../../test-support/code-intel-quality-test-store')
  return { useAppStore: createCodeIntelQualityTestStore((...a) => call(...a)) }
})
vi.mock('@/lib/code-intel-worktree-selector', () => ({
  useCodeIntelSelector: () => ({
    state: 'ready',
    worktreeId: 'wt',
    projectId: 'p',
    environmentId: null
  })
}))
const blockLoads = vi.hoisted(() => ({ coverage: 0, trend: 0, hotspot: 0, dependency: 0 }))
vi.mock('./QualityCoveragePanel', () => {
  blockLoads.coverage += 1
  return { default: () => <div data-testid="coverage-panel" /> }
})
vi.mock('./QualityTrendPanel', () => {
  blockLoads.trend += 1
  return { default: () => <div /> }
})
vi.mock('./QualityHotspotPanel', () => {
  blockLoads.hotspot += 1
  return { default: () => <div /> }
})
vi.mock('./QualityDependencyPanel', () => {
  blockLoads.dependency += 1
  return { default: () => <div /> }
})

import { useAppStore } from '@/store'
import { ENABLED_QUALITY_SUPPORT } from '../../../test-support/code-intel-quality-test-store'
import { getReviewLenses } from '../review-lens-registry'
import QualityLens from './QualityLens'

const lensProps = {
  worktreeId: 'wt',
  scope: { kind: 'branch', baseRef: 'main', includeUncommitted: true },
  onOpenDiff: vi.fn()
} as unknown as ReviewLensProps

const gateBody = (verdict: string, runIds: string[]) => ({
  gate: {
    verdict,
    reasons: [],
    mode: 'inform',
    profile: 'full@repo/v1',
    basedOn: { runIds, indexCommit: 'i', stale: false }
  }
})

function reply(map: Record<string, unknown>): void {
  call.mockImplementation((_w, method) =>
    Promise.resolve({ ok: true as const, result: map[method] ?? {} })
  )
}

beforeEach(() => {
  call.mockReset()
  Object.keys(blockLoads).forEach((k) => ((blockLoads as Record<string, number>)[k] = 0))
  useAppStore.setState({
    codeIntelSupportState: ENABLED_QUALITY_SUPPORT,
    codeIntelQualityByWorktree: {},
    gitBranchCompareSummaryByWorktree: {}
  } as never)
})
afterEach(() => cleanup())

describe('quality lens registration', () => {
  it('is hidden without the quality flag and listed with it', () => {
    expect(getReviewLenses({ quality: false }).some((l) => l.id === 'quality')).toBe(false)
    const lens = getReviewLenses({ quality: true }).find((l) => l.id === 'quality')
    expect(lens?.requiresQuality).toBe(true)
    expect(typeof lens?.load).toBe('function')
  })
})

describe('QualityLens', () => {
  it('renders nothing and calls nothing when quality is disabled', async () => {
    useAppStore.setState({
      codeIntelSupportState: {
        state: 'enabled',
        effective: { codeIntelEnabled: true, qualityGateEnabled: false }
      }
    })
    const { container } = render(<QualityLens {...lensProps} />)
    await Promise.resolve()
    expect(container.innerHTML).toBe('')
    expect(call).not.toHaveBeenCalled()
  })

  it('shows the not-run screen when nothing has run, without a scorecard', async () => {
    reply({
      'codeIntel.quality.gate': gateBody('unknown', []),
      'codeIntel.quality.runs': { runs: [] }
    })
    render(<QualityLens {...lensProps} />)
    await vi.waitFor(() =>
      expect(screen.getByTestId('quality-state').getAttribute('data-kind')).toBe('not-run')
    )
    expect(screen.queryByTestId('quality-scorecard')).toBeNull()
  })

  it('shows the scorecard for a known gate and says a clean-looking run ran without findings only when it did', async () => {
    reply({
      'codeIntel.quality.gate': gateBody('pass', ['r1']),
      'codeIntel.quality.runs': {
        runs: [{ id: 'r1', status: 'succeeded', source: 'local', summary: {}, steps: [] }]
      }
    })
    render(<QualityLens {...lensProps} />)
    await vi.waitFor(() => expect(screen.getByTestId('quality-scorecard')).toBeTruthy())
    await vi.waitFor(() =>
      expect(screen.getByTestId('quality-state').getAttribute('data-kind')).toBe('ready-empty')
    )
    expect(screen.getByText('No findings in the checks that ran (full)')).toBeTruthy()
  })

  it('shows a persistent error with retry when the gate cannot be loaded', async () => {
    call.mockImplementation(() =>
      Promise.resolve({
        ok: false as const,
        error: {
          kind: 'tool-failed',
          code: null,
          message: '',
          data: null,
          retryable: false
        } as never
      })
    )
    render(<QualityLens {...lensProps} />)
    await vi.waitFor(() =>
      expect(screen.getByTestId('quality-state').getAttribute('data-kind')).toBe('load-error')
    )
    expect(screen.getByRole('button', { name: 'Retry' })).toBeTruthy()
  })

  it('loads each block only when it is opened and remembers the open state', async () => {
    reply({
      'codeIntel.quality.gate': gateBody('warn', ['r1']),
      'codeIntel.quality.runs': { runs: [] }
    })
    render(<QualityLens {...lensProps} />)
    await vi.waitFor(() => expect(screen.getByTestId('quality-scorecard')).toBeTruthy())
    expect(blockLoads).toEqual({ coverage: 0, trend: 0, hotspot: 0, dependency: 0 })
    fireEvent.click(screen.getByRole('button', { name: /Test coverage/ }))
    await vi.waitFor(() => expect(screen.getByTestId('coverage-panel')).toBeTruthy())
    expect(useAppStore.getState().codeIntelQualityByWorktree.wt.ui.openBlocks).toEqual(['coverage'])
    expect(blockLoads.trend).toBe(0)
  })

  it('a reason filter sets the quality source in the UI state', async () => {
    reply({
      'codeIntel.quality.gate': {
        gate: {
          ...gateBody('fail', ['r1']).gate,
          reasons: [
            { check: 'lint', observed: '1', threshold: '0', result: 'fail', category: 'lint' }
          ]
        }
      },
      'codeIntel.quality.runs': { runs: [] }
    })
    render(<QualityLens {...lensProps} />)
    await vi.waitFor(() =>
      expect(screen.getAllByRole('button', { name: 'Show findings' }).length).toBeGreaterThan(0)
    )
    fireEvent.click(screen.getAllByRole('button', { name: 'Show findings' })[0])
    expect(useAppStore.getState().codeIntelQualityByWorktree.wt.ui).toMatchObject({
      source: 'quality',
      category: ['lint']
    })
  })

  it('an active run shows the toolbar progress while keeping the last scorecard', async () => {
    reply({
      'codeIntel.quality.gate': gateBody('pass', ['r1']),
      'codeIntel.quality.runs': { runs: [] }
    })
    render(<QualityLens {...lensProps} />)
    await vi.waitFor(() => expect(screen.getByTestId('quality-scorecard')).toBeTruthy())
    useAppStore.setState((s) => ({
      codeIntelQualityByWorktree: {
        ...s.codeIntelQualityByWorktree,
        wt: {
          ...s.codeIntelQualityByWorktree.wt,
          run: {
            runId: 'r2',
            profile: 'full',
            scope: 'changed',
            phase: 'running',
            stage: 'lint',
            percent: null,
            message: '',
            startedAt: 0
          }
        }
      }
    }))
    await vi.waitFor(() => expect(screen.getByTestId('quality-run-progress')).toBeTruthy())
    expect(screen.getByTestId('quality-scorecard')).toBeTruthy()
  })
})
