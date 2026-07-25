// @vitest-environment happy-dom
import { renderHook } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { useAuthStatus, useAuthUser, useAuthSession } from '../useAuthSession'
import { useAppStore } from '../../store'

vi.mock('../../store', () => ({
  useAppStore: vi.fn()
}))

describe('useAuthStatus', () => {
  it('returns authenticated status from store', () => {
    vi.mocked(useAppStore).mockImplementation((selector: (s: any) => any) =>
      selector({ authStatus: 'authenticated', currentUser: null, authError: null })
    )
    const { result } = renderHook(() => useAuthStatus())
    expect(result.current).toBe('authenticated')
  })

  it('returns unknown status before bootstrap', () => {
    vi.mocked(useAppStore).mockImplementation((selector: (s: any) => any) =>
      selector({ authStatus: 'unknown', currentUser: null, authError: null })
    )
    const { result } = renderHook(() => useAuthStatus())
    expect(result.current).toBe('unknown')
  })

  it('returns unauthenticated when no session', () => {
    vi.mocked(useAppStore).mockImplementation((selector: (s: any) => any) =>
      selector({ authStatus: 'unauthenticated', currentUser: null, authError: null })
    )
    const { result } = renderHook(() => useAuthStatus())
    expect(result.current).toBe('unauthenticated')
  })
})

describe('useAuthUser', () => {
  it('returns currentUser when authenticated', () => {
    const mockUser = { id: 'u1', email: 'a@b.com', name: 'A', role: 'developer', teams: [], projects: [] }
    vi.mocked(useAppStore).mockImplementation((selector: (s: any) => any) =>
      selector({ authStatus: 'authenticated', currentUser: mockUser, authError: null })
    )
    const { result } = renderHook(() => useAuthUser())
    expect(result.current?.email).toBe('a@b.com')
  })

  it('returns null when not authenticated', () => {
    vi.mocked(useAppStore).mockImplementation((selector: (s: any) => any) =>
      selector({ authStatus: 'unauthenticated', currentUser: null, authError: null })
    )
    const { result } = renderHook(() => useAuthUser())
    expect(result.current).toBeNull()
  })
})

describe('useAuthSession', () => {
  it('returns full session shape', () => {
    const mockUser = { id: 'u1', email: 'a@b.com', name: 'A', role: 'developer', teams: [], projects: [] }
    vi.mocked(useAppStore).mockImplementation((selector: (s: any) => any) =>
      selector({ authStatus: 'authenticated', currentUser: mockUser, authError: null })
    )
    const { result } = renderHook(() => useAuthSession())
    expect(result.current.status).toBe('authenticated')
    expect(result.current.user?.email).toBe('a@b.com')
    expect(result.current.error).toBeNull()
  })
})
