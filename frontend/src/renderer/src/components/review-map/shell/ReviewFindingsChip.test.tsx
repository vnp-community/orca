// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'

const h = vi.hoisted(() => ({ result: null as unknown, calls: [] as unknown[] }))
vi.mock('../../../hooks/useCodeIntelFindings', () => ({
  useCodeIntelFindings: (...args: unknown[]) => {
    h.calls.push(args)
    return h.result
  }
}))

import { ReviewFindingsChip } from './ReviewFindingsChip'
import { ReviewBottomDock } from './ReviewBottomDock'
import { registerReviewDockPanel } from './review-dock-registry'
import { requestReviewDockPanel, subscribeReviewDockFocus } from './review-dock-focus'

const finding = (key: string, dismissed: unknown = null) => ({ findingKey: key, dismissed })

beforeEach(() => {
  h.calls.length = 0
  h.result = {
    status: 'success',
    findings: [finding('a'), finding('b'), finding('c', { state: 'ignored' })],
    hasNextPage: false
  }
})
afterEach(() => {
  cleanup()
  window.localStorage.clear()
})

describe('ReviewFindingsChip', () => {
  it('counts open findings in the changed scope with the panel default filters', () => {
    render(<ReviewFindingsChip worktreeId="wt" environmentId={null} />)
    expect(screen.getByRole('button').textContent).toBe('2 findings')
    expect(h.calls[0]).toEqual([
      'wt',
      null,
      { scope: 'changed', includeDismissed: false, severities: [] }
    ])
  })

  it('shows "+" when more pages exist and hides until the first load succeeds', () => {
    h.result = { status: 'success', findings: [finding('a')], hasNextPage: true }
    const { rerender } = render(<ReviewFindingsChip worktreeId="wt" environmentId={null} />)
    expect(screen.getByRole('button').textContent).toBe('1+ findings')
    h.result = { status: 'loading', findings: [], hasNextPage: false }
    rerender(<ReviewFindingsChip worktreeId="wt" environmentId={null} />)
    expect(screen.queryByRole('button')).toBeNull()
  })

  it('is disabled at zero and requests the findings dock panel on click', () => {
    const seen = vi.fn()
    const off = subscribeReviewDockFocus(seen)
    render(<ReviewFindingsChip worktreeId="wt" environmentId={null} />)
    fireEvent.click(screen.getByRole('button'))
    expect(seen).toHaveBeenCalledWith('wt', 'findings')
    off()
    cleanup()
    h.result = { status: 'success', findings: [], hasNextPage: false }
    render(<ReviewFindingsChip worktreeId="wt" environmentId={null} />)
    expect((screen.getByRole('button') as HTMLButtonElement).disabled).toBe(true)
  })

  it('opens the matching dock panel for the same worktree only', () => {
    registerReviewDockPanel({
      id: 'chip-test-panel',
      order: 99,
      labelKey: 'x.chipTest',
      labelFallback: 'ChipTest',
      render: () => <div data-testid="chip-test-body" />
    })
    render(
      <ReviewBottomDock
        flags={{ quality: false }}
        panelProps={{ worktreeId: 'wt' } as Parameters<typeof ReviewBottomDock>[0]['panelProps']}
      />
    )
    expect(screen.getByTestId('review-dock').getAttribute('data-open')).toBe('false')
    act(() => requestReviewDockPanel('other', 'chip-test-panel'))
    expect(screen.getByTestId('review-dock').getAttribute('data-open')).toBe('false')
    act(() => requestReviewDockPanel('wt', 'chip-test-panel'))
    expect(screen.getByTestId('review-dock').getAttribute('data-open')).toBe('true')
    expect(screen.getByTestId('chip-test-body')).toBeTruthy()
  })
})
