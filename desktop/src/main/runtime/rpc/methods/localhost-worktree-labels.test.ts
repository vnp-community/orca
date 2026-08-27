import { describe, expect, it, vi, beforeEach } from 'vitest'
import { RpcDispatcher } from '../dispatcher'
import type { RpcRequest } from '../core'
import type { OrcaRuntimeService } from '../../orca-runtime'

const { registerLocalhostWorktreeLabelRouteMock } = vi.hoisted(() => ({
  registerLocalhostWorktreeLabelRouteMock: vi.fn()
}))

vi.mock('../../../ipc/localhost-worktree-labels', () => ({
  registerLocalhostWorktreeLabelRoute: registerLocalhostWorktreeLabelRouteMock
}))

function makeRequest(method: string, params?: unknown): RpcRequest {
  return { id: 'req-1', authToken: 'tok', method, params }
}

describe('localhost worktree labels RPC methods', () => {
  beforeEach(() => {
    registerLocalhostWorktreeLabelRouteMock.mockReset()
  })

  it('registers a localhost worktree label route using the RPC-scoped store', async () => {
    registerLocalhostWorktreeLabelRouteMock.mockResolvedValue({
      url: 'http://my-worktree.localhost:3000',
      label: 'my-worktree'
    })
    const runtime = {
      getRuntimeId: () => 'test-runtime',
      getRuntimeStoreForRpc: vi.fn().mockReturnValue({})
    } as unknown as OrcaRuntimeService
    const { LOCALHOST_WORKTREE_LABEL_METHODS } = await import('./localhost-worktree-labels')
    const dispatcher = new RpcDispatcher({ runtime, methods: LOCALHOST_WORKTREE_LABEL_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('localhostWorktreeLabels.register', {
        targetUrl: 'http://localhost:3000',
        projectName: 'orca',
        worktreeName: 'my-worktree'
      })
    )

    expect(registerLocalhostWorktreeLabelRouteMock).toHaveBeenCalledWith(
      {},
      expect.objectContaining({ targetUrl: 'http://localhost:3000' })
    )
    expect(response).toMatchObject({
      ok: true,
      result: { url: 'http://my-worktree.localhost:3000' }
    })
  })

  it('rejects when required fields are missing', async () => {
    const runtime = {
      getRuntimeId: () => 'test-runtime',
      getRuntimeStoreForRpc: vi.fn().mockReturnValue({})
    } as unknown as OrcaRuntimeService
    const { LOCALHOST_WORKTREE_LABEL_METHODS } = await import('./localhost-worktree-labels')
    const dispatcher = new RpcDispatcher({ runtime, methods: LOCALHOST_WORKTREE_LABEL_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('localhostWorktreeLabels.register', { targetUrl: 'http://localhost:3000' })
    )

    expect(response).toMatchObject({ ok: false })
    expect(registerLocalhostWorktreeLabelRouteMock).not.toHaveBeenCalled()
  })
})
