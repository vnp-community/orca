// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { RETURN_REFRESH_MIN_GAP_MS, useRequestRefreshOnReturn } from './useRequestRefreshOnReturn'

function setVisibility(state: 'visible' | 'hidden'): void {
  Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state })
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-10-20T10:00:00Z'))
  setVisibility('visible')
})
afterEach(() => {
  vi.useRealTimers()
})

describe('useRequestRefreshOnReturn', () => {
  it('refetches when the window gets focus', () => {
    const refetch = vi.fn()
    renderHook(() => useRequestRefreshOnReturn(refetch))
    act(() => {
      window.dispatchEvent(new Event('focus'))
    })
    expect(refetch).toHaveBeenCalledTimes(1)
  })

  it('refetches when the tab becomes visible, not when it is hidden', () => {
    const refetch = vi.fn()
    renderHook(() => useRequestRefreshOnReturn(refetch))
    setVisibility('hidden')
    act(() => {
      document.dispatchEvent(new Event('visibilitychange'))
    })
    expect(refetch).not.toHaveBeenCalled()
    setVisibility('visible')
    act(() => {
      document.dispatchEvent(new Event('visibilitychange'))
    })
    expect(refetch).toHaveBeenCalledTimes(1)
  })

  it('does not refetch twice for the focus and visibility events of one return', () => {
    const refetch = vi.fn()
    renderHook(() => useRequestRefreshOnReturn(refetch))
    act(() => {
      window.dispatchEvent(new Event('focus'))
      document.dispatchEvent(new Event('visibilitychange'))
    })
    expect(refetch).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(RETURN_REFRESH_MIN_GAP_MS + 1)
    act(() => {
      window.dispatchEvent(new Event('focus'))
    })
    expect(refetch).toHaveBeenCalledTimes(2)
  })

  it('calls the latest refetch and stops listening after unmount', () => {
    const first = vi.fn()
    const second = vi.fn()
    const { rerender, unmount } = renderHook(({ fn }) => useRequestRefreshOnReturn(fn), {
      initialProps: { fn: first }
    })
    rerender({ fn: second })
    act(() => {
      window.dispatchEvent(new Event('focus'))
    })
    expect(first).not.toHaveBeenCalled()
    expect(second).toHaveBeenCalledTimes(1)
    unmount()
    vi.advanceTimersByTime(RETURN_REFRESH_MIN_GAP_MS + 1)
    act(() => {
      window.dispatchEvent(new Event('focus'))
    })
    expect(second).toHaveBeenCalledTimes(1)
  })
})
