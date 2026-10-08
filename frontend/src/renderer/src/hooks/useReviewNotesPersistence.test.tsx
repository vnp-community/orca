// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'
import type { ReviewSentBatch, ReviewTurnMarker } from '../../../shared/code-intel-types'

const holder = vi.hoisted(() => ({ store: null as unknown }))
vi.mock('@/store', () => ({
  useAppStore: (selector: (s: unknown) => unknown) =>
    (holder.store as { (sel: (s: unknown) => unknown): unknown })(selector)
}))

import { create } from 'zustand'
import {
  createReviewProgressSlice,
  REVIEW_PROGRESS_DEBOUNCE_MS,
  setReviewProgressApi,
  type ReviewProgressSlice
} from '../store/slices/review-progress'
import { emptyReadingProgress } from '../components/review-map/review-wire-types'
import type { ReviewStateView } from '../components/review-map/review-wire-types'
import type { ReviewDataApi } from '../components/review-map/review-shell-data'
import { pendingTurnMarkerQueues } from '../components/review-map/turns/review-turn-marker-row'
import { useReviewNotesPersistence } from './useReviewNotesPersistence'

const WT = 'repo::wt'
type Result<T> = { ok: true; value: T } | { ok: false; error: { kind: string; message: string } }

const batch = (id: string, sentAt: number): ReviewSentBatch => ({
  batchId: id,
  sentAt,
  turnId: null,
  targetPaneKey: null,
  agentType: null,
  notes: [
    { commentId: `${id}-c`, anchor: { kind: 'diff-line', filePath: 'a', lineNumber: 1 }, filePath: 'a', lineNumber: 1, body: 'b' }
  ]
})
const marker = (n: number): ReviewTurnMarker => ({
  turnId: `p:${n}`,
  worktreeId: WT,
  paneKey: 'p',
  agentType: null,
  startedAt: null,
  endedAt: n,
  baseOid: null,
  headOid: null,
  files: [],
  overlayAvailable: false
})

function makeApi(rows: Record<string, ReviewStateView>, script: { save?: (state: ReviewStateView, expected: number) => Result<ReviewStateView> | undefined } = {}) {
  const key = (k: { baseCommit: string; headCommit: string }) => `${k.baseCommit}|${k.headCommit}`
  const saves: { state: ReviewStateView; expected: number }[] = []
  const api: ReviewDataApi = {
    getStatus: vi.fn(),
    reindex: vi.fn(),
    bindRepo: vi.fn(),
    getChangeOverlay: vi.fn(),
    getReviewState: vi.fn(async (_w, k) => ({
      ok: true as const,
      value: rows[key(k)] ?? { baseCommit: k.baseCommit, headCommit: k.headCommit, readingProgress: emptyReadingProgress(), version: 0 }
    })),
    saveReviewState: vi.fn(async (_w, state, expected) => {
      saves.push({ state, expected })
      const scripted = script.save?.(state, expected)
      if (scripted) {
        return scripted
      }
      const stored = { ...state, version: expected + 1 }
      rows[key(state)] = stored
      return { ok: true as const, value: stored }
    })
  } as unknown as ReviewDataApi
  return { api, saves, rows }
}

async function setup(api: ReviewDataApi) {
  setReviewProgressApi(api)
  const store = create<ReviewProgressSlice>()(
    (...a) =>
      createReviewProgressSlice(...(a as unknown as Parameters<typeof createReviewProgressSlice>)) as ReviewProgressSlice
  )
  holder.store = store
  await store.getState().loadReviewProgress(WT, 'b1', 'h1')
  return store
}

// Stepped so React effects run between timer ticks, like real time.
async function settle(ms = REVIEW_PROGRESS_DEBOUNCE_MS + 50): Promise<void> {
  for (let elapsed = 0; elapsed < ms; elapsed += 200) {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(200)
    })
  }
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  cleanup()
  vi.useRealTimers()
  setReviewProgressApi(null)
})

