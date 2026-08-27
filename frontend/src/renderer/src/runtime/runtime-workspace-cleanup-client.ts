import type { GlobalSettings } from '../../../shared/types'
import type { RuntimeRpcResponse } from '../../../shared/runtime-rpc-envelope'
import type {
  WorkspaceCleanupDismissArgs,
  WorkspaceCleanupLocalProcessArgs,
  WorkspaceCleanupLocalProcessResult,
  WorkspaceCleanupScanArgs,
  WorkspaceCleanupScanProgress,
  WorkspaceCleanupScanResult
} from '../../../shared/workspace-cleanup'
import { callRuntimeRpc, getActiveRuntimeTarget, unwrapRuntimeRpcResult } from './runtime-rpc-client'

type WorkspaceCleanupSettings = Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>

type WorkspaceCleanupScanStreamEvent =
  | { type: 'progress'; progress: WorkspaceCleanupScanProgress }
  | { type: 'result'; result: WorkspaceCleanupScanResult }

// Why: workspaceCleanup.scan streams progress and resolves with the final
// result, matching the local preload's scan(args, onProgress) split. Local
// mode keeps calling the existing preload API unchanged. Not `async` — see
// runtime-localhost-worktree-labels-client.ts for why.
export function scanWorkspaceCleanup(
  settings: WorkspaceCleanupSettings | null | undefined,
  args?: WorkspaceCleanupScanArgs,
  onProgress?: (progress: WorkspaceCleanupScanProgress) => void
): Promise<WorkspaceCleanupScanResult> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    // Why: preload's scan() branches its ipcRenderer.invoke call shape on
    // whether onProgress is provided — always passing an explicit `undefined`
    // second argument would change call-site call signatures callers assert on.
    return onProgress
      ? window.api.workspaceCleanup.scan(args, onProgress)
      : window.api.workspaceCleanup.scan(args)
  }
  return new Promise<WorkspaceCleanupScanResult>((resolve, reject) => {
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
        { selector: target.environmentId, method: 'workspaceCleanup.scan', params: args },
        {
          onResponse: (response) => {
            try {
              const event = unwrapRuntimeRpcResult<WorkspaceCleanupScanStreamEvent>(
                response as RuntimeRpcResponse<WorkspaceCleanupScanStreamEvent>
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
          onClose: () => settle(() => reject(new Error('Workspace cleanup scan connection closed')))
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

export function dismissWorkspaceCleanupCandidates(
  settings: WorkspaceCleanupSettings | null | undefined,
  args: WorkspaceCleanupDismissArgs
): Promise<void> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.workspaceCleanup.dismiss(args)
  }
  return callRuntimeRpc<void>(target, 'workspaceCleanup.dismiss', args)
}

export function clearWorkspaceCleanupDismissals(
  settings: WorkspaceCleanupSettings | null | undefined
): Promise<void> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.workspaceCleanup.clearDismissals()
  }
  return callRuntimeRpc<void>(target, 'workspaceCleanup.clearDismissals')
}

export function hasKillableWorkspaceCleanupLocalProcesses(
  settings: WorkspaceCleanupSettings | null | undefined,
  args: WorkspaceCleanupLocalProcessArgs
): Promise<WorkspaceCleanupLocalProcessResult> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.workspaceCleanup.hasKillableLocalProcesses(args)
  }
  return callRuntimeRpc<WorkspaceCleanupLocalProcessResult>(
    target,
    'workspaceCleanup.hasKillableLocalProcesses',
    args
  )
}
