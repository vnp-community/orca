import { z } from 'zod'
import { defineMethod, defineStreamingMethod, type RpcAnyMethod } from '../core'
import { OptionalString, requiredNumber, requiredString } from '../schemas'
import type { Store } from '../../../persistence'
import type {
  WorkspaceCleanupLocalProcessResult,
  WorkspaceCleanupScanArgs
} from '../../../../shared/workspace-cleanup'
import {
  hasKillableProcesses,
  mergeWorkspaceCleanupDismissals,
  scanWorkspaceCleanup
} from '../../../ipc/workspace-cleanup'

const WorkspaceCleanupScanParams = z.object({
  worktreeId: OptionalString,
  skipGitWorktreeIds: z.array(z.string()).optional(),
  scanId: OptionalString
})

const WorkspaceCleanupDismissalSchema = z.object({
  worktreeId: requiredString('Missing worktreeId'),
  dismissedAt: requiredNumber('Missing dismissedAt'),
  fingerprint: requiredString('Missing fingerprint'),
  classifierVersion: requiredNumber('Missing classifierVersion')
})

const WorkspaceCleanupDismissParams = z.object({
  dismissals: z.array(WorkspaceCleanupDismissalSchema).optional()
})

const WorkspaceCleanupLocalProcessParams = z.object({
  worktreeId: requiredString('Missing worktreeId'),
  connectionId: OptionalString.nullable().optional(),
  worktreePath: OptionalString
})

export const WORKSPACE_CLEANUP_METHODS: readonly RpcAnyMethod[] = [
  defineStreamingMethod({
    name: 'workspaceCleanup.scan',
    params: WorkspaceCleanupScanParams,
    handler: async (params, { runtime }, emit) => {
      // Why: RuntimeStore only narrows the store's static type — the instance
      // backing it is always the real Store (see getRuntimeStoreForRpc).
      const store = runtime.getRuntimeStoreForRpc() as unknown as Store | null
      if (!store) {
        emit({ type: 'result', result: { scannedAt: Date.now(), candidates: [], errors: [] } })
        return
      }
      // Why: always emit progress for RPC callers, even when they omit a
      // scanId — scanWorkspaceCleanup only reports progress when a scanId is set.
      const args: WorkspaceCleanupScanArgs = {
        ...params,
        scanId: params.scanId ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
      }
      const result = await scanWorkspaceCleanup(store, args, {
        onProgress: (progress) => emit({ type: 'progress', progress })
      })
      emit({ type: 'result', result })
    }
  }),
  defineMethod({
    name: 'workspaceCleanup.dismiss',
    params: WorkspaceCleanupDismissParams,
    handler: async (params, { runtime }): Promise<void> => {
      const current = runtime.getUIState().workspaceCleanup?.dismissals ?? {}
      runtime.updateUIState({
        workspaceCleanup: {
          dismissals: mergeWorkspaceCleanupDismissals(current, params.dismissals)
        }
      })
    }
  }),
  defineMethod({
    name: 'workspaceCleanup.clearDismissals',
    params: null,
    handler: async (_params, { runtime }): Promise<void> => {
      runtime.updateUIState({ workspaceCleanup: { dismissals: {} } })
    }
  }),
  defineMethod({
    name: 'workspaceCleanup.hasKillableLocalProcesses',
    params: WorkspaceCleanupLocalProcessParams,
    handler: async (params, { runtime }): Promise<WorkspaceCleanupLocalProcessResult> => ({
      hasKillableProcesses: await hasKillableProcesses(params, {
        runtime,
        getLocalPtyProvider: () => runtime.getLocalProvider()
      })
    })
  })
]
