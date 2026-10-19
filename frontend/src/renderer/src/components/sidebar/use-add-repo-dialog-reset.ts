import { useCallback } from 'react'
import type { AddRepoDialogStep } from './add-repo-dialog-types'
import { useAddRepoHostChangeReset } from './use-add-repo-host-change-reset'

/**
 * Add Project resets: everything on close/back, and only the host-scoped typed state when the
 * chosen host changes (typed paths belong to one host's filesystem).
 */
export function useAddRepoDialogReset({
  isOpen,
  selectedHostId,
  setStep,
  setIsAdding,
  setAddProjectBusyLabel,
  resetLocalFolderFlow,
  resetServerPathFlow,
  resetCloneFlow,
  resetNestedImportFlow,
  resetNestedRepoReviewState,
  resetCreateDefaultState,
  resetCreateState,
  resetRemoteState
}: {
  isOpen: boolean
  selectedHostId: string
  setStep: (step: AddRepoDialogStep) => void
  setIsAdding: (isAdding: boolean) => void
  setAddProjectBusyLabel: (label: string | null) => void
  resetLocalFolderFlow: () => void
  resetServerPathFlow: () => void
  resetCloneFlow: () => void
  resetNestedImportFlow: () => void
  resetNestedRepoReviewState: () => void
  resetCreateDefaultState: () => void
  resetCreateState: () => void
  resetRemoteState: () => void
}): { resetState: () => void } {
  const resetState = useCallback(() => {
    // Why: kill the git clone process if one is running, so backing out
    // or closing the dialog doesn't leave a clone running on disk.
    void window.api.repos.cloneAbort()
    resetLocalFolderFlow()
    setStep('add')
    setIsAdding(false)
    setAddProjectBusyLabel(null)
    resetServerPathFlow()
    resetCloneFlow()
    resetNestedImportFlow()
    resetNestedRepoReviewState()
    resetCreateDefaultState()
    resetCreateState()
    resetRemoteState()
  }, [
    resetCloneFlow,
    resetLocalFolderFlow,
    resetNestedRepoReviewState,
    resetCreateDefaultState,
    resetServerPathFlow,
    resetNestedImportFlow,
    resetRemoteState,
    resetCreateState,
    setAddProjectBusyLabel,
    setIsAdding,
    setStep
  ])

  const resetHostScopedState = useCallback(() => {
    setIsAdding(false)
    setAddProjectBusyLabel(null)
    resetServerPathFlow()
    resetCloneFlow()
    resetCreateDefaultState()
    resetCreateState()
    resetRemoteState()
  }, [
    resetCloneFlow,
    resetCreateDefaultState,
    resetCreateState,
    resetRemoteState,
    resetServerPathFlow,
    setAddProjectBusyLabel,
    setIsAdding
  ])

  useAddRepoHostChangeReset({
    isOpen,
    selectedHostId,
    onResetClosed: resetState,
    onResetHostScopedState: resetHostScopedState
  })

  return { resetState }
}