describe('useReviewNotesPersistence: notes row', () => {
  it('refuses updates until the row is loaded', () => {
    holder.store = create<ReviewProgressSlice>()(
      (...a) => createReviewProgressSlice(...(a as unknown as Parameters<typeof createReviewProgressSlice>)) as ReviewProgressSlice
    )
    const { result } = renderHook(() => useReviewNotesPersistence(WT, { api: makeApi({}).api }))
    expect(result.current.ready).toBe(false)
    expect(result.current.updateNotes((p) => p)).toBe(false)
  })

  it('saves anchors and batches with the version that was read', async () => {
    const { api, saves } = makeApi({ 'b1|h1': { baseCommit: 'b1', headCommit: 'h1', readingProgress: emptyReadingProgress(), version: 4 } })
    await setup(api)
    const { result } = renderHook(() => useReviewNotesPersistence(WT, { api }))
    expect(result.current.ready).toBe(true)
    act(() => {
      result.current.updateNotes((p) => ({
        anchors: { ...p.anchors, c1: { kind: 'diff-line', filePath: 'a', lineNumber: 2 } },
        sentBatches: [...p.sentBatches, batch('b1', 10)]
      }))
    })
    await settle()
    expect(saves).toHaveLength(1)
    expect(saves[0].expected).toBe(4)
    expect(saves[0].state.notes).toMatchObject({ anchors: { c1: { lineNumber: 2 } }, sentBatches: [{ batchId: 'b1' }] })
    expect(result.current.saveStatus).toBe('saved')
  })

  it('merges our pending notes back after a conflict replaced the server copy', async () => {
    let conflicted = false
    const remote: ReviewStateView = {
      baseCommit: 'b1',
      headCommit: 'h1',
      readingProgress: emptyReadingProgress(),
      version: 7,
      notes: { anchors: {}, sentBatches: [batch('remote', 5)] }
    }
    const { api, saves } = makeApi(
      { 'b1|h1': { baseCommit: 'b1', headCommit: 'h1', readingProgress: emptyReadingProgress(), version: 1 } },
      {
        save: () => {
          if (!conflicted) {
            conflicted = true
            return { ok: false, error: { kind: 'conflict', message: 'CODEINTEL_VERSION_CONFLICT' } }
          }
          return undefined
        }
      }
    )
    ;(api.getReviewState as ReturnType<typeof vi.fn>).mockImplementation(async (_w: string, k: { baseCommit: string }) => ({
      ok: true,
      value: conflicted && k.baseCommit === 'b1' ? remote : { baseCommit: k.baseCommit, headCommit: 'h1', readingProgress: emptyReadingProgress(), version: 1 }
    }))
    await setup(api)
    const { result } = renderHook(() => useReviewNotesPersistence(WT, { api }))
    act(() => {
      result.current.updateNotes((p) => ({ ...p, sentBatches: [batch('local', 10)] }))
    })
    await settle(REVIEW_PROGRESS_DEBOUNCE_MS * 4)
    const last = saves.at(-1)!
    const ids = (last.state.notes as { sentBatches: ReviewSentBatch[] }).sentBatches.map((b) => b.batchId)
    expect(ids).toEqual(['remote', 'local'])
  })

  it('retries once with a smaller budget on too-large', async () => {
    let first = true
    const { api, saves } = makeApi(
      { 'b1|h1': { baseCommit: 'b1', headCommit: 'h1', readingProgress: emptyReadingProgress(), version: 0 } },
      {
        save: () => {
          if (first) {
            first = false
            return { ok: false, error: { kind: 'too-large', message: 'CODEINTEL_PAYLOAD_TOO_LARGE' } }
          }
          return undefined
        }
      }
    )
    await setup(api)
    const { result } = renderHook(() => useReviewNotesPersistence(WT, { api }))
    const many = Array.from({ length: 400 }, (_v, i) => batch(`b${i}`, i))
    act(() => {
      result.current.updateNotes((p) => ({ ...p, sentBatches: many }))
    })
    await settle(REVIEW_PROGRESS_DEBOUNCE_MS * 3)
    const lastNotes = saves.at(-1)!.state.notes as { sentBatches: ReviewSentBatch[] }
    expect(saves.length).toBeGreaterThanOrEqual(2)
    expect(lastNotes.sentBatches.length).toBeLessThanOrEqual(250)
  })

  it('does nothing when disabled', async () => {
    const { api } = makeApi({})
    await setup(api)
    renderHook(() => useReviewNotesPersistence(WT, { api, enabled: false }))
    await settle()
    expect(api.saveReviewState).not.toHaveBeenCalled()
    expect((api.getReviewState as ReturnType<typeof vi.fn>).mock.calls.filter((c) => c[1].baseCommit === '')).toHaveLength(0)
  })
})

describe('useReviewNotesPersistence: turn markers row', () => {
  it('saves markers only on the worktree-level row ("", "")', async () => {
    const { api, saves } = makeApi({})
    await setup(api)
    const { result } = renderHook(() => useReviewNotesPersistence(WT, { api }))
    let ok = false
    await act(async () => {
      ok = await result.current.saveTurnMarkers([marker(1)])
    })
    expect(ok).toBe(true)
    expect(saves).toHaveLength(1)
    expect(saves[0].state.baseCommit).toBe('')
    expect(saves[0].state.headCommit).toBe('')
    expect(result.current.turnMarkers.map((m) => m.turnId)).toEqual(['p:1'])
    expect(pendingTurnMarkerQueues()).toBe(0)
  })

  it('merges with markers already stored, caps at five and serializes concurrent saves', async () => {
    const stored: ReviewTurnMarker[] = [1, 2, 3, 4, 5].map(marker)
    const { api, saves } = makeApi({
      '|': { baseCommit: '', headCommit: '', readingProgress: emptyReadingProgress(), version: 2, turnMarkers: stored }
    })
    await setup(api)
    const { result } = renderHook(() => useReviewNotesPersistence(WT, { api }))
    await act(async () => {
      await Promise.all([result.current.saveTurnMarkers([marker(6)]), result.current.saveTurnMarkers([marker(7)])])
    })
    const last = saves.at(-1)!.state.turnMarkers as ReviewTurnMarker[]
    expect(last.map((m) => m.endedAt)).toEqual([3, 4, 5, 6, 7])
    expect(saves.map((s) => s.expected)).toEqual([2, 3])
  })

  it('retries once on a version conflict and then reports error status', async () => {
    const { api } = makeApi({}, { save: () => ({ ok: false, error: { kind: 'conflict', message: '' } }) })
    await setup(api)
    const { result } = renderHook(() => useReviewNotesPersistence(WT, { api }))
    let ok = true
    await act(async () => {
      ok = await result.current.saveTurnMarkers([marker(1)])
    })
    expect(ok).toBe(false)
    expect(result.current.markersStatus).toBe('error')
    expect((api.saveReviewState as ReturnType<typeof vi.fn>).mock.calls).toHaveLength(2)
  })
})
