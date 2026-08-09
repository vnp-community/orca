// @vitest-environment happy-dom
import { renderHook, act } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { useLogout } from '../useLogout'
import { useAppStore } from '../../store'
import * as authApiClient from '../../auth/auth-api-client'

vi.mock('../../store', () => ({
  useAppStore: vi.fn()
}))
vi.mock('../../auth/auth-api-client')

const mockClearAuth = vi.fn()

describe('useLogout', () => {
  beforeEach(() => {
    vi.mocked(useAppStore).mockImplementation((selector: (s: any) => any) =>
      selector({ clearAuth: mockClearAuth })
    )
    // Stub window.location.href setter
    Object.defineProperty(window, 'location', {
      value: { href: '' },
      writable: true
    })
  })
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('calls logoutUser API then clearAuth', async () => {
    vi.mocked(authApiClient.logoutUser).mockResolvedValueOnce(undefined)
    const { result } = renderHook(() => useLogout())
    await act(async () => {
      await result.current()
    })
    expect(authApiClient.logoutUser).toHaveBeenCalledOnce()
    expect(mockClearAuth).toHaveBeenCalledOnce()
  })

  it('still clears auth even if logoutUser throws', async () => {
    vi.mocked(authApiClient.logoutUser).mockRejectedValueOnce(new Error('Network'))
    const { result } = renderHook(() => useLogout())
    await act(async () => {
      await result.current()
    })
    // clearAuth must still be called despite the API error
    expect(mockClearAuth).toHaveBeenCalledOnce()
  })

  it('redirects to /login after logout', async () => {
    vi.mocked(authApiClient.logoutUser).mockResolvedValueOnce(undefined)
    const { result } = renderHook(() => useLogout())
    await act(async () => {
      await result.current()
    })
    expect(window.location.href).toBe('/login')
  })
})
