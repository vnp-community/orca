import { describe, expect, it, vi, beforeEach } from 'vitest'
import { RpcDispatcher } from '../dispatcher'
import type { RpcRequest } from '../core'
import type { OrcaRuntimeService } from '../../orca-runtime'
import type {
  WorkspaceSpaceAnalysis,
  WorkspaceSpaceScanProgress
} from '../../../../shared/workspace-space-types'

const { analyzeWorkspaceSpaceMock } = vi.hoisted(() => ({
  analyzeWorkspaceSpaceMock: vi.fn()
}))

vi.mock('../../../workspace-space-analysis', () => ({
  analyzeWorkspaceSpace: analyzeWorkspaceSpaceMock,
  WorkspaceSpaceScanCancelledError: class WorkspaceSpaceScanCancelledError extends Error {}
}))

function makeRequest(method: string, params?: unknown): RpcRequest {
  return { id: 'req-1', authToken: 'tok', method, params }
}

function makeStoreRuntime(): OrcaRuntimeService {
  return {
    getRuntimeId: () => 'test-runtime',
    getRuntimeStoreForRpc: vi.fn().mockReturnValue({ getRepos: vi.fn(), getWorktreeMeta: vi.fn() })
  } as unknown as OrcaRuntimeService
}

describe('workspace space RPC methods', () => {
  beforeEach(() => {
    analyzeWorkspaceSpaceMock.mockReset()
  })

  it('reports no in-flight scan to cancel', async () => {
    const runtime = makeStoreRuntime()
    const { WORKSPACE_SPACE_METHODS } = await import('./workspace-space')
    const dispatcher = new RpcDispatcher({ runtime, methods: WORKSPACE_SPACE_METHODS })

    const response = await dispatcher.dispatch(makeRequest('workspaceSpace.cancel'))

    expect(response).toMatchObject({ ok: true, result: false })
  })

  it('streams progress then the final result for workspaceSpace.analyze', async () => {
    let resolveScan!: (analysis: WorkspaceSpaceAnalysis) => void
    analyzeWorkspaceSpaceMock.mockImplementation((_store, options) => {
      const progress: WorkspaceSpaceScanProgress = {
        scanId: options.scanId,
        state: 'running',
        startedAt: Date.now(),
        updatedAt: Date.now(),
        totalRepoCount: 1,
        scannedRepoCount: 0,
        totalWorktreeCount: 1,
        scannedWorktreeCount: 1,
        currentRepoDisplayName: 'repo',
        currentWorktreeDisplayName: 'wt'
      }
      options.onProgress(progress)
      return new Promise((resolve) => {
        resolveScan = resolve
      })
    })
    const runtime = makeStoreRuntime()
    const { WORKSPACE_SPACE_METHODS } = await import('./workspace-space')
    const dispatcher = new RpcDispatcher({ runtime, methods: WORKSPACE_SPACE_METHODS })
    const replies: unknown[] = []

    const dispatch = dispatcher.dispatchStreaming(makeRequest('workspaceSpace.analyze'), (response) =>
      replies.push(JSON.parse(response))
    )

    await vi.waitFor(() => {
      expect(replies).toHaveLength(1)
    })
    expect(replies[0]).toMatchObject({
      ok: true,
      streaming: true,
      result: { type: 'progress', progress: { state: 'running' } }
    })

    resolveScan({
      scannedAt: 1,
      totalSizeBytes: 0,
      reclaimableBytes: 0,
      worktreeCount: 1,
      scannedWorktreeCount: 1,
      unavailableWorktreeCount: 0,
      repos: [],
      worktrees: []
    })

    await dispatch
    expect(replies).toHaveLength(2)
    expect(replies[1]).toMatchObject({
      ok: true,
      streaming: true,
      result: { type: 'result', result: { ok: true } }
    })
    expect(runtime.getRuntimeStoreForRpc).toHaveBeenCalled()
  })
})
