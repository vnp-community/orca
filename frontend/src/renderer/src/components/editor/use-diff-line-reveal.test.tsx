// @vitest-environment happy-dom
import { renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useDiffLineReveal } from './use-diff-line-reveal'
import { isDiffCursorSuppressed, resetDiffCursorLineBusForTests } from '@/lib/diff-cursor-line-bus'

function fakeEditor(lines = 100) {
  return {
    getModel: () => ({ getLineCount: () => lines }),
    revealLineInCenter: vi.fn(),
    setPosition: vi.fn()
  }
}

beforeEach(() => {
  vi.useFakeTimers()
  resetDiffCursorLineBusForTests()
})
afterEach(() => vi.useRealTimers())

describe('useDiffLineReveal', () => {
  it('waits until Monaco is mounted, then reveals once and reports applied', () => {
    const ed = fakeEditor()
    const onApplied = vi.fn()
    const { rerender } = renderHook(
      (p: { mounted: unknown }) =>
        useDiffLineReveal({
          mountedSignal: p.mounted,
          resolveEditor: () => ed as never,
          line: 42,
          nonce: 1,
          onApplied
        }),
      { initialProps: { mounted: null as unknown } }
    )
    vi.advanceTimersByTime(50)
    expect(ed.revealLineInCenter).not.toHaveBeenCalled()
    rerender({ mounted: {} })
    vi.advanceTimersByTime(50)
    expect(ed.revealLineInCenter).toHaveBeenCalledWith(42)
    expect(ed.setPosition).toHaveBeenCalledWith({ lineNumber: 42, column: 1 })
    expect(onApplied).toHaveBeenCalledWith(1)
    expect(isDiffCursorSuppressed()).toBe(true)
    rerender({ mounted: {} })
    vi.advanceTimersByTime(50)
    expect(ed.revealLineInCenter).toHaveBeenCalledTimes(1)
  })

  it('re-applies only for a new nonce and clamps the line', () => {
    const ed = fakeEditor(10)
    const { rerender } = renderHook(
      (p: { nonce: number }) =>
        useDiffLineReveal({
          mountedSignal: true,
          resolveEditor: () => ed as never,
          line: 999,
          nonce: p.nonce
        }),
      { initialProps: { nonce: 1 } }
    )
    vi.advanceTimersByTime(50)
    expect(ed.revealLineInCenter).toHaveBeenLastCalledWith(10)
    rerender({ nonce: 2 })
    vi.advanceTimersByTime(50)
    expect(ed.revealLineInCenter).toHaveBeenCalledTimes(2)
  })

  it('does nothing without a request', () => {
    const ed = fakeEditor()
    renderHook(() => useDiffLineReveal({ mountedSignal: true, resolveEditor: () => ed as never }))
    vi.advanceTimersByTime(50)
    expect(ed.revealLineInCenter).not.toHaveBeenCalled()
  })
})
