// Create-project flow hook for AddRepoDialog (orca#763), split from
// AddRepoCreateStep so the create-state machine stays scoped and testable.
import { useCallback, useRef, useState } from 'react'
import { toast } from 'sonner'
import { useAppStore } from '@/store'
import { useMountedRef } from '@/hooks/useMountedRef'
import { activateAndRevealWorktree } from '@/lib/worktree-activation'
import { markOnboardingProjectAdded } from '@/lib/onboarding-project-checklist'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import {
  getOrCreateDefaultProject,
  mergeRepoViewIntoRepo,
  type RemoteRepoView
} from '@/store/slices/repos'
import { isGitRepoKind } from '../../../../shared/repo-kind'
import type { Repo } from '../../../../shared/types'
import { translate } from '@/i18n/i18n'
import { extractIpcErrorMessage } from '@/lib/ipc-error'
import { upsertAddedRepoWithProjectHostSetup } from './add-repo-store-upsert'

export function useCreateRepo(
  fetchWorktrees: (
    repoId: string,
    options?: { requireAuthoritative?: boolean }
  ) => Promise<boolean>,
  closeModal: () => void,
  onGitRepoReady?: (repoId: string) => void | Promise<void>,
  options: {
    hostId?: string | null
    runtimeEnvironmentId?: string | null
    sshTargetId?: string | null
    // The dev server picked in AddRepoDialog's own Host selector — NOT
    // the same thing as useAppStore's global activeDevServerId (a
    // different, unrelated piece of state for the currently active
    // workspace). Live-verified gap this fixes: the dialog's Host
    // dropdown could show "test-01 Connected" while this hook still read
    // the global value (often empty), so repo.create's own
    // GITGATEWAY_MISSING_DEV_SERVER_ID validation rejected every create
    // even with a real, connected host selected.
    devServerId?: string | null
  } = {}
) {
  const [createName, setCreateName] = useState('')
  const [createParent, setCreateParent] = useState('')
  const [createError, setCreateError] = useState<string | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const mountedRef = useMountedRef()
  const hostToken = options.hostId ?? options.sshTargetId ?? ''
  const hostTokenRef = useRef(hostToken)
  hostTokenRef.current = hostToken

  // Why: monotonic ID so stale create callbacks can detect they were superseded
  // when the user clicks Back or closes the dialog mid-create. Mirrors the
  // cloneGenRef pattern in AddRepoDialog.
  const createGenRef = useRef(0)

  const resetCreateState = useCallback(() => {
    createGenRef.current++
    setCreateName('')
    setCreateParent('')
    setCreateError(null)
    setIsCreating(false)
  }, [])

  const handlePickParent = useCallback(async (): Promise<string | null> => {
    if (options.sshTargetId) {
      // Why: the native picker can only browse the client machine. SSH create
      // uses a host path typed by the user until remote folder picking exists.
      toast.error(
        translate(
          'auto.components.sidebar.AddRepoCreateStep.ssh_parent_manual',
          'Enter an SSH parent path.'
        )
      )
      return null
    }
    if (options.runtimeEnvironmentId?.trim()) {
      // Why: the native folder picker returns a client-local path. Runtime
      // project creation needs an explicit host parent path.
      toast.error(
        translate(
          'auto.components.sidebar.AddRepoCreateStep.875dda0995',
          'Enter a host parent path.'
        )
      )
      return null
    }
    const gen = createGenRef.current
    const dir = await window.api.repos.pickDirectory()
    if (dir && gen === createGenRef.current && mountedRef.current) {
      setCreateParent(dir)
      setCreateError(null)
      return dir
    }
    return null
  }, [mountedRef, options.runtimeEnvironmentId, options.sshTargetId])

  const handleCreate = useCallback(async () => {
    const name = createName.trim()
    const parentPath = createParent.trim()
    if (!name || !parentPath) {
      return
    }
    const requestHostToken = hostTokenRef.current
    const gen = ++createGenRef.current
    setIsCreating(true)
    setCreateError(null)
    try {
      // Why activeRuntimeEnvironmentId is nulled out UNLESS a dev server was
      // picked: normally this hook must not silently piggyback on whatever
      // runtime environment happens to be globally active elsewhere in the
      // app (SSH/explicit-environment creates use their own target). But a
      // devServer-kind Host selection (this dialog's own dropdown) needs the
      // REAL active environment (web mode's persisted "session-auth" one,
      // confirmed live) so target.kind becomes 'environment' — nulling it
      // out here made devServerId-based creates always resolve to
      // getOrCreateDefaultProject's unsupported 'local' case instead.
      const target = options.runtimeEnvironmentId?.trim()
        ? { kind: 'environment' as const, environmentId: options.runtimeEnvironmentId.trim() }
        : getActiveRuntimeTarget({
            ...useAppStore.getState().settings,
            activeRuntimeEnvironmentId: options.devServerId
              ? useAppStore.getState().settings?.activeRuntimeEnvironmentId
              : null
          })
      // Why: Create Project is intentionally Git-only; non-Git folders use the
      // existing add-folder flows instead of this path.
      const createKind = 'git' as const
      // Why options.devServerId, not just target.kind === 'environment': a
      // web-mode Dev Server host selection (this dialog's own Host
      // dropdown) parses to `kind: 'devServer'`, not `'runtime'` — so
      // options.runtimeEnvironmentId stays empty and target falls back to
      // getActiveRuntimeTarget's own resolution, which resolves 'local'
      // for a plain web session with no active runtime environment set.
      // Live-verified gap this fixes: with only `target.kind ===
      // 'environment'` as the gate, a devServer-kind selection fell
      // through to window.api.repos.create (the Electron-only local IPC
      // path) instead of the RPC branch below — callRuntimeRpc's own
      // 'local' branch (window.api.runtime.call) forwards arbitrary
      // methods+params correctly regardless of target.kind, confirmed via
      // a direct repo.create call through it.
      const result = options.sshTargetId
        ? await window.api.repos.createRemote({
            connectionId: options.sshTargetId,
            parentPath,
            name,
            kind: createKind
          })
        : options.devServerId || target.kind === 'environment'
          ? await (async (): Promise<{ repo: Repo } | { error: string }> => {
              // Why a two-step create+add, not one repo.create call: the Go
              // handler (channels_repo_ssh_status_workspace.go) decodes
              // {devServerId, destPath, defaultBranch} and only relays to
              // git-gateway-service's InitRepo — it creates the bare repo on
              // disk but never registers a project.repos row. repo.add is
              // the only call that does that (mirrors CreateProjectDialog.
              // tsx's own create-then-add sequence).
              const devServerId = options.devServerId ?? ''
              const destPath = `${parentPath.replace(/[/\\]+$/, '')}/${name}`
              try {
                await callRuntimeRpc<{ path: string; defaultBranch: string }>(
                  target,
                  'repo.create',
                  { devServerId, destPath, defaultBranch: '' },
                  { timeoutMs: 60_000 }
                )
              } catch (err) {
                return { error: err instanceof Error ? err.message : String(err) }
              }
              // getOrCreateDefaultProject needs a real environment target —
              // only reachable if devServerId was set with no active
              // runtime environment resolved at all, an edge case with no
              // sensible default project to resolve.
              if (target.kind !== 'environment') {
                return {
                  error:
                    'No active runtime environment to register the created repo with a project.'
                }
              }
              const projectId = await getOrCreateDefaultProject(target)
              try {
                const view = await callRuntimeRpc<RemoteRepoView>(
                  target,
                  'repo.add',
                  { projectId, url: destPath, displayName: name },
                  { timeoutMs: 15_000 }
                )
                return { repo: { ...mergeRepoViewIntoRepo(view), kind: createKind } }
              } catch (err) {
                return { error: err instanceof Error ? err.message : String(err) }
              }
            })()
          : await window.api.repos.create({
              parentPath,
              name,
              kind: createKind
            })
      // Why: if the user closed the dialog or clicked Back mid-create,
      // createGenRef was bumped by resetCreateState. Ignore stale results.
      if (
        gen !== createGenRef.current ||
        requestHostToken !== hostTokenRef.current ||
        !mountedRef.current
      ) {
        return
      }
      if ('error' in result) {
        setCreateError(result.error)
        return
      }
      const repo = result.repo
      const state = useAppStore.getState()
      const existingIdx = state.repos.findIndex((r) => r.id === repo.id)
      // Why: the IPC handler dedupes by path (see repos:create) and returns
      // the existing repo unchanged. If its ID is already in our store, the
      // handler took the dedup path — no new project was created, so don't
      // claim one was.
      const wasDeduped = existingIdx !== -1
      upsertAddedRepoWithProjectHostSetup(repo)
      if (wasDeduped) {
        toast.info(
          translate(
            'auto.components.sidebar.AddRepoCreateStep.2c12db1511',
            'Project already added'
          ),
          {
            description: repo.displayName
          }
        )
      } else {
        toast.success(
          translate('auto.components.sidebar.AddRepoCreateStep.5e97f0c4b9', 'Project created'),
          {
            description: repo.displayName
          }
        )
      }
      if (isGitRepoKind(repo)) {
        // Why: Git repos use the shared default-checkout completion path.
        // Why: if refresh is temporarily non-authoritative, the shared opener
        // still reveals the project so the user is not left in a completed add flow.
        await fetchWorktrees(repo.id, { requireAuthoritative: true })
        if (
          gen !== createGenRef.current ||
          requestHostToken !== hostTokenRef.current ||
          !mountedRef.current
        ) {
          return
        }
        await onGitRepoReady?.(repo.id)
      } else {
        // Why: folder repos skip the Git default-checkout handoff, so activate the synthetic
        // root workspace before closing. Matches addNonGitFolder's behavior.
        await fetchWorktrees(repo.id)
        if (
          gen !== createGenRef.current ||
          requestHostToken !== hostTokenRef.current ||
          !mountedRef.current
        ) {
          return
        }
        const folderWorktree = useAppStore.getState().worktreesByRepo[repo.id]?.[0]
        if (folderWorktree) {
          activateAndRevealWorktree(folderWorktree.id, { sidebarRevealBehavior: 'auto' })
        }
        await markOnboardingProjectAdded('addedFolder', useAppStore.getState().settings)
        closeModal()
      }
    } catch (err) {
      if (
        gen !== createGenRef.current ||
        requestHostToken !== hostTokenRef.current ||
        !mountedRef.current
      ) {
        return
      }
      setCreateError(extractIpcErrorMessage(err, String(err)))
    } finally {
      // Why: only clear the loading state if this invocation is still current;
      // a superseded create must not flip the flag back off for a new flow.
      if (
        gen === createGenRef.current &&
        requestHostToken === hostTokenRef.current &&
        mountedRef.current
      ) {
        setIsCreating(false)
      }
    }
  }, [
    createName,
    createParent,
    fetchWorktrees,
    mountedRef,
    closeModal,
    onGitRepoReady,
    options.runtimeEnvironmentId,
    options.sshTargetId,
    options.devServerId
  ])

  return {
    createName,
    createParent,
    createError,
    isCreating,
    setCreateName,
    setCreateParent,
    setCreateError,
    resetCreateState,
    handlePickParent,
    handleCreate
  }
}
