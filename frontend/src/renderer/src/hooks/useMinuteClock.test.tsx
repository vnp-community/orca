// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useMinuteClock } from './useMinuteClock'

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-10-07T12:00:00Z'))
})
afterEach(() => vi.useRealTimers())

describe('useMinuteClock', () => {
  it('ticks once per minute', () => {
    const { result } = renderHook(() => useMinuteClock())
    const first = result.current
    act(() => { vi.advanceTimersByTime(59_000) })
    expect(result.current).toBe(first)
    act(() => { vi.advanceTimersByTime(1_000) })
    expect(result.current).toBe(first + 60_000)
  })

  it('clears its interval on unmount and when disabled', () => {
    const { unmount } = renderHook(() => useMinuteClock())
    expect(vi.getTimerCount()).toBe(1)
    unmount()
    expect(vi.getTimerCount()).toBe(0)
    const off = renderHook(() => useMinuteClock(false))
    expect(vi.getTimerCount()).toBe(0)
    off.unmount()
  })
})
