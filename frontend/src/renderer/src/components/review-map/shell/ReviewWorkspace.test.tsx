// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useAppStore } from '@/store'
import { setReviewProgressApi } from '@/store/slices/review-progress'
import {
  REVIEW_LENS_DEFINITIONS,
  registerReviewLens,
  setReviewDrawerRenderer,
  type ReviewLensProps
} from '../review-lens-registry'
import type { ReviewDataApi } from '../review-shell-data'
import { makeOverlay, makeStatus } from '../review-test-data'
import type { ChangeOverlayView, ReviewError } from '../review-wire-types'
import ReviewWorkspace from './ReviewWorkspace'

const WT = 'repo1::/path/wt1'

let roCallbacks: ResizeObserverCallback[] = []
beforeEach(() => {
  roCallbacks = []
  globalThis.ResizeObserver = class {
    cb: ResizeObserverCallback
    constructor(cb: ResizeObserverCallback) {
      this.cb = cb
    }
    // Only the workspace root is driven by tests; panel-library observers stay inert.
    observe(el: Element) {
      if (el.getAttribute('data-testid') === 'review-workspace') {
        roCallbacks.push(this.cb)
      }
    }
    unobserve() {}
    disconnect() {}
  } as never
  window.localStorage.clear()
  useAppStore.setState({
    repos: [{ id: 'repo1', projectId: 'proj' } as never],
    worktreesByRepo: {
      repo1: [
        {
          id: WT,
          repoId: 'repo1',
          path: '/path/wt1',
          branch: 'feat/x',
          baseRef: 'origin/main'
        } as never
      ]
    },
    gitBranchCompareSummaryByWorktree: {},
    codeIntelSupportState: { state: 'enabled' },
    reviewUiByWorktree: {},
    reviewProgressByWorktree: {}
  })
})
afterEach(() => {
  cleanup()
  setReviewProgressApi(null)
  setReviewDrawerRenderer(null)
})

function setWidth(width: number): void {
  act(() =>
    roCallbacks.forEach((cb) =>
      cb([{ contentRect: { width } } as ResizeObserverEntry], {} as ResizeObserver)
    )
  )
}

type Overrides = {
  status?: ReviewError | ReturnType<typeof makeStatus>
  overlay?: ReviewError | ChangeOverlayView
}
const isErr = (v: unknown): v is ReviewError => typeof v === 'object' && v !== null && 'kind' in v

function makeApi(o: Overrides = {}) {
  const status = o.status ?? makeStatus()
  const overlay = o.overlay ?? makeOverlay()
  const api = {
    getStatus: vi.fn(async () =>
      isErr(status) ? { ok: false as const, error: status } : { ok: true as const, value: status }
    ),
    getChangeOverlay: vi.fn(async () =>
      isErr(overlay)
        ? { ok: false as const, error: overlay }
        : { ok: true as const, value: overlay }
    ),
    getReviewState: vi.fn(async (_w: string, k: { baseCommit: string; headCommit: string }) => ({
      ok: true as const,
      value: {
        ...k,
        readingProgress: { version: 1 as const, entries: {}, lastFocusedKey: null },
        version: 0
      }
    })),
    saveReviewState: vi.fn(async (_w: string, s: never, expected: number) => ({
      ok: true as const,
      value: { ...(s as object), version: expected + 1 } as never
    })),
    reindex: vi.fn(async () => ({ ok: true as const, value: { jobId: 'j', status: 'queued' } })),
    bindRepo: vi.fn(async () => ({ ok: true as const, value: makeStatus() }))
  }
  setReviewProgressApi(api as unknown as ReviewDataApi)
  return api
}

async function mount(
  o: Overrides = {},
  props: Partial<React.ComponentProps<typeof ReviewWorkspace>> = {}
) {
  const api = makeApi(o)
  const utils = render(
    <ReviewWorkspace worktreeId={WT} api={api as unknown as ReviewDataApi} {...props} />
  )
  return { api, ...utils }
}

