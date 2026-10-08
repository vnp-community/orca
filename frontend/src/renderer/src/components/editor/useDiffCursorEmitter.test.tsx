// @vitest-environment happy-dom
import { renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useDiffCursorEmitter } from './useDiffCursorEmitter'
import {
  resetDiffCursorLineBusForTests,
  subscribeDiffCursorLine
} from '@/lib/diff-cursor-line-bus'

function fakeEditor() {
  let cb: ((e: { position: { lineNumber: number } }) => void) | null = null
  const dispose = vi.fn()
  return {
    onDidChangeCursorPosition: vi.fn((f) => {
      cb = f
      return { dispose }
    }),
    fire: (line: number) => cb?.({ position: { lineNumber: line } }),
    dispose
  }
}
beforeEach(() => {
  vi.useFakeTimers()
  resetDiffCursorLineBusForTests()
})
afterEach(() => vi.useRealTimers())

describe('useDiffCursorEmitter', () => {
  it('attaches nothing without listeners', () => {
    const ed = fakeEditor()
    renderHook(() => useDiffCursorEmitter(ed as never, 'wt', 'a.ts'))
    expect(ed.onDidChangeCursorPosition).not.toHaveBeenCalled()
    expect(vi.getTimerCount()).toBe(0)
  })
  it('attaches when a listener appears, debounces, and detaches after', () => {
    const ed = fakeEditor()
    renderHook(() => useDiffCursorEmitter(ed as never, 'wt', 'a.ts'))
    const l = vi.fn()
    const off = subscribeDiffCursorLine(l)
    expect(ed.onDidChangeCursorPosition).toHaveBeenCalledTimes(1)
    ed.fire(5)
    ed.fire(6)
    vi.advanceTimersByTime(149)
    expect(l).not.toHaveBeenCalled()
    vi.advanceTimersByTime(2)
    expect(l).toHaveBeenCalledTimes(1)
    expect(l).toHaveBeenCalledWith({ worktreeId: 'wt', relativePath: 'a.ts', line: 6 })
    off()
    expect(ed.dispose).toHaveBeenCalled()
  })
})
