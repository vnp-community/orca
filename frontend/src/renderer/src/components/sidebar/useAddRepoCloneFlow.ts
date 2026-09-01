import { useCallback, useEffect, useRef, useState, type Dispatch, type SetStateAction } from 'react'
import { toast } from 'sonner'
import { useAppStore } from '@/store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import {
  getOrCreateDefaultProject,
  mergeRepoViewIntoRepo,
  repoDisplayNameFromUrl,
  type RemoteRepoView
} from '@/store/slices/repos'
import type { AddRepoExistingWorkspaceSource } from '../../../../shared/telemetry-events'
import type { Repo } from '../../../../shared/types'
import { getCloneDestinationAutoFill } from './clone-defaults'
import type { AddRepoDialogStep } from './add-repo-dialog-types'
import { translate } from '@/i18n/i18n'
import { extractIpcErrorMessage } from '@/lib/ipc-error'
import { upsertAddedRepoWithProjectHostSetup } from './add-repo-store-upsert'

export function useAddRepoCloneFlow({
  step,
  activeRuntimeEnvironmentId,
  sshTargetId,
  devServerId,
  workspaceDir,
  fetchWorktrees,
  onGitRepoReady
}: {
  step: AddRepoDialogStep
  activeRuntimeEnvironmentId: string | null | undefined
  sshTargetId?: string | null
  // The dev server picked in AddRepoDialog's own Host selector — see
  // useCreateRepo.ts's identical option for why this must NOT be read
  // from useAppStore's global activeDevServerId (a different, unrelated
  // piece of state), the same live-verified GITGATEWAY_MISSING_DEV_SERVER_ID
  // gap this fixes for the clone flow too.
  devServerId?: string | null
  workspaceDir: string | null | undefined
  fetchWorktrees: (repoId: string, options?: { requireAuthoritative?: boolean }) => Promise<unknown>
  onGitRepoReady: (repoId: string, source: AddRepoExistingWorkspaceSource) => Promise<void>
}): {
  cloneUrl: string
  cloneDestination: string
  cloneError: string | null
  cloneProgress: { phase: string; percent: number } | null
  isCloning: boolean
  setCloneUrl: Dispatch<SetStateAction<string>>
  setCloneDestination: Dispatch<SetStateAction<string>>
  setCloneError: Dispatch<SetStateAction<string | null>>
  resetCloneFlow: () => void
  handlePickDestination: () => Promise<void>
  handleClone: () => Promise<void>
} {
  const [cloneUrl, setCloneUrl] = useState('')
  const [cloneDestination, setCloneDestination] = useState('')
  const [isCloning, setIsCloning] = useState(false)
  const [cloneError, setCloneError] = useState<string | null>(null)
  const [cloneProgress, setCloneProgress] = useState<{ phase: string; percent: number } | null>(
    null
  )
  const hostToken = `${activeRuntimeEnvironmentId?.trim() ?? ''}:${sshTargetId?.trim() ?? ''}`
  const hostTokenRef = useRef(hostToken)
  hostTokenRef.current = hostToken
  // Why: monotonic ID so stale clone callbacks can detect they were superseded.
  const cloneGenRef = useRef(0)
  // Why: track whether we've already auto-filled for this entry into the clone step,
  // so a late settings hydration still gets a chance to set the default.
  const cloneStepAutoFilledRef = useRef(false)

  useEffect(() => {
    if (!isCloning) {
      return
    }
    return window.api.repos.onCloneProgress(setCloneProgress)
  }, [isCloning])

  const cloneDestinationAutoFill = getCloneDestinationAutoFill({
    step,
    cloneDestination,
    activeRuntimeEnvironmentId,
    sshTargetId,
    workspaceDir,
    cloneStepAutoFilled: cloneStepAutoFilledRef.current
  })
  if (step !== 'clone') {
    cloneStepAutoFilledRef.current = false
  } else if (cloneDestinationAutoFill) {
    // Why: late settings hydration should still seed the local clone path,
    // but runtime/server clone flows must keep their destination user-entered.
    cloneStepAutoFilledRef.current = true
    setCloneDestination(cloneDestinationAutoFill.destination)
  }

  const resetCloneFlow = useCallback((): void => {
    cloneGenRef.current++
    setCloneUrl('')
    setCloneDestination('')
    setIsCloning(false)
    setCloneError(null)
    setCloneProgress(null)
  }, [])

  const handlePickDestination = useCallback(async (): Promise<void> => {
    if (activeRuntimeEnvironmentId?.trim() || sshTargetId?.trim()) {
      // Why: the native folder picker returns a client-local path. Runtime
      // and SSH clone destinations must be typed as paths on that host.
      toast.error(
        translate(
          'auto.components.sidebar.useAddRepoCloneFlow.0dc4d1b657',
          'Enter a host path for the clone destination.'
        )
      )
      return
    }
    const gen = cloneGenRef.current
    const dir = await window.api.repos.pickDirectory()
    if (dir && gen === cloneGenRef.current) {
      setCloneDestination(dir)
      setCloneError(null)
    }
  }, [activeRuntimeEnvironmentId, sshTargetId])

  const handleClone = useCallback(async (): Promise<void> => {
    const trimmedUrl = cloneUrl.trim()
    if (!trimmedUrl || !cloneDestination.trim()) {
      return
    }
    const requestHostToken = hostTokenRef.current
    const gen = ++cloneGenRef.current
    setIsCloning(true)
    setCloneError(null)
    setCloneProgress(null)
    try {
      // Why activeRuntimeEnvironmentId is nulled out UNLESS a dev server was
      // picked: see useCreateRepo.ts's identical doc comment — a web-mode
      // Dev Server host selection needs the real active environment (web
      // mode's persisted "session-auth" one) so target.kind becomes
      // 'environment'; nulling it out unconditionally made devServerId-based
      // clones always resolve to getOrCreateDefaultProject's unsupported
      // 'local' case instead.
      const target = activeRuntimeEnvironmentId?.trim()
        ? { kind: 'environment' as const, environmentId: activeRuntimeEnvironmentId.trim() }
        : getActiveRuntimeTarget({
            ...useAppStore.getState().settings,
            activeRuntimeEnvironmentId: devServerId
              ? useAppStore.getState().settings?.activeRuntimeEnvironmentId
              : null
          })
      const repo = sshTargetId?.trim()
        ? await window.api.repos.cloneRemote({
            connectionId: sshTargetId.trim(),
            url: trimmedUrl,
            destination: cloneDestination.trim()
          })
        : devServerId || target.kind === 'environment'
          ? await (async (): Promise<Repo> => {
              // Why a two-step clone+add, not one repo.clone call: the Go
              // handler (channels_repo_ssh_status_workspace.go) decodes
              // {devServerId, url, destPath} and only relays to
              // git-gateway-service's Clone — it clones the repo onto disk
              // but never registers a project.repos row. repo.add is the
              // only call that does that. devServerId comes from this
              // hook's own option (the dialog's Host selector), not
              // useAppStore's global activeDevServerId — see this hook's
              // param doc comment.
              const destPath = cloneDestination.trim()
              await callRuntimeRpc<{ worktreePath: string; defaultBranch: string }>(
                target,
                'repo.clone',
                { devServerId: devServerId ?? '', url: trimmedUrl, destPath },
                { timeoutMs: 10 * 60_000 }
              )
              // getOrCreateDefaultProject needs a real environment target —
              // only reachable if devServerId was set with no active
              // runtime environment resolved at all.
              if (target.kind !== 'environment') {
                throw new Error(
                  'No active runtime environment to register the cloned repo with a project.'
                )
              }
              const projectId = await getOrCreateDefaultProject(target)
              const view = await callRuntimeRpc<RemoteRepoView>(
                target,
                'repo.add',
                { projectId, url: destPath, displayName: repoDisplayNameFromUrl(trimmedUrl) },
                { timeoutMs: 15_000 }
              )
              return mergeRepoViewIntoRepo(view)
            })()
          : ((await window.api.repos.clone({
              url: trimmedUrl,
              destination: cloneDestination.trim()
            })) as Repo)
      if (gen !== cloneGenRef.current || requestHostToken !== hostTokenRef.current) {
        return
      }
      toast.success(
        translate('auto.components.sidebar.useAddRepoCloneFlow.4d0013cc93', 'Repository cloned'),
        { description: repo.displayName }
      )
      upsertAddedRepoWithProjectHostSetup(repo)
      // Why: once the repo exists, a transient non-authoritative refresh
      // should fall through to project reveal instead of leaving the add flow open.
      await fetchWorktrees(repo.id, { requireAuthoritative: true })
      if (gen !== cloneGenRef.current || requestHostToken !== hostTokenRef.current) {
        return
      }
      await onGitRepoReady(repo.id, 'clone_url')
    } catch (err) {
      if (gen !== cloneGenRef.current || requestHostToken !== hostTokenRef.current) {
        return
      }
      const message = extractIpcErrorMessage(err, String(err))
      setCloneError(message)
    } finally {
      if (gen === cloneGenRef.current && requestHostToken === hostTokenRef.current) {
        setIsCloning(false)
      }
    }
  }, [
    activeRuntimeEnvironmentId,
    cloneUrl,
    cloneDestination,
    fetchWorktrees,
    onGitRepoReady,
    sshTargetId,
    devServerId
  ])

  return {
    cloneUrl,
    cloneDestination,
    cloneError,
    cloneProgress,
    isCloning,
    setCloneUrl,
    setCloneDestination,
    setCloneError,
    resetCloneFlow,
    handlePickDestination,
    handleClone
  }
}
