import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { create } from 'zustand'
import type { ReviewDataApi } from '../../components/review-map/review-shell-data'
import {
  emptyReadingProgress,
  type ReviewStateView
} from '../../components/review-map/review-wire-types'
import {
  createReviewProgressSlice,
  REVIEW_PROGRESS_DEBOUNCE_MS,
  REVIEW_PROGRESS_RETRY_MS,
  setReviewProgressApi,
  type ReviewProgressSlice
} from './review-progress'

const WT = 'repo::wt'
const server = (over: Partial<ReviewStateView> = {}): ReviewStateView => ({
  baseCommit: 'b1',
  headCommit: 'h1',
  readingProgress: emptyReadingProgress(),
  version: 0,
  ...over
})

function setup(over: Partial<ReviewDataApi> = {}) {
  type SaveResult = Awaited<ReturnType<ReviewDataApi['saveReviewState']>>
  const save = vi.fn(
    async (_wt: string, state: ReviewStateView, expected: number): Promise<SaveResult> => ({
      ok: true,
      value: { ...state, version: expected + 1 }
    })
  )
  type GetResult = Awaited<ReturnType<ReviewDataApi['getReviewState']>>
  const get = vi.fn(async (): Promise<GetResult> => ({ ok: true, value: server() }))
  const api = { saveReviewState: save, getReviewState: get, ...over } as unknown as ReviewDataApi
  setReviewProgressApi(api)
  const store = create<ReviewProgressSlice>()(
    (...a) =>
      createReviewProgressSlice(
        ...(a as unknown as Parameters<typeof createReviewProgressSlice>)
      ) as ReviewProgressSlice
  )
  return { store, save, get }
}
const entry = (s: ReturnType<typeof setup>['store']) => s.getState().reviewProgressByWorktree[WT]

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  vi.useRealTimers()
  setReviewProgressApi(null)
})

