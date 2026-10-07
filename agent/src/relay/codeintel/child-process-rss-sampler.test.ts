import { describe, it, expect, vi } from 'vitest'
import {
  parseLinuxProcStatus,
  parseMacOsPsOutput,
  createRssSampler
} from './child-process-rss-sampler'

describe('child-process-rss-sampler', () => {
  it('parses VmHWM and falls back to VmRSS', () => {
    const textWithHwm = `
Name:   node
VmHWM:      10240 kB
VmRSS:       5120 kB
    `.trim()
    expect(parseLinuxProcStatus(textWithHwm)).toBe(10240)

    const textWithRssOnly = `
Name:   node
VmRSS:       5120 kB
    `.trim()
    expect(parseLinuxProcStatus(textWithRssOnly)).toBe(5120)

    expect(parseLinuxProcStatus('empty status')).toBeNull()
  })

  it('parses macOS ps output', () => {
    expect(parseMacOsPsOutput('  12345 \n')).toBe(12345)
    expect(parseMacOsPsOutput('')).toBeNull()
    expect(parseMacOsPsOutput('invalid')).toBeNull()
  })

  it('returns the maximum across samples', async () => {
    let callCount = 0
    const mockReader = vi.fn().mockImplementation(async () => {
      callCount++
      if (callCount === 1) return 1000
      if (callCount === 2) return 5000
      return 2000
    })

    const sampler = createRssSampler({
      pid: 1234,
      intervalMs: 10,
      platform: 'linux',
      reader: mockReader
    })

    // Wait for at least 2 samples
    await new Promise(r => setTimeout(r, 30))
    const peak = await sampler.stop()

    expect(peak).toBe(5000)
    expect(mockReader).toHaveBeenCalled()
  })

  it('returns undefined when every read fails', async () => {
    const mockReader = vi.fn().mockResolvedValue(null)
    const sampler = createRssSampler({
      pid: 1234,
      intervalMs: 10,
      platform: 'linux',
      reader: mockReader
    })

    const peak = await sampler.stop()
    expect(peak).toBeUndefined()
  })

  it('stop clears the timer and never throws even when reader throws', async () => {
    const mockReader = vi.fn().mockRejectedValue(new Error('Process vanished'))
    const sampler = createRssSampler({
      pid: 1234,
      intervalMs: 10,
      platform: 'linux',
      reader: mockReader
    })

    await expect(sampler.stop()).resolves.toBeUndefined()
  })

  it('does not sample on win32', async () => {
    const mockReader = vi.fn().mockResolvedValue(1000)
    const sampler = createRssSampler({
      pid: 1234,
      intervalMs: 10,
      platform: 'win32',
      reader: mockReader
    })

    const peak = await sampler.stop()
    expect(peak).toBeUndefined()
    expect(mockReader).not.toHaveBeenCalled()
  })
})
