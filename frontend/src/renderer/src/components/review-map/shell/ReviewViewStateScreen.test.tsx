// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { ReviewScreen } from '../review-view-state'
import type { ReviewBanner } from '../review-view-state'
import { ReviewStateBanners } from './ReviewStateBanners'
import { ReviewViewStateScreen, type ReviewScreenActions } from './ReviewViewStateScreen'

afterEach(cleanup)

const actions = (): ReviewScreenActions => ({
  onReindex: vi.fn(),
  onRebind: vi.fn(),
  onRetry: vi.fn(),
  onCloseTab: vi.fn(),
  onOpenScopePicker: vi.fn()
})
const err = (kind: 'no-binding' | 'tool-failed', message = 'm') => ({ kind, message })

describe('ReviewViewStateScreen', () => {
  const cases: [ReviewScreen, keyof ReviewScreenActions | null, string | RegExp][] = [
    [{ id: 'unsupported' }, 'onCloseTab', 'Close tab'],
    [{ id: 'disabled' }, 'onCloseTab', 'Close tab'],
    [{ id: 'scope-error', reason: 'invalid-base' }, 'onOpenScopePicker', 'Choose what to compare'],
    [{ id: 'no-binding', error: err('no-binding') }, 'onRebind', 'Try reconnecting'],
    [{ id: 'forbidden' }, null, ''],
    [{ id: 'loading-initial' }, null, ''],
    [{ id: 'tool-unavailable' }, 'onRetry', 'Try again'],
    [{ id: 'repo-not-registered' }, 'onReindex', 'Build index'],
    [{ id: 'path-not-allowed' }, 'onRetry', 'Try again'],
    [{ id: 'index-missing' }, 'onReindex', 'Build index'],
    [{ id: 'index-building', percent: null, stage: null }, null, ''],
    [{ id: 'offline' }, 'onRetry', 'Try again'],
    [{ id: 'error', error: err('tool-failed', 'oops') }, 'onRetry', 'Try again'],
    [{ id: 'no-changes', reason: null }, 'onOpenScopePicker', 'Choose what to compare']
  ]
  it.each(cases)(
    '%j renders its screen with the right primary action',
    (screenState, action, label) => {
      const a = actions()
      const { container } = render(<ReviewViewStateScreen screen={screenState} actions={a} />)
      expect(container.querySelector(`[data-screen="${screenState.id}"]`)).toBeTruthy()
      if (action) {
        fireEvent.click(screen.getByRole('button', { name: label }))
        expect(a[action]).toHaveBeenCalledTimes(1)
      } else {
        expect(
          screen.queryAllByRole('button').filter((b) => b.textContent !== 'Copy details')
        ).toHaveLength(0)
      }
    }
  )
  it('building with unknown percent never shows 0%', () => {
    render(
      <ReviewViewStateScreen
        screen={{ id: 'index-building', percent: null, stage: null }}
        actions={actions()}
      />
    )
    expect(document.body.textContent).not.toMatch(/0%/)
  })
  it('error details are copyable; path-not-allowed shows no detail at all', () => {
    render(
      <ReviewViewStateScreen
        screen={{ id: 'error', error: err('tool-failed', 'secret detail') }}
        actions={actions()}
      />
    )
    expect(screen.getByText('Copy details')).toBeTruthy()
    cleanup()
    render(<ReviewViewStateScreen screen={{ id: 'path-not-allowed' }} actions={actions()} />)
    expect(screen.queryByText('Copy details')).toBeNull()
  })
})

describe('ReviewStateBanners', () => {
  const banners: ReviewBanner[] = [
    { id: 'stale', indexedCommit: 'abcdef123', headCommit: '1234567890' },
    { id: 'truncated', shown: 100, total: 900 },
    { id: 'offline-cached' },
    { id: 'new-data' }
  ]
  it('renders each banner with its text and no toast', () => {
    const onApply = vi.fn()
    const { container } = render(
      <ReviewStateBanners
        banners={banners}
        onRetry={vi.fn()}
        onApplyNewData={onApply}
        onRefreshIndex={vi.fn()}
      />
    )
    expect(container.querySelectorAll('[data-banner]')).toHaveLength(4)
    expect(screen.getByText(/Showing 100 of 900 files/)).toBeTruthy()
    expect(screen.getByText(/abcdef1 to 1234567/)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Update now' }))
    expect(onApply).toHaveBeenCalled()
  })
  it('renders nothing for an empty list', () => {
    const { container } = render(
      <ReviewStateBanners
        banners={[]}
        onRetry={vi.fn()}
        onApplyNewData={vi.fn()}
        onRefreshIndex={vi.fn()}
      />
    )
    expect(container.firstChild).toBeNull()
  })
})
