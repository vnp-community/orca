// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useAppStore } from '@/store'
import { setReviewProgressApi } from '@/store/slices/review-progress'
import { setReviewOpenSource } from '@/lib/review-open-source'
import type { ReviewDataApi } from '../review-shell-data'
import { makeOverlay, makeStatus } from '../review-test-data'
import { registerReviewDockPanel } from './review-dock-registry'
import { resetReviewLensAvailability, setReviewLensUnavailable } from './review-lens-availability'
import ReviewWorkspace from './ReviewWorkspace'
import { publishTurnSaved } from '../turns/review-turn-recorder-bus'

const h = vi.hoisted(() => ({
  decide: vi.fn(),
  opened: vi.fn(),
  lensViewed: vi.fn()
}))
vi.mock('@/lib/review-decision-tracker', () => ({
  decide: h.decide,
  noteReviewOpened: vi.fn(),
  registerCompletion: vi.fn()
}))
vi.mock('@/lib/review-telemetry', async (orig) => ({
  ...((await orig()) as object),
  trackReviewOpened: h.opened,
  trackReviewLensViewed: h.lensViewed
}))

const WT = 'repo1::/path/wt1'
const marker = (turnId: string, endedAt: number, hash: string) => ({
  turnId,
  worktreeId: WT,
  paneKey: 'p',
  agentType: 'claude',
  startedAt: null,
  endedAt,
  baseOid: null,
  headOid: null,
  files: [{ p: 'src/a.ts', h: hash }],
  overlayAvailable: false
})

function makeApi(turnMarkers: unknown[] = []) {
  const api = {
    getStatus: vi.fn(async () => ({ ok: true as const, value: makeStatus() })),
    getChangeOverlay: vi.fn(async () => ({ ok: true as const, value: makeOverlay() })),
    getReviewState: vi.fn(async (_w: string, k: { baseCommit: string; headCommit: string }) => ({
      ok: true as const,
      value: {
        ...k,
        readingProgress: { version: 1 as const, entries: {}, lastFocusedKey: null },
        turnMarkers: k.baseCommit === '' ? turnMarkers : [],
        version: 0
      }
    })),
    saveReviewState: vi.fn(async (_w: string, s: never, v: number) => ({
      ok: true as const,
      value: { ...(s as object), version: v + 1 } as never
    })),
    reindex: vi.fn(),
    bindRepo: vi.fn()
  }
  setReviewProgressApi(api as unknown as ReviewDataApi)
  return api
}

beforeEach(() => {
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as never
  window.localStorage.clear()
  vi.clearAllMocks()
  resetReviewLensAvailability()
  useAppStore.setState({
    repos: [{ id: 'repo1', projectId: 'proj' } as never],
    worktreesByRepo: {
      repo1: [
        {
          id: WT,
          repoId: 'repo1',
          path: '/path/wt1',
          branch: 'feat/x',
          baseRef: 'origin/main',
          projectId: 'proj'
        } as never
      ]
    },
    gitBranchCompareSummaryByWorktree: {},
    codeIntelSupportState: {
      state: 'enabled',
      effective: { codeIntelEnabled: true, qualityGateEnabled: false, aiReviewEnabled: false }
    } as never,
    reviewUiByWorktree: {},
    reviewProgressByWorktree: {}
  })
})
afterEach(() => {
  cleanup()
  setReviewProgressApi(null)
})

async function mount(api = makeApi()) {
  render(<ReviewWorkspace worktreeId={WT} api={api as unknown as ReviewDataApi} />)
  await waitFor(() => screen.getByTestId('reading-order'))
  return api
}

