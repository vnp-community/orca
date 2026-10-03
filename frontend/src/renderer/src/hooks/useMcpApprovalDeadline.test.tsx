// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'
import { useMcpApprovalDeadline } from './useMcpApprovalDeadline'

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(1_000_000)
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('useMcpApprovalDeadline', () => {
  it('counts down at 1 Hz and clamps at zero', () => {
    const { result } = renderHook(() => useMcpApprovalDeadline(1_003_000))
    expect(result.current).toBe(3000)
    act(() => void vi.advanceTimersByTime(1000))
    expect(result.current).toBe(2000)
    act(() => void vi.advanceTimersByTime(5000))
    expect(result.current).toBe(0)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('returns 0 and sets no timer without a deadline', () => {
    const { result } = renderHook(() => useMcpApprovalDeadline(undefined))
    expect(result.current).toBe(0)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('re-baselines immediately when the deadline prop changes', () => {
    const { result, rerender } = renderHook(({ d }) => useMcpApprovalDeadline(d), {
      initialProps: { d: 1_002_000 as number | undefined }
    })
    act(() => void vi.advanceTimersByTime(1500))
    vi.setSystemTime(1_001_500 + 400)
    rerender({ d: 1_010_000 })
    expect(result.current).toBe(1_010_000 - 1_001_900)
    act(() => void vi.advanceTimersByTime(1000))
    expect(result.current).toBe(1_010_000 - 1_002_900)
    rerender({ d: undefined })
    expect(result.current).toBe(0)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('clears the interval on unmount', () => {
    const { unmount } = renderHook(() => useMcpApprovalDeadline(1_009_000))
    expect(vi.getTimerCount()).toBe(1)
    unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})
