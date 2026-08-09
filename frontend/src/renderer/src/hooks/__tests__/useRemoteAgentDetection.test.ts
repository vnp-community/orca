// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useRemoteAgentDetection } from '../useRemoteAgentDetection'

describe('useRemoteAgentDetection', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(1000000)
    vi.stubGlobal('window', {
      api: {
        onboarding: {
          detectAgents: vi.fn().mockResolvedValue({ agents: ['claude'], platform: 'darwin' }),
        },
      },
    })
  })
  
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  // To clear the module cache, we can just use a unique devServerId for each test
  // since the cache is scoped to the module and we can't easily clear it without exporting it.

  it('devServerId = null → DEFAULT_STATE, không gọi API', () => {
    const { result } = renderHook(() => useRemoteAgentDetection(null))
    expect(result.current.agents).toEqual([])
    expect(result.current.platform).toBeNull()
    expect(window.api.onboarding.detectAgents).not.toHaveBeenCalled()
  })

  it('gọi window.api.onboarding.detectAgents với devServerId khi mount', async () => {
    const id = 'ds-1'
    const { result } = renderHook(() => useRemoteAgentDetection(id))
    
    // Initially loading
    expect(result.current.loading).toBe(true)
    
    // Wait for effect
    await act(async () => {
      await vi.runAllTimersAsync()
    })
    
    expect(window.api.onboarding.detectAgents).toHaveBeenCalledWith({ devServerId: id })
    expect(result.current.agents).toEqual(['claude'])
    expect(result.current.platform).toBe('darwin')
    expect(result.current.loading).toBe(false)
  })

  it('cache hit (< 60s) → không gọi API lần 2', async () => {
    const id = 'ds-2'
    
    // First render populates cache
    const { unmount } = renderHook(() => useRemoteAgentDetection(id))
    await act(async () => {
      await vi.runAllTimersAsync()
    })
    expect(window.api.onboarding.detectAgents).toHaveBeenCalledTimes(1)
    unmount()

    // Second render hits cache
    const { result } = renderHook(() => useRemoteAgentDetection(id))
    expect(result.current.loading).toBe(false)
    expect(result.current.agents).toEqual(['claude'])
    expect(window.api.onboarding.detectAgents).toHaveBeenCalledTimes(1) // still 1
  })

  it('cache miss sau 60s → gọi API lại', async () => {
    const id = 'ds-3'
    
    const { unmount } = renderHook(() => useRemoteAgentDetection(id))
    await act(async () => {
      await vi.runAllTimersAsync()
    })
    expect(window.api.onboarding.detectAgents).toHaveBeenCalledTimes(1)
    unmount()

    // Advance time > 60s
    vi.setSystemTime(1000000 + 60001)

    const { result } = renderHook(() => useRemoteAgentDetection(id))
    expect(result.current.loading).toBe(true)
    await act(async () => {
      await vi.runAllTimersAsync()
    })
    expect(window.api.onboarding.detectAgents).toHaveBeenCalledTimes(2)
  })

  it('error state khi API throw', async () => {
    const id = 'ds-4'
    vi.mocked(window.api.onboarding.detectAgents).mockRejectedValueOnce(new Error('Network error'))
    
    const { result } = renderHook(() => useRemoteAgentDetection(id))
    await act(async () => {
      await vi.runAllTimersAsync()
    })
    
    expect(result.current.error).toBe('Network error')
    expect(result.current.loading).toBe(false)
  })

  it('redetect() bỏ qua cache và gọi API', async () => {
    const id = 'ds-5'
    
    const { result } = renderHook(() => useRemoteAgentDetection(id))
    await act(async () => {
      await vi.runAllTimersAsync()
    })
    expect(window.api.onboarding.detectAgents).toHaveBeenCalledTimes(1)

    // Call redetect before 60s
    await act(async () => {
      await result.current.redetect()
    })
    
    expect(window.api.onboarding.detectAgents).toHaveBeenCalledTimes(2)
  })
})
