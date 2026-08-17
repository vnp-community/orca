import { describe, expect, it, vi, beforeEach } from 'vitest'
import { RpcDispatcher } from '../dispatcher'
import type { RpcRequest } from '../core'
import type { OrcaRuntimeService } from '../../orca-runtime'
import type { WorkspaceCleanupScanProgress } from '../../../../shared/workspace-cleanup'

const { hasKillableProcessesMock, mergeWorkspaceCleanupDismissalsMock, scanWorkspaceCleanupMock } =
  vi.hoisted(() => ({
    hasKillableProcessesMock: vi.fn(),
    mergeWorkspaceCleanupDismissalsMock: vi.fn(),
    scanWorkspaceCleanupMock: vi.fn()
  }))

vi.mock('../../../ipc/workspace-cleanup', () => ({
  hasKillableProcesses: hasKillableProcessesMock,
  mergeWorkspaceCleanupDismissals: mergeWorkspaceCleanupDismissalsMock,
  scanWorkspaceCleanup: scanWorkspaceCleanupMock
}))

function makeRequest(method: string, params?: unknown): RpcRequest {
  return { id: 'req-1', authToken: 'tok', method, params }
}

function makeStoreRuntime(): OrcaRuntimeService {
  return {
    getRuntimeId: () => 'test-runtime',
    getRuntimeStoreForRpc: vi.fn().mockReturnValue({}),
    getUIState: vi.fn().mockReturnValue({ workspaceCleanup: { dismissals: { a: {} } } }),
    updateUIState: vi.fn().mockReturnValue({}),
    getLocalProvider: vi.fn().mockReturnValue(null)
  } as unknown as OrcaRuntimeService
}

describe('workspace cleanup RPC methods', () => {
  beforeEach(() => {
    hasKillableProcessesMock.mockReset()
    mergeWorkspaceCleanupDismissalsMock.mockReset()
    scanWorkspaceCleanupMock.mockReset()
  })

  it('streams scan progress then the final result', async () => {
    scanWorkspaceCleanupMock.mockImplementation(async (_store, args, options) => {
      const progress: WorkspaceCleanupScanProgress = {
        scanId: args.scanId,
        scannedAt: 1,
        candidates: [],
        errors: [],
        scannedWorktreeCount: 0,
        totalWorktreeCount: 1
      }
      options.onProgress(progress)
      return { scannedAt: 2, candidates: [], errors: [] }
    })
    const runtime = makeStoreRuntime()
    const { WORKSPACE_CLEANUP_METHODS } = await import('./workspace-cleanup')
    const dispatcher = new RpcDispatcher({ runtime, methods: WORKSPACE_CLEANUP_METHODS })
    const replies: unknown[] = []

    await dispatcher.dispatchStreaming(makeRequest('workspaceCleanup.scan', {}), (response) =>
      replies.push(JSON.parse(response))
    )

    expect(replies).toHaveLength(2)
    expect(replies[0]).toMatchObject({ result: { type: 'progress' } })
    expect(replies[1]).toMatchObject({ result: { type: 'result', result: { scannedAt: 2 } } })
    expect(scanWorkspaceCleanupMock).toHaveBeenCalledWith(
      {},
      expect.objectContaining({ scanId: expect.any(String) }),
      expect.objectContaining({ onProgress: expect.any(Function) })
    )
  })

  it('merges dismissals through the shared UI state accessor', async () => {
    mergeWorkspaceCleanupDismissalsMock.mockReturnValue({ merged: true })
    const runtime = makeStoreRuntime()
    const { WORKSPACE_CLEANUP_METHODS } = await import('./workspace-cleanup')
    const dispatcher = new RpcDispatcher({ runtime, methods: WORKSPACE_CLEANUP_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('workspaceCleanup.dismiss', {
        dismissals: [
          { worktreeId: 'wt-1', dismissedAt: 1, fingerprint: 'fp', classifierVersion: 2 }
        ]
      })
    )

    expect(response).toMatchObject({ ok: true })
    expect(mergeWorkspaceCleanupDismissalsMock).toHaveBeenCalledWith(
      { a: {} },
      [{ worktreeId: 'wt-1', dismissedAt: 1, fingerprint: 'fp', classifierVersion: 2 }]
    )
    expect(runtime.updateUIState).toHaveBeenCalledWith({
      workspaceCleanup: { dismissals: { merged: true } }
    })
  })

  it('clears dismissals', async () => {
    const runtime = makeStoreRuntime()
    const { WORKSPACE_CLEANUP_METHODS } = await import('./workspace-cleanup')
    const dispatcher = new RpcDispatcher({ runtime, methods: WORKSPACE_CLEANUP_METHODS })

    await dispatcher.dispatch(makeRequest('workspaceCleanup.clearDismissals'))

    expect(runtime.updateUIState).toHaveBeenCalledWith({ workspaceCleanup: { dismissals: {} } })
  })

  it('checks for killable local processes via the shared helper', async () => {
    hasKillableProcessesMock.mockResolvedValue(true)
    const runtime = makeStoreRuntime()
    const { WORKSPACE_CLEANUP_METHODS } = await import('./workspace-cleanup')
    const dispatcher = new RpcDispatcher({ runtime, methods: WORKSPACE_CLEANUP_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('workspaceCleanup.hasKillableLocalProcesses', { worktreeId: 'wt-1' })
    )

    expect(response).toMatchObject({ ok: true, result: { hasKillableProcesses: true } })
    expect(hasKillableProcessesMock).toHaveBeenCalledWith(
      { worktreeId: 'wt-1' },
      expect.objectContaining({ runtime, getLocalPtyProvider: expect.any(Function) })
    )
  })
})
