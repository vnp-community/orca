import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { parseProgressLine, createProgressLimiter } from './codeintel-reindex-progress'

describe('codeintel-reindex-progress', () => {
  beforeEach(() => {
    process.env.HOME = '/Users/test'
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('parseProgressLine removes ANSI, parses percent, and redacts HOME', () => {
    const line = '\x1b[32m[INFO]\x1b[0m Processing 42% of /Users/test/workspace/file.ts'
    const res = parseProgressLine(line)
    expect(res.message).toBe('[INFO] Processing 42% of ~/workspace/file.ts')
    expect(res.percent).toBe(42)
  })

  it('ignores invalid percentages', () => {
    expect(parseProgressLine('Progress 150%').percent).toBe(null)
    expect(parseProgressLine('Progress -10%').percent).toBe(null) // '-' is not in \d
  })

  it('truncates long messages', () => {
    const long = 'a'.repeat(300)
    const res = parseProgressLine(long)
    expect(res.message.length).toBe(200)
    expect(res.message.endsWith('...')).toBe(true)
  })

  it('limits progress emissions to 1/s unless state changes', () => {
    const onEmit = vi.fn()
    const emit = createProgressLimiter(onEmit)

    emit('m1', 10) // called
    expect(onEmit).toHaveBeenCalledTimes(1)

    vi.advanceTimersByTime(500)
    emit('m2', 20) // dropped
    expect(onEmit).toHaveBeenCalledTimes(1)

    vi.advanceTimersByTime(500)
    emit('m3', 30) // called
    expect(onEmit).toHaveBeenCalledTimes(2)

    emit('m4', 30, true) // called because stateChanged
    expect(onEmit).toHaveBeenCalledTimes(3)
  })
})
