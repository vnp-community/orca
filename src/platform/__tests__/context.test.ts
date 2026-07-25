import { describe, it, expect, beforeEach } from 'vitest'
import {
  setPlatform,
  getPlatform,
  isPlatformInitialized,
  _resetPlatformForTesting
} from '../context'
import type { IPlatformServices } from '../types'

function mockPlatform(): IPlatformServices {
  return {
    mode: 'node',
    app: {} as any,
    ipc: {} as any,
    windowManager: {} as any,
    storage: {} as any,
    system: {} as any
  }
}

describe('Platform Context', () => {
  beforeEach(() => {
    _resetPlatformForTesting()
  })

  it('isPlatformInitialized() returns false before setPlatform()', () => {
    expect(isPlatformInitialized()).toBe(false)
  })

  it('getPlatform() throws before setPlatform()', () => {
    expect(() => getPlatform()).toThrow('Platform not initialized')
  })

  it('setPlatform() + getPlatform() roundtrip', () => {
    const p = mockPlatform()
    setPlatform(p)
    expect(getPlatform()).toBe(p)
  })

  it('isPlatformInitialized() returns true after setPlatform()', () => {
    setPlatform(mockPlatform())
    expect(isPlatformInitialized()).toBe(true)
  })

  it('setPlatform() throws if called twice', () => {
    setPlatform(mockPlatform())
    expect(() => setPlatform(mockPlatform())).toThrow('Platform already initialized')
  })

  it('_resetPlatformForTesting() allows re-initialization', () => {
    setPlatform(mockPlatform())
    _resetPlatformForTesting()
    expect(isPlatformInitialized()).toBe(false)
    // Should not throw
    setPlatform(mockPlatform())
    expect(isPlatformInitialized()).toBe(true)
  })
})
