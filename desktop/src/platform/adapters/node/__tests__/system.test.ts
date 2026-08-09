import { describe, it, expect } from 'vitest'
import { NodeSystemInfo } from '../system'

describe('NodeSystemInfo', () => {
  const system = new NodeSystemInfo()

  it('getPlatform() returns known platform', () => {
    const known = ['linux', 'darwin', 'win32', 'freebsd', 'openbsd']
    expect(known).toContain(system.getPlatform())
  })

  it('getTotalMemory() > 0', () => {
    expect(system.getTotalMemory()).toBeGreaterThan(0)
  })

  it('getFreeMemory() > 0 and <= total', () => {
    expect(system.getFreeMemory()).toBeGreaterThan(0)
    expect(system.getFreeMemory()).toBeLessThanOrEqual(system.getTotalMemory())
  })

  it('getCpuCount() >= 1', () => {
    expect(system.getCpuCount()).toBeGreaterThanOrEqual(1)
  })

  it('getHostname() is non-empty string', () => {
    expect(typeof system.getHostname()).toBe('string')
    expect(system.getHostname().length).toBeGreaterThan(0)
  })
})
