// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { perceivedLoadingStageAt, usePerceivedLoadingStage } from './usePerceivedLoadingStage'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('perceivedLoadingStageAt', () => {
  it('walks the ladder for local targets', () => {
    expect(
      [0, 99, 100, 999, 1000, 2999, 3000].map((ms) => perceivedLoadingStageAt(ms, false))
    ).toEqual(['busy', 'busy', 'dimmed', 'dimmed', 'spinner', 'spinner', 'stages'])
  })
  it('remote targets wait 200ms before dimming', () => {
    expect(perceivedLoadingStageAt(150, true)).toBe('busy')
    expect(perceivedLoadingStageAt(200, true)).toBe('dimmed')
  })
})

describe('usePerceivedLoadingStage', () => {
  it('is idle when nothing is pending', () => {
    expect(renderHook(() => usePerceivedLoadingStage(false)).result.current).toBe('idle')
  })
  it('goes busy -> dimmed -> spinner -> stages with timers', () => {
    const { result } = renderHook(() => usePerceivedLoadingStage(true))
    expect(result.current).toBe('busy')
    act(() => void vi.advanceTimersByTime(100))
    expect(result.current).toBe('dimmed')
    act(() => void vi.advanceTimersByTime(900))
    expect(result.current).toBe('spinner')
    act(() => void vi.advanceTimersByTime(2000))
    expect(result.current).toBe('stages')
  })
  it('remote waits 200ms to dim', () => {
    const { result } = renderHook(() => usePerceivedLoadingStage(true, { remote: true }))
    act(() => void vi.advanceTimersByTime(150))
    expect(result.current).toBe('busy')
    act(() => void vi.advanceTimersByTime(50))
    expect(result.current).toBe('dimmed')
  })
  it('finishing before a threshold cancels the later stages', () => {
    const { result, rerender } = renderHook(({ p }) => usePerceivedLoadingStage(p), {
      initialProps: { p: true }
    })
    act(() => void vi.advanceTimersByTime(50))
    rerender({ p: false })
    expect(result.current).toBe('idle')
    act(() => void vi.advanceTimersByTime(5000))
    expect(result.current).toBe('idle')
  })
})
