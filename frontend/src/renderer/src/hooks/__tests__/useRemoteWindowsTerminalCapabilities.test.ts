// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useRemoteWindowsTerminalCapabilities } from '../useRemoteWindowsTerminalCapabilities'

const MOCK_CAPS = {
  wslAvailable: true,
  wslDistros: ['Ubuntu'],
  pwshAvailable: true,
  pwshVersion: '7.3.0',
  gitBashAvailable: false,
  gitBashPath: undefined
}

describe('useRemoteWindowsTerminalCapabilities', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(1_000_000)
    vi.stubGlobal('window', {
      api: {
        onboarding: {
          detectWindowsCapabilities: vi.fn().mockResolvedValue(MOCK_CAPS)
        }
      }
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    // Clear the module-level capsCache between tests by using unique IDs per test
  })

  it('devServerId = null → DEFAULT_STATE, no API call', () => {
    const { result } = renderHook(() =>
      useRemoteWindowsTerminalCapabilities(null, true)
    )
    expect(result.current.wslAvailable).toBe(false)
    expect(result.current.loading).toBe(false)
    expect(window.api.onboarding.detectWindowsCapabilities).not.toHaveBeenCalled()
  })

  it('enabled = false → DEFAULT_STATE, no API call', () => {
    const { result } = renderHook(() =>
      useRemoteWindowsTerminalCapabilities('server-disabled', false)
    )
    expect(result.current.wslAvailable).toBe(false)
    expect(window.api.onboarding.detectWindowsCapabilities).not.toHaveBeenCalled()
  })

  it('cache miss → fetches API and populates state', async () => {
    const id = 'server-miss-01'
    const { result } = renderHook(() =>
      useRemoteWindowsTerminalCapabilities(id, true)
    )

    await act(async () => {
      await vi.runAllTimersAsync()
    })

    expect(window.api.onboarding.detectWindowsCapabilities).toHaveBeenCalledWith({
      devServerId: id
    })
    expect(result.current.wslAvailable).toBe(true)
    expect(result.current.wslDistros).toEqual(['Ubuntu'])
    expect(result.current.pwshAvailable).toBe(true)
    expect(result.current.loading).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('cache hit (< 60s) → uses cached value, no second API call', async () => {
    const id = 'server-cache-hit-02'

    // First render populates cache
    const { unmount } = renderHook(() =>
      useRemoteWindowsTerminalCapabilities(id, true)
    )
    await act(async () => {
      await vi.runAllTimersAsync()
    })
    expect(window.api.onboarding.detectWindowsCapabilities).toHaveBeenCalledTimes(1)
    unmount()

    // Second render within TTL
    const { result } = renderHook(() =>
      useRemoteWindowsTerminalCapabilities(id, true)
    )
    expect(result.current.wslAvailable).toBe(true)
    expect(window.api.onboarding.detectWindowsCapabilities).toHaveBeenCalledTimes(1) // still 1
  })

  it('loading state during fetch', async () => {
    const id = 'server-loading-03'
    let resolvePromise!: (v: typeof MOCK_CAPS) => void
    vi.mocked(window.api.onboarding.detectWindowsCapabilities).mockImplementationOnce(
      () => new Promise((res) => { resolvePromise = res })
    )

    const { result } = renderHook(() =>
      useRemoteWindowsTerminalCapabilities(id, true)
    )

    // Initially should go into loading
    await act(async () => {
      await Promise.resolve() // let useEffect run
    })
    expect(result.current.loading).toBe(true)

    // Resolve the fetch
    await act(async () => {
      resolvePromise(MOCK_CAPS)
      await vi.runAllTimersAsync()
    })
    expect(result.current.loading).toBe(false)
  })

  it('error state when API throws + retry clears error', async () => {
    const id = 'server-error-04'
    vi.mocked(window.api.onboarding.detectWindowsCapabilities).mockRejectedValueOnce(
      new Error('Connection failed')
    )

    const { result } = renderHook(() =>
      useRemoteWindowsTerminalCapabilities(id, true)
    )
    await act(async () => {
      await vi.runAllTimersAsync()
    })

    expect(result.current.error).toBe('Connection failed')
    expect(result.current.loading).toBe(false)

    // retry clears error and re-fetches
    vi.mocked(window.api.onboarding.detectWindowsCapabilities).mockResolvedValueOnce(MOCK_CAPS)
    await act(async () => {
      result.current.retry()
      await vi.runAllTimersAsync()
    })

    expect(result.current.error).toBeNull()
    expect(result.current.wslAvailable).toBe(true)
  })
})