describe('ReviewWorkspace', () => {
  it('shows the loading screen first, then header, seven chips, lens tabs and reading order', async () => {
    await mount()
    expect(screen.getByTestId('review-workspace')).toBeTruthy()
    await waitFor(() => expect(screen.getByTestId('reading-order')).toBeTruthy())
    expect(screen.getByTestId('review-header').textContent).toContain('feat/x')
    expect(screen.getByRole('toolbar').querySelectorAll('button')).toHaveLength(7)
    expect(screen.getAllByRole('tab')).toHaveLength(7)
    // The Impact lens is registered, so its lazy body (no centre yet) replaces the placeholder.
    await waitFor(() => expect(screen.getByTestId('impact-empty')).toBeTruthy())
  })

  it('asks the backend with an explicit mode derived from the default branch scope', async () => {
    const { api } = await mount()
    await waitFor(() => expect(api.getChangeOverlay).toHaveBeenCalled())
    expect(api.getChangeOverlay).toHaveBeenCalledWith(WT, { base: 'origin/main', mode: 'worktree' })
  })

  it('layout follows the tab width: three, two then one column', async () => {
    await mount()
    await waitFor(() => screen.getByTestId('reading-order'))
    expect(screen.getByTestId('review-workspace').getAttribute('data-layout')).toBe('three-column')
    setWidth(900)
    expect(screen.getByTestId('review-workspace').getAttribute('data-layout')).toBe('two-column')
    setWidth(500)
    expect(screen.getByTestId('review-workspace').getAttribute('data-layout')).toBe('one-column')
    expect(screen.getByRole('radio', { name: 'Reading order' })).toBeTruthy()
    fireEvent.click(screen.getByRole('radio', { name: 'Reading order' }))
    expect(screen.getByTestId('reading-order')).toBeTruthy()
  })

  it('no-binding shows an inline screen whose action rebinds then refreshes status', async () => {
    const { api } = await mount({
      overlay: { kind: 'no-binding', message: 'CODEINTEL_NO_DEV_SERVER' }
    })
    const btn = await screen.findByRole('button', { name: 'Try reconnecting' })
    fireEvent.click(btn)
    await waitFor(() => expect(api.bindRepo).toHaveBeenCalledWith(WT))
    expect(document.querySelector('[data-sonner-toast]')).toBeNull()
  })

  it('index MISSING offers to build the index', async () => {
    const { api } = await mount({
      status: makeStatus({ overall: 'MISSING' }),
      overlay: { kind: 'index-missing', message: 'x' }
    })
    fireEvent.click(await screen.findByRole('button', { name: 'Build index' }))
    await waitFor(() => expect(api.reindex).toHaveBeenCalledWith(WT, 'incremental'))
  })

  it('offline with cached data keeps the workspace and shows a banner', async () => {
    const { api } = await mount()
    await waitFor(() => screen.getByTestId('reading-order'))
    api.getChangeOverlay.mockResolvedValueOnce({
      ok: false,
      error: { kind: 'offline', message: 'down' }
    } as never)
    act(() => useAppStore.getState().triggerCodeIntelResync())
    // trigger a refetch through the banner flow
    const { publishCodeIntelEvent } = await import('@/lib/code-intel-event-bus')
    act(() => publishCodeIntelEvent({ event: 'changed', worktreeId: WT }))
    expect(await screen.findByText('New data is available.')).toBeTruthy()
    api.getChangeOverlay.mockResolvedValueOnce({
      ok: false,
      error: { kind: 'offline', message: 'down' }
    } as never)
    fireEvent.click(screen.getByRole('button', { name: 'Update now' }))
    await waitFor(() =>
      expect(document.querySelector('[data-banner="offline-cached"]')).toBeTruthy()
    )
    expect(screen.getByTestId('reading-order')).toBeTruthy()
  })

  it('a files chip filters the reading order; the same chip clears it; flow chips only switch lens', async () => {
    const overlay = makeOverlay({ affectedFlows: [{}] })
    await mount({ overlay })
    await waitFor(() => screen.getByTestId('reading-order'))
    fireEvent.click(screen.getByRole('button', { name: /3 files/ }))
    expect(screen.getByRole('button', { name: /3 files/ }).getAttribute('aria-pressed')).toBe(
      'true'
    )
    expect(screen.getByTestId('reading-filter-notice')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: /3 files/ }))
    expect(screen.queryByTestId('reading-filter-notice')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: /1 flows/ }))
    expect(useAppStore.getState().reviewUiByWorktree[WT].chipFilter).toBeNull()
    expect(useAppStore.getState().reviewUiByWorktree[WT].lens).toBe('dataflow')
  })

  it('selecting a symbol opens the drawer; ] and Esc close/open it, [ hides the reading order', async () => {
    await mount()
    await waitFor(() => screen.getByTestId('reading-order'))
    setReviewDrawerRenderer(({ selectedSymbolKey }) => (
      <div data-testid="drawer-body">{selectedSymbolKey}</div>
    ))
    act(() => useAppStore.getState().selectReviewSymbol(WT, 'sym-1'))
    expect(screen.getByTestId('drawer-body').textContent).toBe('sym-1')
    const root = screen.getByTestId('review-workspace')
    fireEvent.keyDown(root, { key: 'Escape' })
    expect(screen.queryByTestId('drawer-body')).toBeNull()
    fireEvent.keyDown(root, { key: ']' })
    expect(screen.getByTestId('drawer-body')).toBeTruthy()
    fireEvent.keyDown(root, { key: '[' })
    expect(screen.queryByTestId('reading-order')).toBeNull()
    expect(JSON.parse(window.localStorage.getItem('orca.review.layout.v1')!).leftOpen).toBe(false)
  })

  it('narrow tabs show the drawer as a sheet', async () => {
    await mount()
    await waitFor(() => screen.getByTestId('reading-order'))
    setWidth(900)
    setReviewDrawerRenderer(() => <div data-testid="drawer-body" />)
    act(() => useAppStore.getState().selectReviewSymbol(WT, 'sym-1'))
    expect(await screen.findByRole('dialog')).toBeTruthy()
  })

  it('marking a step read is saved once after the debounce, with the resolved base/head', async () => {
    const { api } = await mount()
    await waitFor(() =>
      expect(useAppStore.getState().reviewProgressByWorktree[WT]?.loadStatus).toBe('ready')
    )
    vi.useFakeTimers()
    fireEvent.click(screen.getAllByRole('checkbox', { name: /Mark src\/f1\.ts as read/ })[0])
    await act(async () => void vi.advanceTimersByTimeAsync(800))
    vi.useRealTimers()
    expect(api.saveReviewState).toHaveBeenCalledTimes(1)
    const sent = api.saveReviewState.mock.calls[0][1] as unknown as {
      baseCommit: string
      headCommit: string
      readingProgress: { entries: Record<string, { state: string }> }
    }
    expect(sent.baseCommit).toBe('m'.repeat(40))
    expect(sent.headCommit).toBe('h'.repeat(40))
    expect(sent.readingProgress.entries.s1.state).toBe('seen')
  })

  it('unsupported code-intel shows the unsupported screen and does not call the backend', async () => {
    useAppStore.setState({ codeIntelSupportState: { state: 'unsupported' } })
    const { api } = await mount()
    expect(document.querySelector('[data-screen="unsupported"]')).toBeTruthy()
    expect(api.getStatus).not.toHaveBeenCalled()
    expect(api.getChangeOverlay).not.toHaveBeenCalled()
  })

  it('an invalid base from the branch compare shows the scope guidance', async () => {
    useAppStore.setState({
      worktreesByRepo: { repo1: [{ id: WT, repoId: 'repo1', path: '/p', branch: 'b' } as never] },
      gitBranchCompareSummaryByWorktree: {
        [WT]: {
          baseRef: 'x',
          baseOid: null,
          compareRef: 'b',
          headOid: null,
          mergeBase: null,
          changedFiles: 0,
          status: 'invalid-base'
        }
      }
    } as never)
    await mount()
    expect(document.querySelector('[data-screen="scope-error"]')).toBeTruthy()
    expect(screen.getByText('The base branch was not found')).toBeTruthy()
  })

  it('Esc closes the drawer and returns focus to the element that opened it', async () => {
    const original = REVIEW_LENS_DEFINITIONS.find((l) => l.id === 'impact')!
    registerReviewLens({
      ...original,
      load: async () => ({
        default: (p: ReviewLensProps) => (
          <button type="button" data-testid="open-sym" onClick={() => p.onSelectSymbol('sym-9')}>
            open
          </button>
        )
      })
    })
    try {
      await mount()
      const opener = await screen.findByTestId('open-sym')
      opener.focus()
      fireEvent.click(opener)
      setReviewDrawerRenderer(() => <div data-testid="drawer-body" />)
      act(() => useAppStore.getState().setReviewDrawerOpen(WT, true))
      expect(await screen.findByTestId('drawer-body')).toBeTruthy()
      fireEvent.keyDown(screen.getByTestId('review-workspace'), { key: 'Escape' })
      expect(screen.queryByTestId('drawer-body')).toBeNull()
      expect(document.activeElement).toBe(opener)
    } finally {
      registerReviewLens(original)
    }
  })
})
