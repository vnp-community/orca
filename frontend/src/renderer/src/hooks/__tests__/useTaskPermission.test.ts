// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { useTaskPermission } from '../useTaskPermission'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

vi.mock('../../store', () => ({
  useAppStore: Object.assign(vi.fn(), { getState: () => ({ settings: {} }) })
}))

const mockRpc = vi.mocked(callRuntimeRpc)

describe('useTaskPermission', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('no userId → does not call the RPC, level stays null, isSupported stays true', () => {
    const { result } = renderHook(() => useTaskPermission('t1', undefined))
    expect(mockRpc).not.toHaveBeenCalled()
    expect(result.current.level).toBeNull()
    expect(result.current.isSupported).toBe(true)
  })

  it('RPC succeeds → parses effectiveLevel (GRANT_LEVEL_* enum string) into TaskGrantLevel', async () => {
    mockRpc.mockResolvedValueOnce({ effectiveLevel: 'GRANT_LEVEL_ADMIN' })
    const { result } = renderHook(() => useTaskPermission('t1', 'u1'))

    await waitFor(() => {
      expect(result.current.level).toBe('admin')
    })
    expect(result.current.isSupported).toBe(true)
    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.resolvePermission', {
      taskId: 't1',
      userId: 'u1'
    })
  })

  it('RPC fails (method not wired yet) → isSupported becomes false, feature-detected instead of hard-coded', async () => {
    mockRpc.mockRejectedValueOnce(new Error('method not found'))
    const { result } = renderHook(() => useTaskPermission('t1', 'u1'))

    await waitFor(() => {
      expect(result.current.isSupported).toBe(false)
    })
    expect(result.current.level).toBeNull()
  })
})
