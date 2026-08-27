import { defineMethod, defineStreamingMethod, type RpcAnyMethod } from '../core'
import type {
  WorkspaceSpaceAnalyzeResult,
  WorkspaceSpaceScanProgress
} from '../../../../shared/workspace-space-types'
import {
  analyzeWorkspaceSpace,
  WorkspaceSpaceScanCancelledError
} from '../../../workspace-space-analysis'
import { getActiveOnboardingStore } from '../../../ipc/onboarding-ipc'

const PROGRESS_EMIT_INTERVAL_MS = 100

type InFlightWorkspaceSpaceScan = {
  scanId: string
  controller: AbortController
  progress: WorkspaceSpaceScanProgress
  promise: Promise<WorkspaceSpaceAnalyzeResult>
}

// Why: mirrors workspaceSpace:analyze's ipcMain-handler in-flight dedup (see
// desktop/src/main/ipc/workspace-space.ts) so duplicate RPC scan requests
// share the same disk-traversal IO instead of racing. This state is a
// separate instance from desktop's own handler state — an RPC-originated scan
// only dedups against other callers of this same backend RPC method.
let inFlightScan: InFlightWorkspaceSpaceScan | null = null

// Why: only the caller who started a scan has their `emit` wired into
// analyzeWorkspaceSpace's onProgress closure. Callers who join an in-flight
// scan still need progress, so every active workspaceSpace.analyze call
// registers its emit here and progress fans out to all of them.
const progressEmitters = new Set<(message: unknown) => void>()

export const WORKSPACE_SPACE_METHODS: readonly RpcAnyMethod[] = [
  defineStreamingMethod({
    name: 'workspaceSpace.analyze',
    params: null,
    handler: async (_params, _ctx, emit) => {
      progressEmitters.add(emit)
      try {
        if (!inFlightScan) {
          // Why: desktop's RPC method reads through RuntimeStore/getRuntimeStoreForRpc,
          // which backend's OrcaRuntimeService does not expose. Backend has no
          // separate onboarding.ts either — getActiveOnboardingStore() (see
          // ipc/onboarding-ipc.ts) is the established backend equivalent, already
          // used by workspaceCleanup's ported dismiss/clearDismissals methods.
          const store = getActiveOnboardingStore() ?? null
          if (!store) {
            emit({ type: 'result', result: { ok: false, cancelled: true } })
            return
          }
          const controller = new AbortController()
          const scanId = `${Date.now()}-${Math.random().toString(36).slice(2)}`
          let latestProgress: WorkspaceSpaceScanProgress = {
            scanId,
            state: 'running',
            startedAt: Date.now(),
            updatedAt: Date.now(),
            totalRepoCount: 0,
            scannedRepoCount: 0,
            totalWorktreeCount: 0,
            scannedWorktreeCount: 0,
            currentRepoDisplayName: null,
            currentWorktreeDisplayName: null
          }
          let lastProgressSentAt = 0
          const sendProgress = (progress: WorkspaceSpaceScanProgress): void => {
            // Why: large fleets can report one progress event per worktree; keep
            // callers responsive without an emit-per-worktree flood.
            const now = Date.now()
            const isFirstProgress = lastProgressSentAt === 0
            const isTerminalProgress =
              progress.state !== 'running' ||
              (progress.totalWorktreeCount > 0 &&
                progress.scannedWorktreeCount >= progress.totalWorktreeCount)
            if (
              !isFirstProgress &&
              !isTerminalProgress &&
              now - lastProgressSentAt < PROGRESS_EMIT_INTERVAL_MS
            ) {
              return
            }
            lastProgressSentAt = now
            for (const progressEmit of progressEmitters) {
              progressEmit({ type: 'progress', progress })
            }
          }
          const scan: InFlightWorkspaceSpaceScan = {
            scanId,
            controller,
            progress: latestProgress,
            promise: Promise.resolve(null as never)
          }
          inFlightScan = scan
          scan.promise = analyzeWorkspaceSpace(store, {
            scanId,
            signal: controller.signal,
            onProgress: (progress) => {
              latestProgress = progress
              scan.progress = progress
              sendProgress(progress)
            }
          })
            .then((analysis): WorkspaceSpaceAnalyzeResult => ({ ok: true, analysis }))
            .catch((error: unknown): WorkspaceSpaceAnalyzeResult => {
              if (error instanceof WorkspaceSpaceScanCancelledError) {
                return { ok: false, cancelled: true }
              }
              throw error
            })
            .finally(() => {
              inFlightScan = null
            })
        }
        const result = await inFlightScan.promise
        emit({ type: 'result', result })
      } finally {
        progressEmitters.delete(emit)
      }
    }
  }),
  defineMethod({
    name: 'workspaceSpace.cancel',
    params: null,
    handler: async (): Promise<boolean> => {
      if (!inFlightScan || inFlightScan.controller.signal.aborted) {
        return false
      }
      inFlightScan.controller.abort()
      inFlightScan.progress = {
        ...inFlightScan.progress,
        state: 'cancelling',
        updatedAt: Date.now()
      }
      return true
    }
  })
]
