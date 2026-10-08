// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const state = vi.hoisted(() => ({ theme: 'light' as string | undefined }))
vi.mock('@/store', () => ({
  useAppStore: (sel: (s: unknown) => unknown) => sel({ settings: state.theme === undefined ? null : { theme: state.theme } })
}))

import { useIsDarkTheme } from './useIsDarkTheme'

let listener: ((e: { matches: boolean }) => void) | null = null
function stubMatchMedia(matches: boolean | null): void {
  if (matches === null) {
    vi.stubGlobal('matchMedia', undefined)
    ;(window as unknown as { matchMedia: unknown }).matchMedia = undefined
    return
  }
  const impl = () => ({
    matches,
    addEventListener: (_: string, fn: (e: { matches: boolean }) => void) => { listener = fn },
    removeEventListener: () => { listener = null }
  })
  ;(window as unknown as { matchMedia: unknown }).matchMedia = impl
}

beforeEach(() => { listener = null })
afterEach(() => vi.unstubAllGlobals())

describe('useIsDarkTheme', () => {
  it('follows explicit dark and light', () => {
    stubMatchMedia(true)
    state.theme = 'dark'
    expect(renderHook(() => useIsDarkTheme()).result.current).toBe(true)
    state.theme = 'light'
    expect(renderHook(() => useIsDarkTheme()).result.current).toBe(false)
  })

  it('uses the OS preference for system and reacts to changes', () => {
    stubMatchMedia(false)
    state.theme = 'system'
    const { result } = renderHook(() => useIsDarkTheme())
    expect(result.current).toBe(false)
    act(() => listener?.({ matches: true }))
    expect(result.current).toBe(true)
  })

  it('is false when matchMedia is missing or settings are not loaded', () => {
    stubMatchMedia(null)
    state.theme = 'system'
    expect(renderHook(() => useIsDarkTheme()).result.current).toBe(false)
    state.theme = undefined
    expect(renderHook(() => useIsDarkTheme()).result.current).toBe(false)
  })
})
