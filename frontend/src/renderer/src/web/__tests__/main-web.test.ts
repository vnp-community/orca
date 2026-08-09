// FIX CR-FE2E-003: main.tsx must only dynamically import pair-code-app-entry
// (and therefore the E2EE pairing bundle) when /auth/config 404s. This test
// protects that branch directly, independent of TASK-FE2E-008's bundle-size
// measurement (which confirms the same thing at the build-artifact level).
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const bootstrapWebApp = vi.fn().mockResolvedValue(undefined)
const mountPairCodeApp = vi.fn()

vi.mock('../main-web-bootstrap', () => ({ bootstrapWebApp }))
vi.mock('../pair-code-app-entry', () => ({ mountPairCodeApp }))
vi.mock('../../assets/main.css', () => ({}))

describe('web/main.tsx — /auth/config branch (CR-FE2E-003)', () => {
  beforeEach(() => {
    vi.resetModules()
    bootstrapWebApp.mockClear()
    mountPairCodeApp.mockClear()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('dynamically imports pair-code-app-entry only when /auth/config 404s', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: false, status: 404 } as Response)
    )
    await import('../main')
    await new Promise((resolve) => setTimeout(resolve, 0))
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(mountPairCodeApp).toHaveBeenCalledTimes(1)
    expect(bootstrapWebApp).not.toHaveBeenCalled()
  })

  it('does NOT import pair-code-app-entry when /auth/config returns 200', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, status: 200 } as Response)
    )
    await import('../main')
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(bootstrapWebApp).toHaveBeenCalledTimes(1)
    expect(mountPairCodeApp).not.toHaveBeenCalled()
  })

  it('falls back to pair-code-app-entry on network error', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network down')))
    await import('../main')
    await new Promise((resolve) => setTimeout(resolve, 0))
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(mountPairCodeApp).toHaveBeenCalledTimes(1)
    expect(bootstrapWebApp).not.toHaveBeenCalled()
  })
})
