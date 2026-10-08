import { beforeEach, describe, expect, it, vi } from 'vitest'

const rpc = vi.hoisted(() => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn()
}))
vi.mock('@/runtime/runtime-rpc-client', () => rpc)
vi.mock('@/store', () => ({ useAppStore: { getState: () => ({ settings: { s: 1 } }) } }))

import { markAnnotationsSentBestEffort } from './annotation-mark-sent-best-effort'

beforeEach(() => {
  rpc.callRuntimeRpc.mockReset()
  rpc.getActiveRuntimeTarget.mockReset()
})

describe('markAnnotationsSentBestEffort', () => {
  it('does nothing for an empty id list', () => {
    markAnnotationsSentBestEffort([])
    expect(rpc.getActiveRuntimeTarget).not.toHaveBeenCalled()
    expect(rpc.callRuntimeRpc).not.toHaveBeenCalled()
  })

  it('does nothing for the local target', () => {
    rpc.getActiveRuntimeTarget.mockReturnValue({ kind: 'local' })
    markAnnotationsSentBestEffort(['a'])
    expect(rpc.callRuntimeRpc).not.toHaveBeenCalled()
  })

  it('calls annotation.markSent with an 8 s timeout for an environment target', () => {
    const target = { kind: 'environment', environmentId: 'e' }
    rpc.getActiveRuntimeTarget.mockReturnValue(target)
    rpc.callRuntimeRpc.mockResolvedValue({})
    markAnnotationsSentBestEffort(['a', 'b'])
    expect(rpc.getActiveRuntimeTarget).toHaveBeenCalledWith({ s: 1 })
    expect(rpc.callRuntimeRpc).toHaveBeenCalledWith(
      target,
      'annotation.markSent',
      { ids: ['a', 'b'] },
      { timeoutMs: 8000 }
    )
  })

  it('swallows RPC failures', async () => {
    rpc.getActiveRuntimeTarget.mockReturnValue({ kind: 'environment' })
    rpc.callRuntimeRpc.mockRejectedValue(new Error('boom'))
    expect(() => markAnnotationsSentBestEffort(['a'])).not.toThrow()
    await Promise.resolve()
  })
})
