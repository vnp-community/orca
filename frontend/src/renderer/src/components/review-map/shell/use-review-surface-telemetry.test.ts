// @vitest-environment happy-dom
import { renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const h = vi.hoisted(() => ({ opened: vi.fn(), lens: vi.fn() }))
vi.mock('@/lib/review-telemetry', () => ({ trackReviewOpened: h.opened, trackReviewLensViewed: h.lens }))

import {
  telemetryIndexState,
  telemetryScopeKind,
  useReviewSurfaceTelemetry
} from './use-review-surface-telemetry'

const scope = { kind: 'branch', baseRef: 'origin/main', includeUncommitted: true } as const
const args = (over: Record<string, unknown> = {}) => ({
  worktreeId: 'wt',
  ready: true,
  scope,
  defaultBaseRef: 'origin/main',
  overall: 'READY' as const,
  activeLensId: 'impact',
  ...over
})

beforeEach(() => {
  vi.useFakeTimers()
  h.opened.mockClear()
  h.lens.mockClear()
})
afterEach(() => vi.useRealTimers())

describe('telemetry mapping', () => {
  it('maps scope and index state to the closed enums', () => {
    expect(telemetryScopeKind(scope, 'origin/main')).toBe('merge_base')
    expect(telemetryScopeKind({ ...scope, baseRef: 'release' }, 'origin/main')).toBe('custom_base')
    expect(telemetryScopeKind({ kind: 'range', baseCommit: 'a', headCommit: 'b' }, null)).toBe('committed')
    expect(telemetryIndexState('OVERLAY')).toBe('stale')
    expect(telemetryIndexState('NOT_INSTALLED')).toBe('missing')
    expect(telemetryIndexState(undefined)).toBe('unknown')
  })
})

describe('useReviewSurfaceTelemetry', () => {
  it('sends review_opened once data is ready, not before and not twice', () => {
    const { rerender } = renderHook((p) => useReviewSurfaceTelemetry(p), {
      initialProps: args({ ready: false })
    })
    expect(h.opened).not.toHaveBeenCalled()
    rerender(args())
    rerender(args({ activeLensId: 'erd' }))
    expect(h.opened).toHaveBeenCalledTimes(1)
    expect(h.opened.mock.calls[0][0]).toMatchObject({ lens: 'impact', index: 'fresh', source: 'restore' })
  })

  it('sends review_lens_viewed only for a lens kept in view for 2 s, once per lens', () => {
    const { rerender } = renderHook((p) => useReviewSurfaceTelemetry(p), { initialProps: args() })
    vi.advanceTimersByTime(500)
    rerender(args({ activeLensId: 'erd' }))
    expect(h.lens).not.toHaveBeenCalled()
    vi.advanceTimersByTime(3000)
    rerender(args({ activeLensId: 'impact' }))
    expect(h.lens).toHaveBeenCalledTimes(1)
    expect(h.lens).toHaveBeenCalledWith({ lens: 'erd', dwellMs: 3000 })
    vi.advanceTimersByTime(3000)
    rerender(args({ activeLensId: 'erd' }))
    rerender(args({ activeLensId: 'impact' }))
    // impact was viewed 3 s now, but the first 0.5 s visit was too short to count.
    expect(h.lens).toHaveBeenCalledTimes(2)
  })
})
