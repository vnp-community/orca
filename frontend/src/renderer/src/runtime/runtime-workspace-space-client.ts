import type { GlobalSettings } from '../../../shared/types'
import type { RuntimeRpcResponse } from '../../../shared/runtime-rpc-envelope'
import type {
  WorkspaceSpaceAnalyzeResult,
  WorkspaceSpaceScanProgress
} from '../../../shared/workspace-space-types'
import { callRuntimeRpc, getActiveRuntimeTarget, unwrapRuntimeRpcResult } from './runtime-rpc-client'

type WorkspaceSpaceSettings = Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>

type WorkspaceSpaceAnalyzeStreamEvent =
  | { type: 'progress'; progress: WorkspaceSpaceScanProgress }
  | { type: 'result'; result: WorkspaceSpaceAnalyzeResult }

export async function cancelWorkspaceSpaceScan(
  settings: WorkspaceSpaceSettings | null | undefined
): Promise<boolean> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.workspaceSpace.cancel()
  }
  return callRuntimeRpc<boolean>(target, 'workspaceSpace.cancel')
}

// Why: workspaceSpace.analyze is a bounded scan that streams progress and
// resolves with the final result, matching the local ipcMain handler's
// analyze()+onProgress() split. Local mode keeps calling the existing
// preload API unchanged; an environment target subscribes to the equivalent
// streaming RPC method (workspaceSpace.analyze wraps analyzeWorkspaceSpace,
// same as the ipcMain handler).
export async function analyzeWorkspaceSpace(
  settings: WorkspaceSpaceSettings | null | undefined,
  onProgress?: (progress: WorkspaceSpaceScanProgress) => void
): Promise<WorkspaceSpaceAnalyzeResult> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    const unsubscribe = onProgress ? window.api.workspaceSpace.onProgress(onProgress) : null
    try {
      return await window.api.workspaceSpace.analyze()
    } finally {
      unsubscribe?.()
    }
  }
  return new Promise<WorkspaceSpaceAnalyzeResult>((resolve, reject) => {
    let settled = false
    let handle: { unsubscribe: () => void } | null = null
    const settle = (run: () => void): void => {
      if (settled) {
        return
      }
      settled = true
      run()
      handle?.unsubscribe()
    }
    window.api.runtimeEnvironments
      .subscribe(
        { selector: target.environmentId, method: 'workspaceSpace.analyze' },
        {
          onResponse: (response) => {
            try {
              const event = unwrapRuntimeRpcResult<WorkspaceSpaceAnalyzeStreamEvent>(
                response as RuntimeRpcResponse<WorkspaceSpaceAnalyzeStreamEvent>
              )
              if (event.type === 'progress') {
                onProgress?.(event.progress)
              } else if (event.type === 'result') {
                settle(() => resolve(event.result))
              }
            } catch (error) {
              settle(() => reject(error))
            }
          },
          onError: (error) => settle(() => reject(new Error(error.message))),
          onClose: () => settle(() => reject(new Error('Workspace space scan connection closed')))
        }
      )
      .then((subscription) => {
        handle = subscription
        if (settled) {
          subscription.unsubscribe()
        }
      })
      .catch((error) => settle(() => reject(error)))
  })
}