describe('review progress slice', () => {
  it('load: version 0 default row is ready with empty progress', async () => {
    const { store } = setup()
    await store.getState().loadReviewProgress(WT, 'b1', 'h1')
    expect(entry(store)).toMatchObject({ loadStatus: 'ready', saveStatus: 'saved' })
  })

  it('load error keeps list usable and records the kind', async () => {
    const { store } = setup({
      getReviewState: vi.fn(async () => ({
        ok: false as const,
        error: { kind: 'offline' as const, message: 'x' }
      }))
    })
    await store.getState().loadReviewProgress(WT, 'b1', 'h1')
    expect(entry(store)).toMatchObject({ loadStatus: 'error', loadErrorKind: 'offline' })
  })

  it('debounces: many toggles within 800ms produce one save with expectedVersion', async () => {
    const { store, save } = setup()
    await store.getState().loadReviewProgress(WT, 'b1', 'h1')
    store.getState().setReadingItemSeen(WT, 'a', true)
    store.getState().setReadingItemSeen(WT, 'b', true)
    expect(entry(store).saveStatus).toBe('dirty')
    expect(save).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(REVIEW_PROGRESS_DEBOUNCE_MS)
    expect(save).toHaveBeenCalledTimes(1)
    expect(save.mock.calls[0][2]).toBe(0)
    expect(Object.keys(save.mock.calls[0][1].readingProgress.entries)).toEqual(['a', 'b'])
    expect(entry(store).saveStatus).toBe('saved')
    expect(entry(store).serverState?.version).toBe(1)
  })

  it('never reports saved before the save resolves', async () => {
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    const { store, save } = setup()
    save.mockImplementation(async (_w, state, expected) => {
      await gate
      return { ok: true as const, value: { ...state, version: expected + 1 } }
    })
    await store.getState().loadReviewProgress(WT, 'b1', 'h1')
    store.getState().setReadingItemSeen(WT, 'a', true)
    await vi.advanceTimersByTimeAsync(REVIEW_PROGRESS_DEBOUNCE_MS)
    expect(entry(store).saveStatus).toBe('saving')
    release()
    await vi.advanceTimersByTimeAsync(0)
    expect(entry(store).saveStatus).toBe('saved')
  })

  it('edits made during a save are sent by a follow-up save (max one in flight)', async () => {
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    const { store, save } = setup()
    let first = true
    save.mockImplementation(async (_w, state, expected) => {
      if (first) {
        first = false
        await gate
      }
      return { ok: true as const, value: { ...state, version: expected + 1 } }
    })
    await store.getState().loadReviewProgress(WT, 'b1', 'h1')
    store.getState().setReadingItemSeen(WT, 'a', true)
    await vi.advanceTimersByTimeAsync(REVIEW_PROGRESS_DEBOUNCE_MS)
    store.getState().setReadingItemSeen(WT, 'b', true)
    await vi.advanceTimersByTimeAsync(REVIEW_PROGRESS_DEBOUNCE_MS)
    expect(save).toHaveBeenCalledTimes(1)
    release()
    await vi.advanceTimersByTimeAsync(0)
    expect(save).toHaveBeenCalledTimes(2)
    expect(Object.keys(save.mock.calls[1][1].readingProgress.entries)).toEqual(['a', 'b'])
    expect(entry(store).saveStatus).toBe('saved')
  })

  it('flush saves immediately without waiting for the debounce', async () => {
    const { store, save } = setup()
    await store.getState().loadReviewProgress(WT, 'b1', 'h1')
    store.getState().setReadingItemSeen(WT, 'a', true)
    await store.getState().flushReviewProgress(WT)
    expect(save).toHaveBeenCalledTimes(1)
  })

  it('version conflict: reload, merge by at, save again', async () => {
    const remote = server({
      version: 5,
      readingProgress: {
        version: 1,
        lastFocusedKey: null,
        entries: { r: { state: 'seen', at: 1 } }
      }
    })
    const { store, save, get } = setup()
    get.mockResolvedValue({ ok: true as const, value: remote })
    save.mockImplementationOnce(async () => ({
      ok: false as const,
      error: { kind: 'conflict' as const, message: 'c' }
    }))
    await store.getState().loadReviewProgress(WT, 'b1', 'h1')
    get.mockClear()
    store.getState().setReadingItemSeen(WT, 'a', true)
    await store.getState().flushReviewProgress(WT)
    expect(get).toHaveBeenCalledTimes(1)
    expect(save).toHaveBeenCalledTimes(2)
    expect(save.mock.calls[1][2]).toBe(5)
    expect(Object.keys(save.mock.calls[1][1].readingProgress.entries).sort()).toEqual(['a', 'r'])
    expect(entry(store).saveStatus).toBe('saved')
  })

  it('offline: stays error (not saved), retries after 15s', async () => {
    const { store, save } = setup()
    save.mockImplementationOnce(async () => ({
      ok: false as const,
      error: { kind: 'offline' as const, message: 'o' }
    }))
    await store.getState().loadReviewProgress(WT, 'b1', 'h1')
    store.getState().setReadingItemSeen(WT, 'a', true)
    await store.getState().flushReviewProgress(WT)
    expect(entry(store)).toMatchObject({
      saveStatus: 'error',
      lastErrorKind: 'offline',
      localOnly: false
    })
    await vi.advanceTimersByTimeAsync(REVIEW_PROGRESS_RETRY_MS)
    expect(save).toHaveBeenCalledTimes(2)
    expect(entry(store).saveStatus).toBe('saved')
  })

  it('forbidden: marks stay local, no retry, marking still works', async () => {
    const { store, save } = setup()
    save.mockImplementation(async () => ({
      ok: false as const,
      error: { kind: 'forbidden' as const, message: 'f' }
    }))
    await store.getState().loadReviewProgress(WT, 'b1', 'h1')
    store.getState().setReadingItemSeen(WT, 'a', true)
    await store.getState().flushReviewProgress(WT)
    expect(entry(store)).toMatchObject({ saveStatus: 'error', localOnly: true })
    store.getState().setReadingItemSeen(WT, 'b', true)
    await vi.advanceTimersByTimeAsync(REVIEW_PROGRESS_RETRY_MS * 2)
    expect(save).toHaveBeenCalledTimes(1)
    expect(Object.keys(entry(store).progress.entries)).toEqual(['a', 'b'])
  })

  it('prunes unseen tombstones before sending an oversized payload', async () => {
    const { store, save } = setup()
    await store.getState().loadReviewProgress(WT, 'b1', 'h1')
    const keys = Array.from({ length: 700 }, (_, i) => `step-${'k'.repeat(80)}-${i}`)
    store.getState().setReadingGroupSeen(WT, keys, false)
    await store.getState().flushReviewProgress(WT)
    const sent = save.mock.calls[0][1].readingProgress
    expect(Object.keys(sent.entries).length).toBeLessThan(700)
    expect(entry(store).oversize).toBe(false)
  })

  it('a new head starts empty and is not seeded from the old row', async () => {
    const { store, get } = setup()
    await store.getState().loadReviewProgress(WT, 'b1', 'h1')
    store.getState().setReadingItemSeen(WT, 'a', true)
    await store.getState().flushReviewProgress(WT)
    get.mockResolvedValueOnce({ ok: true as const, value: server({ headCommit: 'h2' }) })
    await store.getState().loadReviewProgress(WT, 'b1', 'h2')
    expect(entry(store).progress.entries).toEqual({})
    expect(entry(store).headCommit).toBe('h2')
  })

  it('patchReviewState rides the same queue and keeps notes verbatim', async () => {
    const { store, save } = setup()
    await store.getState().loadReviewProgress(WT, 'b1', 'h1')
    store.getState().patchReviewState(WT, { notes: { anchors: { n1: {} } } })
    await store.getState().flushReviewProgress(WT)
    expect(save.mock.calls[0][1].notes).toEqual({ anchors: { n1: {} } })
  })

  it('dropReviewProgress clears pending timers and state', async () => {
    const { store, save } = setup()
    await store.getState().loadReviewProgress(WT, 'b1', 'h1')
    store.getState().setReadingItemSeen(WT, 'a', true)
    store.getState().dropReviewProgress(WT)
    await vi.advanceTimersByTimeAsync(REVIEW_PROGRESS_DEBOUNCE_MS * 2)
    expect(save).not.toHaveBeenCalled()
    expect(entry(store)).toBeUndefined()
  })
})
