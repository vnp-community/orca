// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { QualityCall } from '../../../../store/slices/code-intel-quality-slice-context'

const clientCall = vi.fn()
vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() }
}))
vi.mock('../../../../runtime/code-intel-client', () => ({
  getCodeIntelClient: () => ({ call: clientCall })
}))
vi.mock('@/store', async () => {
  const { createCodeIntelQualityTestStore } =
    await import('../../../../test-support/code-intel-quality-test-store')
  return {
    useAppStore: createCodeIntelQualityTestStore((async () => ({
      ok: true,
      result: {}
    })) as QualityCall)
  }
})

import { useAppStore } from '@/store'
import { ENABLED_QUALITY_SUPPORT } from '../../../../test-support/code-intel-quality-test-store'
import { getReviewDockPanels } from '../../shell/review-dock-registry'
import '../quality-lens-registration'
import { TooltipProvider } from '../../../ui/tooltip'
import QualityFindingsDockPanel from './QualityFindingsDockPanel'

const finding = (i: number, extra: Record<string, unknown> = {}) => ({
  fingerprint: `fp-${i}`,
  ruleId: 'no-unused-vars',
  severity: 'error',
  category: 'lint',
  file: `src/f${i}.ts`,
  line: i + 1,
  endLine: i + 1,
  column: 0,
  endColumn: 0,
  message: `message ${i}`,
  tool: 'oxlint',
  toolVersion: '1',
  stepId: 's',
  inScope: true,
  ...extra
})

beforeEach(() => {
  clientCall.mockReset()
  vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(400)
  vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockReturnValue(600)
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
  )
  useAppStore.setState({
    codeIntelSupportState: ENABLED_QUALITY_SUPPORT,
    codeIntelQualityByWorktree: {}
  })
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

const view = (onOpenDiff = vi.fn()) =>
  render(
    <TooltipProvider>
      <QualityFindingsDockPanel worktreeId="wt" onOpenDiff={onOpenDiff} />
    </TooltipProvider>
  )

describe('quality findings dock panel', () => {
  it('is its own dock panel, only with the quality flag, separate from structural findings', () => {
    expect(getReviewDockPanels({ quality: false }).some((p) => p.id === 'quality-findings')).toBe(
      false
    )
    const ids = getReviewDockPanels({ quality: true }).map((p) => p.id)
    expect(ids).toContain('quality-findings')
  })

  it('renders nothing and calls nothing when quality is disabled', () => {
    useAppStore.setState({
      codeIntelSupportState: {
        state: 'enabled',
        effective: { codeIntelEnabled: true, qualityGateEnabled: false }
      }
    })
    const { container } = view()
    expect(container.innerHTML).toBe('')
    expect(clientCall).not.toHaveBeenCalled()
  })

  it('lists findings from quality.findings with the in-scope filter sent to the server', async () => {
    clientCall.mockImplementation(async (_w: string, method: string) => ({
      ok: true,
      result: method.endsWith('findings')
        ? {
            findings: [finding(1), finding(2)],
            totalCount: 2,
            truncated: false,
            outsideScopeCount: 3
          }
        : { runs: [] }
    }))
    view()
    await vi.waitFor(() => expect(screen.getAllByRole('row').length).toBe(2))
    const findingsCall = clientCall.mock.calls.find((c) => c[1] === 'codeIntel.quality.findings')
    expect(findingsCall?.[2]).toMatchObject({ inScope: true })
    expect(screen.getByText(/3 outside the changed scope/)).toBeTruthy()
  })

  it('shows message text as plain text', async () => {
    clientCall.mockImplementation(async (_w: string, method: string) => ({
      ok: true,
      result: method.endsWith('findings')
        ? { findings: [finding(1, { message: '<b>bold</b>' })], totalCount: 1 }
        : { runs: [] }
    }))
    const { container } = view()
    await vi.waitFor(() => expect(screen.getByText('<b>bold</b>')).toBeTruthy())
    expect(container.querySelector('b')).toBeNull()
    fireEvent.click(screen.getAllByRole('row')[0])
  })
})
