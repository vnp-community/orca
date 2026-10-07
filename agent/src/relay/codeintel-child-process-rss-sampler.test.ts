import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  parseLinuxProcStatus,
  parseMacPsOutput,
  createRssSampler
} from './codeintel-child-process-rss-sampler'

describe('codeintel-child-process-rss-sampler', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('parses VmHWM and falls back to VmRSS', () => {
    expect(parseLinuxProcStatus('Name:\tcat\nVmHWM:\t 1234 kB\nVmRSS:\t 1000 kB\n')).toBe(1234)
    expect(parseLinuxProcStatus('Name:\tcat\nVmRSS:\t 1000 kB\n')).toBe(1000)
    expect(parseLinuxProcStatus('Name:\tcat\n')).toBe(undefined)
  })

  it('macOS reader invokes ps without a shell (parsable)', () => {
    expect(parseMacPsOutput('  1234 \n')).toBe(1234)
    expect(parseMacPsOutput('\n')).toBe(undefined)
    expect(parseMacPsOutput('invalid')).toBe(undefined)
  })

  it('returns the maximum across samples', async () => {
    const readProcStatus = vi.fn()
      .mockResolvedValueOnce('VmHWM: 1000 kB')
      .mockResolvedValueOnce('VmHWM: 1500 kB')
      .mockResolvedValueOnce('VmHWM: 1200 kB')

    const sampler = createRssSampler({
      pid: 123,
      platform: 'linux',
      intervalMs: 100,
      readProcStatus
    })

    // Advance timer a few times
    await vi.advanceTimersByTimeAsync(100)
    await vi.advanceTimersByTimeAsync(100)

    const peak = await sampler.stop()
    expect(peak).toBe(1500)
    expect(readProcStatus).toHaveBeenCalledTimes(3)
  })

  it('returns undefined when every read fails', async () => {
    const runPs = vi.fn().mockRejectedValue(new Error('Process exited'))
    
    const sampler = createRssSampler({
      pid: 123,
      platform: 'darwin',
      intervalMs: 200,
      runPs
    })

    await vi.advanceTimersByTimeAsync(200)

    const peak = await sampler.stop()
    expect(peak).toBe(undefined)
    expect(runPs).toHaveBeenCalledTimes(2)
  })

  it('stop clears the timer (fake timers) and never throws', async () => {
    const readProcStatus = vi.fn().mockResolvedValue('VmHWM: 1024 kB')
    
    const sampler = createRssSampler({
      pid: 123,
      platform: 'linux',
      intervalMs: 100,
      readProcStatus
    })

    const peak = await sampler.stop()
    expect(peak).toBe(1024)
    
    // Timer should be cleared, advancing it shouldn't trigger more calls
    await vi.advanceTimersByTimeAsync(500)
    expect(readProcStatus).toHaveBeenCalledTimes(1)
  })

  it('does not sample on win32', async () => {
    const sampler = createRssSampler({
      pid: 123,
      platform: 'win32'
    })

    const peak = await sampler.stop()
    expect(peak).toBe(undefined)
  })
})
