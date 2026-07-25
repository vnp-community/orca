import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useActiveDevServer } from '../../store/slices/dev-servers'
import {
  useActiveDevServerPlatform,
  useShowWindowsTerminalStep,
  useShowGhosttyImport,
  useIsLinuxDevServer,
} from '../useActiveDevServerPlatform'

// Mock the store slice hook
vi.mock('../../store/slices/dev-servers', () => ({
  useActiveDevServer: vi.fn(),
}))

describe('useActiveDevServerPlatform', () => {
  beforeEach(() => {
    vi.mocked(useActiveDevServer).mockReset()
  })

  it('returns null when no active dev server', () => {
    vi.mocked(useActiveDevServer).mockReturnValue(null)
    
    const { result } = renderHook(() => useActiveDevServerPlatform())
    expect(result.current).toBeNull()
  })

  it('returns platform when dev server is connected', () => {
    vi.mocked(useActiveDevServer).mockReturnValue({ platform: 'darwin' } as any)
    
    const { result } = renderHook(() => useActiveDevServerPlatform())
    expect(result.current).toBe('darwin')
  })

  it('useShowWindowsTerminalStep returns true only for win32', () => {
    vi.mocked(useActiveDevServer).mockReturnValue({ platform: 'win32' } as any)
    const { result: win } = renderHook(() => useShowWindowsTerminalStep())
    expect(win.current).toBe(true)
    
    vi.mocked(useActiveDevServer).mockReturnValue({ platform: 'darwin' } as any)
    const { result: mac } = renderHook(() => useShowWindowsTerminalStep())
    expect(mac.current).toBe(false)
  })

  it('useShowGhosttyImport returns true only for darwin', () => {
    vi.mocked(useActiveDevServer).mockReturnValue({ platform: 'darwin' } as any)
    const { result: mac } = renderHook(() => useShowGhosttyImport())
    expect(mac.current).toBe(true)
    
    vi.mocked(useActiveDevServer).mockReturnValue({ platform: 'win32' } as any)
    const { result: win } = renderHook(() => useShowGhosttyImport())
    expect(win.current).toBe(false)
  })

  it('useIsLinuxDevServer returns true only for linux', () => {
    vi.mocked(useActiveDevServer).mockReturnValue({ platform: 'linux' } as any)
    const { result: linux } = renderHook(() => useIsLinuxDevServer())
    expect(linux.current).toBe(true)
    
    vi.mocked(useActiveDevServer).mockReturnValue({ platform: 'darwin' } as any)
    const { result: mac } = renderHook(() => useIsLinuxDevServer())
    expect(mac.current).toBe(false)
  })
})
