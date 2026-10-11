import { useCallback, useRef, useState, type Dispatch, type SetStateAction } from 'react'
import { track } from '@/lib/telemetry'
import { useAppStore } from '@/store'
import { markOnboardingProjectAdded } from '@/lib/onboarding-project-checklist'
import { isGitRepoKind } from '../../../../shared/repo-kind'
import {
  buildNestedRepoScanTelemetry,
  createNestedRepoTelemetryAttemptId,
  type NestedRepoTelemetryRuntimeKind
} from '../../../../shared/nested-repo-telemetry'
import type { AddRepoExistingWorkspaceSource } from '../../../../shared/telemetry-events'
import type { NestedRepoScanResult, Repo } from '../../../../shared/types'
import { deriveProjectNameFromPath } from '@/lib/own-project-registration'
import { createNestedRepoScanId } from './add-repo-dialog-types'
import type { RegisteredRepoMatch } from './add-repo-duplicate-lookup'

type ShowNestedRepoReview = (args: {
  scan: NestedRepoScanResult
  selectedPath: string
  connectionId: string | null
  attemptId: string
  runtimeKind: NestedRepoTelemetryRuntimeKind
  inProgress: boolean
  scanId: string | null
}) => void

export function useAddRepoServerPathFlow({
  devServerId,
  addRepoPath,
  closeModal,
  fetchWorktrees,
  getNestedRepoRuntimeKind,
  scanNestedRepos,
  setActiveNestedScanId,
  setNestedScanInProgress,
  showNestedRepoReview,
  onGitRepoReady,
  setAddProjectBusyLabel,
  findRegisteredRepo,
  onOpenRegisteredRepo
}: {
  // Host picked in the Add Project dialog; it must reach repo.add, not only the folder browser.
  devServerId?: string | null
  addRepoPath: (
    path: string,
    kind?: 'git' | 'folder',
    options?: { devServerId?: string | null; ownProject?: { name: string } }
  ) => Promise<Repo | null>
  // Blocks adding a folder that is already a repo on this host (SOL-FE-E2E-005).
  findRegisteredRepo?: (args: {
    devServerId: string
    path: string
  }) => Promise<RegisteredRepoMatch | null>
  onOpenRegisteredRepo?: (match: RegisteredRepoMatch) => void | Promise<void>
  closeModal: () => void
  fetchWorktrees: (repoId: string, options?: { requireAuthoritative?: boolean }) => Promise<unknown>
  getNestedRepoRuntimeKind: (connectionId: string | null) => NestedRepoTelemetryRuntimeKind
  scanNestedRepos: (
    path: string,
    connectionId?: string,
    controls?: {
      scanId?: string
      onProgress?: (scan: NestedRepoScanResult) => void
      devServerId?: string | null
    }
  ) => Promise<NestedRepoScanResult | null>
  setActiveNestedScanId: (scanId: string | null) => void
  setNestedScanInProgress: (inProgress: boolean) => void
  showNestedRepoReview: ShowNestedRepoReview
  onGitRepoReady: (repoId: string, source: AddRepoExistingWorkspaceSource) => Promise<void>
  setAddProjectBusyLabel: (label: string | null) => void
}): {
  serverPath: string
  isAddingServerPath: boolean
  setServerPath: Dispatch<SetStateAction<string>>
  projectName: string
  setProjectName: (name: string) => void
  duplicate: RegisteredRepoMatch | null
  openDuplicate: () => void
  resetServerPathFlow: () => void
  handleAddServerPath: (kind: 'git' | 'folder') => Promise<void>
} {
  const [serverPath, setServerPathState] = useState('')
  const [isAddingServerPath, setIsAddingServerPath] = useState(false)
  const [editedProjectName, setEditedProjectName] = useState<string | null>(null)
  const [duplicate, setDuplicate] = useState<RegisteredRepoMatch | null>(null)
  const serverAddGenRef = useRef(0)

  // Why: a new path is a new question, so a previous "already added" answer no longer applies.
  const setServerPath: Dispatch<SetStateAction<string>> = useCallback((value) => {
    setServerPathState(value)
    setDuplicate(null)
  }, [])

  // Why derived, not stored: the name follows the folder until the user types their own.
  const projectName = editedProjectName ?? deriveProjectNameFromPath(serverPath, '')

  const resetServerPathFlow = useCallback((): void => {
    serverAddGenRef.current++
    setServerPathState('')
    setEditedProjectName(null)
    setDuplicate(null)
    setIsAddingServerPath(false)
  }, [])

  const openDuplicate = useCallback((): void => {
    if (duplicate) {
      void onOpenRegisteredRepo?.(duplicate)
    }
  }, [duplicate, onOpenRegisteredRepo])

  const handleAddServerPath = useCallback(
    async (kind: 'git' | 'folder'): Promise<void> => {
      const path = serverPath.trim()
      if (!path) {
        return
      }
      const gen = ++serverAddGenRef.current
      setIsAddingServerPath(true)
      setAddProjectBusyLabel(kind === 'git' ? 'Scanning for repositories...' : 'Opening folder...')
      try {
        if (devServerId && findRegisteredRepo) {
          const match = await findRegisteredRepo({ devServerId, path })
          if (gen !== serverAddGenRef.current) {
            return
          }
          if (match) {
            setDuplicate(match)
            return
          }
        }
        if (kind === 'git') {
          const attemptId = createNestedRepoTelemetryAttemptId()
          const runtimeKind = getNestedRepoRuntimeKind(null)
          const supportsStreamingScan = runtimeKind !== 'runtime'
          const scanId = supportsStreamingScan ? createNestedRepoScanId() : null
          if (scanId) {
            setActiveNestedScanId(scanId)
            setNestedScanInProgress(true)
          }
          const scan = await scanNestedRepos(
            path,
            undefined,
            scanId || devServerId
              ? {
                  ...(devServerId ? { devServerId } : {}),
                  ...(scanId ? { scanId } : {}),
                  onProgress: (progressScan) => {
                    if (
                      gen !== serverAddGenRef.current ||
                      progressScan.selectedPathKind !== 'non_git_folder' ||
                      progressScan.repos.length === 0
                    ) {
                      return
                    }
                    showNestedRepoReview({
                      scan: progressScan,
                      selectedPath: path,
                      connectionId: null,
                      attemptId,
                      runtimeKind,
                      inProgress: true,
                      scanId
                    })
                  }
                }
              : undefined
          )
          if (gen !== serverAddGenRef.current) {
            return
          }
          setNestedScanInProgress(false)
          setActiveNestedScanId(null)
          track(
            'add_repo_nested_scan_result',
            buildNestedRepoScanTelemetry({
              attemptId,
              surface: 'sidebar',
              runtimeKind,
              scan
            })
          )
          if (scan?.selectedPathKind === 'non_git_folder' && scan.repos.length > 0) {
            showNestedRepoReview({
              scan,
              selectedPath: path,
              connectionId: null,
              attemptId,
              runtimeKind,
              inProgress: false,
              scanId
            })
            return
          }
        }
        setAddProjectBusyLabel(kind === 'git' ? 'Opening project...' : 'Opening folder...')
        // Why ownProject: each added repo gets its own project; Requests run on a project's first repo.
        const repo = devServerId
          ? await addRepoPath(path, kind, {
              devServerId,
              ownProject: { name: projectName.trim() }
            })
          : await addRepoPath(path, kind)
        if (gen !== serverAddGenRef.current) {
          return
        }
        if (repo && isGitRepoKind(repo)) {
          // Why: once the repo exists, a transient non-authoritative refresh
          // should fall through to project reveal instead of leaving the add flow open.
          await fetchWorktrees(repo.id, { requireAuthoritative: true })
          if (gen !== serverAddGenRef.current) {
            return
          }
          await onGitRepoReady(repo.id, 'runtime_server_path')
        } else if (repo) {
          // Why: folder repos skip the Git default-checkout handoff; their synthetic
          // root workspace is opened by the folder add flow.
          await markOnboardingProjectAdded('addedFolder', useAppStore.getState().settings)
          closeModal()
        }
      } finally {
        if (gen === serverAddGenRef.current) {
          setNestedScanInProgress(false)
          setActiveNestedScanId(null)
          setIsAddingServerPath(false)
          setAddProjectBusyLabel(null)
        }
      }
    },
    [
      addRepoPath,
      closeModal,
      devServerId,
      fetchWorktrees,
      findRegisteredRepo,
      getNestedRepoRuntimeKind,
      onGitRepoReady,
      projectName,
      scanNestedRepos,
      serverPath,
      setActiveNestedScanId,
      setAddProjectBusyLabel,
      setNestedScanInProgress,
      showNestedRepoReview
    ]
  )

  return {
    serverPath,
    isAddingServerPath,
    setServerPath,
    projectName,
    setProjectName: setEditedProjectName,
    duplicate,
    openDuplicate,
    resetServerPathFlow,
    handleAddServerPath
  }
}
