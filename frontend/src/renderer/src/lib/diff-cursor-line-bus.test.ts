import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  emitDiffCursorLine,
  hasDiffCursorListeners,
  isDiffCursorSuppressed,
  onDiffCursorListenersChange,
  resetDiffCursorLineBusForTests,
  subscribeDiffCursorLine,
  suppressDiffCursorFor
} from './diff-cursor-line-bus'

afterEach(resetDiffCursorLineBusForTests)
const ev = { relativePath: 'a.ts', line: 3 }

describe('diff-cursor-line-bus', () => {
  it('has no listeners by default and emit is a no-op', () => {
    expect(hasDiffCursorListeners()).toBe(false)
    expect(() => emitDiffCursorLine(ev)).not.toThrow()
  })
  it('delivers to listeners and stops after unsubscribe', () => {
    const l = vi.fn()
    const off = subscribeDiffCursorLine(l)
    emitDiffCursorLine(ev)
    off()
    emitDiffCursorLine(ev)
    expect(l).toHaveBeenCalledTimes(1)
  })
  it('reports listener presence changes', () => {
    const c = vi.fn()
    onDiffCursorListenersChange(c)
    const off = subscribeDiffCursorLine(() => {})
    off()
    expect(c.mock.calls.map((x) => x[0])).toEqual([false, true, false])
  })
  it('suppresses events for the window', () => {
    const l = vi.fn()
    subscribeDiffCursorLine(l)
    suppressDiffCursorFor(500, Date.now())
    expect(isDiffCursorSuppressed()).toBe(true)
    emitDiffCursorLine(ev)
    expect(l).not.toHaveBeenCalled()
    expect(isDiffCursorSuppressed(Date.now() + 600)).toBe(false)
  })
})
