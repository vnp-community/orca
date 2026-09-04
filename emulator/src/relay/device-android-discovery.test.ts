import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { discoverAndroidSdk } from './device-android-discovery'

const existsSyncMock = vi.fn<(path: string) => boolean>()

// vi.mock calls are hoisted above imports by vitest's transform, so this
// still applies before device-android-discovery.ts's own `import { existsSync
// } from 'node:fs'` resolves — see https://vitest.dev/api/vi.html#vi-mock
vi.mock('node:fs', () => ({
  existsSync: (path: string) => existsSyncMock(path)
}))

describe('discoverAndroidSdk', () => {
  beforeEach(() => {
    existsSyncMock.mockReset()
    existsSyncMock.mockReturnValue(false)
  })

  afterEach(() => {
    delete process.env['ANDROID_HOME']
    delete process.env['ANDROID_SDK_ROOT']
  })

  it('returns sdkFound=false when nothing matches', () => {
    const result = discoverAndroidSdk(null)
    expect(result.sdkFound).toBe(false)
    expect(result.message).toContain('Android SDK not found')
  })

  it('prefers an explicit override path when it looks like a valid SDK root', () => {
    existsSyncMock.mockImplementation((p) => String(p).includes('/custom/sdk'))
    const result = discoverAndroidSdk('/custom/sdk')
    expect(result.sdkFound).toBe(true)
    expect(result.sdkPath).toBe('/custom/sdk')
  })

  it('falls back to ANDROID_HOME when no override is given', () => {
    process.env['ANDROID_HOME'] = '/env/sdk'
    existsSyncMock.mockImplementation((p) => String(p).includes('/env/sdk'))
    const result = discoverAndroidSdk(null)
    expect(result.sdkFound).toBe(true)
    expect(result.sdkPath).toBe('/env/sdk')
  })
})
