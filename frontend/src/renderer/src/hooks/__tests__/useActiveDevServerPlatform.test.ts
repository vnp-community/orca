// @vitest-environment happy-dom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useActiveDevServer } from '../../store/slices/dev-servers-selectors'
import {
  useActiveDevServerPlatform,
  useShowWindowsTerminalStep,
  useShowGhosttyImport,
  useIsLinuxDevServer
} from '../useActiveDevServerPlatform'
import type { DevServer } from '../../../../shared/dev-server-types'

// Mock the store slice hook
vi.mock('../../store/slices/dev-servers-selectors', () => ({
  useActiveDevServer: vi.fn()
}))

// Only `platform` is read by the hooks under test — the rest of these fields
// just need to satisfy the real DevServer shape the mocked hook returns.
function makeDevServer(platform: NodeJS.Platform): DevServer {
  return {
    id: 'ds-test',
    name: 'Test Dev Server',
    connectionType: 'relay-ssh',
    status: 'connected',
    platform,
    arch: null,
    nodeVersion: null,
    lastConnectedAt: null,
    lastError: null,
    workspaceDir: null,
    addedAt: 0,
    capabilities: null
  }
}

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
    vi.mocked(useActiveDevServer).mockReturnValue(makeDevServer('darwin'))

    const { result } = renderHook(() => useActiveDevServerPlatform())
    expect(result.current).toBe('darwin')
  })

  it('useShowWindowsTerminalStep returns true only for win32', () => {
    vi.mocked(useActiveDevServer).mockReturnValue(makeDevServer('win32'))
    const { result: win } = renderHook(() => useShowWindowsTerminalStep())
    expect(win.current).toBe(true)

    vi.mocked(useActiveDevServer).mockReturnValue(makeDevServer('darwin'))
    const { result: mac } = renderHook(() => useShowWindowsTerminalStep())
    expect(mac.current).toBe(false)
  })

  it('useShowGhosttyImport returns true only for darwin', () => {
    vi.mocked(useActiveDevServer).mockReturnValue(makeDevServer('darwin'))
    const { result: mac } = renderHook(() => useShowGhosttyImport())
    expect(mac.current).toBe(true)

    vi.mocked(useActiveDevServer).mockReturnValue(makeDevServer('win32'))
    const { result: win } = renderHook(() => useShowGhosttyImport())
    expect(win.current).toBe(false)
  })

  it('useIsLinuxDevServer returns true only for linux', () => {
    vi.mocked(useActiveDevServer).mockReturnValue(makeDevServer('linux'))
    const { result: linux } = renderHook(() => useIsLinuxDevServer())
    expect(linux.current).toBe(true)

    vi.mocked(useActiveDevServer).mockReturnValue(makeDevServer('darwin'))
    const { result: mac } = renderHook(() => useIsLinuxDevServer())
    expect(mac.current).toBe(false)
  })
})
