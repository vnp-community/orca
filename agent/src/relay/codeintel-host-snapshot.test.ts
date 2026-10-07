import { describe, it, expect } from 'vitest'
import { readHostSnapshot } from './codeintel-host-snapshot'

describe('readHostSnapshot', () => {
  it('returns valid system metrics using real os module', () => {
    const snapshot = readHostSnapshot()
    expect(typeof snapshot.platform).toBe('string')
    expect(snapshot.cores).toBeGreaterThanOrEqual(1)
    expect(snapshot.loadavg1).toBeGreaterThanOrEqual(0)
    expect(snapshot.freeMemBytes).toBeGreaterThanOrEqual(0)
  })

  it('uses injected os module and rounds loadavg1', () => {
    const fakeOs = {
      platform: () => 'linux' as NodeJS.Platform,
      availableParallelism: () => 8,
      loadavg: () => [1.234567, 1.0, 1.0],
      freemem: () => 1024 * 1024
    }

    const snapshot = readHostSnapshot({ os: fakeOs as any })
    expect(snapshot).toEqual({
      platform: 'linux',
      cores: 8,
      loadavg1: 1.23, // 1.234567 rounded to 2 decimals
      freeMemBytes: 1048576
    })
  })
})
