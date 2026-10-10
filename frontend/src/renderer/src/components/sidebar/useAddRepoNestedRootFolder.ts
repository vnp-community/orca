import { useCallback, type MutableRefObject } from 'react'
import { toast } from 'sonner'
import { track } from '@/lib/telemetry'
import { useAppStore } from '@/store'
import {
  buildNestedRepoImportActionTelemetry,
  type NestedRepoTelemetryRuntimeKind
} from '../../../../shared/nested-repo-telemetry'
import type { NestedRepoScanResult } from '../../../../shared/types'

/**
 * "Open as folder" on the nested-repo review step: adds the scanned root itself as a non-git folder
 * on the host the dialog was scanning (SSH confirmation dialog, runtime environment or dev server).
 */
export function useAddRepoNestedRootFolder({
  nestedImportGenRef,
  nestedScan,
  nestedAttemptId,
  nestedRuntimeKind,
  nestedConnectionId,
  nestedSelectedPathCount,
  activeRuntimeEnvironmentId,
  devServerId,
  getNestedRepoRuntimeKind,
  setIsAdding
}: {
  // Shared with the import handler so either one supersedes the other's in-flight work.
  nestedImportGenRef: MutableRefObject<number>
  nestedScan: NestedRepoScanResult | null
  nestedAttemptId: string | null
  nestedRuntimeKind: NestedRepoTelemetryRuntimeKind | null
  nestedConnectionId: string | null
  nestedSelectedPathCount: number
  activeRuntimeEnvironmentId: string | null | undefined
  devServerId?: string | null
  getNestedRepoRuntimeKind: (connectionId: string | null) => NestedRepoTelemetryRuntimeKind
  setIsAdding: (isAdding: boolean) => void
}): () => Promise<void> {
  return useCallback(async (): Promise<void> => {
    if (!nestedScan) {
      return
    }
    const gen = ++nestedImportGenRef.current
    const path = nestedScan.selectedPath
    if (nestedAttemptId) {
      track(
        'add_repo_nested_import_action',
        buildNestedRepoImportActionTelemetry({
          attemptId: nestedAttemptId,
          surface: 'sidebar',
          runtimeKind: nestedRuntimeKind ?? getNestedRepoRuntimeKind(nestedConnectionId),
          action: 'open_as_folder',
          foundCount: nestedScan.repos.length,
          selectedCount: nestedSelectedPathCount
        })
      )
    }
    setIsAdding(true)
    try {
      const state = useAppStore.getState()
      if (nestedConnectionId) {
        state.closeModal()
        state.openModal('confirm-non-git-folder', {
          folderPath: path,
          connectionId: nestedConnectionId
        })
        return
      }
      const repo = await state.addNonGitFolder(path, {
        runtimeEnvironmentId: activeRuntimeEnvironmentId?.trim() || null,
        ...(devServerId ? { devServerId } : {})
      })
      if (gen !== nestedImportGenRef.current) {
        return
      }
      if (repo) {
        useAppStore.getState().closeModal()
      }
    } catch (err) {
      if (gen === nestedImportGenRef.current) {
        toast.error(err instanceof Error ? err.message : String(err))
      }
    } finally {
      if (gen === nestedImportGenRef.current) {
        setIsAdding(false)
      }
    }
  }, [
    activeRuntimeEnvironmentId,
    devServerId,
    getNestedRepoRuntimeKind,
    nestedAttemptId,
    nestedConnectionId,
    nestedImportGenRef,
    nestedRuntimeKind,
    nestedScan,
    nestedSelectedPathCount,
    setIsAdding
  ])
}
