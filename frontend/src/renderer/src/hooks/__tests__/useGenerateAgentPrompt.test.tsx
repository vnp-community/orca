// @vitest-environment happy-dom
import { renderHook, act, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { useGenerateAgentPrompt } from '../useGenerateAgentPrompt'

vi.mock('../../store', () => ({
  useAppStore: Object.assign(vi.fn(), { getState: () => ({ settings: {} }) })
}))
vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)

describe('useGenerateAgentPrompt', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('returns the prompt and always previews with save=false', async () => {
    mockRpc.mockResolvedValue({ prompt: 'generated' })
    const { result } = renderHook(() => useGenerateAgentPrompt())
    let out: unknown
    await act(async () => {
      out = await result.current.generate('t1')
    })
    expect(out).toEqual({ prompt: 'generated' })
    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.generateAgentPrompt', {
      taskId: 't1',
      save: false
    })
    expect(result.current.isGenerating).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it.each([
    ['PermissionDenied: nope', /permission/i],
    ['FailedPrecondition: no dev server', /dev server/i],
    ['method not supported', /not supported/i]
  ])('maps %s to a readable error without throwing', async (raw, expected) => {
    mockRpc.mockRejectedValue(new Error(raw))
    const { result } = renderHook(() => useGenerateAgentPrompt())
    let out: { error?: string } = {}
    await act(async () => {
      out = (await result.current.generate('t1')) as { error?: string }
    })
    expect(out.error).toMatch(expected)
    expect(result.current.error).toMatch(expected)
    expect(result.current.isGenerating).toBe(false)
  })

  it('ignores a second call while one is in flight', async () => {
    let resolve!: (v: { prompt: string }) => void
    mockRpc.mockReturnValue(new Promise((r) => (resolve = r)))
    const { result } = renderHook(() => useGenerateAgentPrompt())
    let first!: Promise<unknown>
    let second: unknown
    await act(async () => {
      first = result.current.generate('t1')
      second = await result.current.generate('t1')
    })
    expect(second).toHaveProperty('error')
    expect(mockRpc).toHaveBeenCalledTimes(1)
    await act(async () => {
      resolve({ prompt: 'x' })
      await first
    })
    expect(result.current.isGenerating).toBe(false)
  })
})