describe('ReviewWorkspace companions', () => {
  it('mounts a collapsed bottom dock whose panels only load once opened', async () => {
    const render1 = vi.fn(() => <p data-testid="stub-findings">stub</p>)
    registerReviewDockPanel({
      id: 'findings',
      order: 10,
      labelKey: 'auto.components.reviewMap.shell.dock.findings',
      labelFallback: 'Findings',
      render: render1
    })
    await mount()
    expect(screen.getByTestId('review-dock').getAttribute('data-open')).toBe('false')
    expect(render1).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Findings' }))
    expect(await screen.findByTestId('stub-findings')).toBeTruthy()
    expect(screen.getByTestId('review-dock').getAttribute('data-open')).toBe('true')
    fireEvent.click(screen.getByRole('button', { name: 'Collapse panel' }))
    expect(screen.queryByTestId('stub-findings')).toBeNull()
  })

  it('shows a quality-only dock panel only when the quality flag is on', async () => {
    registerReviewDockPanel({
      id: 'quality-findings',
      order: 30,
      labelKey: 'x.quality',
      labelFallback: 'Quality findings',
      requiresQuality: true,
      render: () => null
    })
    const { unmount } = render(
      <ReviewWorkspace worktreeId={WT} api={makeApi() as unknown as ReviewDataApi} />
    )
    await waitFor(() => screen.getByTestId('reading-order'))
    expect(screen.queryByRole('button', { name: 'Quality findings' })).toBeNull()
    unmount()
    render(
      <ReviewWorkspace
        worktreeId={WT}
        api={makeApi() as unknown as ReviewDataApi}
        flags={{ quality: true }}
      />
    )
    expect(await screen.findByRole('button', { name: 'Quality findings' })).toBeTruthy()
  })

  it('shows the turn switcher from stored markers and narrows the reading order since the previous turn', async () => {
    await mount(makeApi([marker('t1', 1, 'h1'), marker('t2', 2, 'h2')]))
    const group = await screen.findByRole('group', { name: 'Turn comparison' })
    expect(group).toBeTruthy()
    const before = screen.getByTestId('reading-order').textContent
    fireEvent.click(screen.getByRole('radio', { name: 'Since previous turn' }))
    // The stored turns only touch src/a.ts, which the fixture overlay does not contain.
    await waitFor(() => expect(screen.getByTestId('reading-order').textContent).not.toBe(before))
  })

  it('shows markers the App-level recorder saves while the workspace is mounted', async () => {
    await mount()
    expect(screen.queryByRole('group', { name: 'Turn comparison' })).toBeNull()
    act(() => publishTurnSaved(WT, [marker('t1', 1, 'h1') as never, marker('t2', 2, 'h2') as never]))
    expect(await screen.findByRole('group', { name: 'Turn comparison' })).toBeTruthy()
  })

  it('reports review_opened once with the source an entry point set', async () => {
    setReviewOpenSource(WT, { source: 'cmd_k', afterAgentTurn: true })
    await mount()
    await waitFor(() => expect(h.opened).toHaveBeenCalledTimes(1))
    expect(h.opened).toHaveBeenCalledWith(
      expect.objectContaining({ source: 'cmd_k', after_agent_turn: true, scope: 'merge_base', index: 'fresh' })
    )
  })

  it('records mark_reviewed once when every reading step is seen', async () => {
    await mount()
    expect(h.decide).not.toHaveBeenCalled()
    act(() =>
      useAppStore
        .getState()
        .setReadingGroupSeen(WT, makeOverlay().readingOrder.map((s) => s.stepKey), true)
    )
    await waitFor(() => expect(h.decide).toHaveBeenCalledTimes(1))
    expect(h.decide).toHaveBeenCalledWith(WT, 'mark_reviewed', expect.anything())
  })

  it('hides the Storage tab once its lens reports the backend as unsupported', async () => {
    await mount()
    expect(document.querySelector('[data-lens="storage"]')).toBeTruthy()
    act(() => setReviewLensUnavailable(WT, 'storage', true))
    expect(document.querySelector('[data-lens="storage"]')).toBeNull()
    expect(document.querySelector('[data-lens="erd"]')).toBeTruthy()
  })

  it('keeps the report menu and AI summary card out of the way while their flags are off', async () => {
    await mount()
    expect(screen.queryByRole('button', { name: 'Export report' })).toBeNull()
    expect(screen.queryByText('AI-inferred summary')).toBeNull()
  })

  it('shows the report menu with the quality flag and the AI card only with the AI flag', async () => {
    const support = (ai: boolean) =>
      ({
        state: 'enabled',
        effective: { codeIntelEnabled: true, qualityGateEnabled: true, aiReviewEnabled: ai }
      }) as never
    useAppStore.setState({ codeIntelSupportState: support(false) })
    const { unmount } = render(
      <ReviewWorkspace worktreeId={WT} api={makeApi() as unknown as ReviewDataApi} flags={{ quality: true }} />
    )
    expect(await screen.findByRole('button', { name: 'Export report' })).toBeTruthy()
    expect(screen.queryByText('AI-inferred summary')).toBeNull()
    unmount()
    useAppStore.setState({ codeIntelSupportState: support(true) })
    render(
      <ReviewWorkspace worktreeId={WT} api={makeApi() as unknown as ReviewDataApi} flags={{ quality: true }} />
    )
    expect(await screen.findByText('AI-inferred summary')).toBeTruthy()
  })
})
