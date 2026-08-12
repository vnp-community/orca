/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
pre-existing removeManagedWorktree method (~490 lines verbatim, already
covered by orca-runtime.ts's own grandfathered max-lines disable before this
move) plus its direct helpers. Registered in config/max-lines-baseline.txt
per AGENTS.md — NEEDS PR REVIEW. The 490-line method itself is a separate,
un-tracked refactor candidate (see TASK-BIGFILE-039's task doc), not
addressed here to keep this a pure Move. */
// frontend/src/main/runtime/orca-runtime-branch-cleanup.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-039): worktree removal / preserved-
// branch cleanup commands extracted from OrcaRuntimeService via the
// composition pattern. Wider than originally scoped (5 methods) — 2 more
// private helpers (resolveWorktreeRemovalTarget, removeWorktreeMetadataAndHistory)
// only used by this domain came along, same "task doc under-scoped from a
// shallow grep" pattern seen in every prior BIGFILE task.
import type { AgentBrowserBridge } from '../browser/agent-browser-bridge'
import type { BrowserBackend } from '../browser/browser-backend'
import type { IPtyProvider } from '../providers/types'
import type {
  ForceDeleteWorktreeBranchResult,
  GitPushTarget,
  RemoveWorktreeResult,
  Repo
} from '../../shared/types'
import { gitExecFileAsync } from '../git/runner'
import { assertWorktreeUnlockedForRemoval } from '../../shared/worktree-removal'
import { splitWorktreeId } from '../../shared/worktree-id'
import { isFolderRepo } from '../../shared/repo-kind'
import { getRepoProviderConnectionKey } from '../../shared/execution-host'
import { isWindowsAbsolutePathLike } from '../../shared/cross-platform-path'
import { getLocalProjectWorktreeGitOptions } from '../project-runtime-git-options'
import {
  getLocalWorktreePathAccess,
  removeLocalWorktreePath,
  toLocalWorktreeRuntimePath
} from '../local-worktree-filesystem'
import {
  recoverLocalWindowsWorktreeRemoval,
  removeStaleLocalWorktreeRegistrationAfterFilesystemRemoval
} from '../local-worktree-removal-recovery'
import {
  assertWorktreeCleanForRemoval,
  forceDeleteLocalBranch,
  listWorktreesStrict,
  removeWorktree
} from '../git/worktree'
import { getEffectiveHooks, runHook } from '../hooks'
import { removeWorktreeLinkedPaths } from '../ipc/worktree-symlinks'
import {
  cleanupUnusedWorktreePushTargetRemote,
  cleanupUnusedWorktreePushTargetRemoteSsh
} from '../ipc/worktree-remote'
import {
  formatWorktreeRemovalError,
  isOrphanCompatiblePreflightError,
  isOrphanedWorktreeError
} from '../ipc/worktree-logic'
import {
  assertWorktreeDoesNotContainRegisteredWorktree,
  canCleanupUnregisteredOrcaLeftoverDirectory,
  canCleanupUnregisteredOrcaWorktreeDirectory,
  canSafelyRemoveOrphanedWorktreeDirectory,
  findRegisteredDeletableWorktree,
  isDangerousWorktreeRemovalPath,
  isWorktreePathMissing,
  ORPHANED_WORKTREE_DIRECTORY_MESSAGE,
  UNREGISTERED_MISSING_WORKTREE_MESSAGE
} from '../worktree-removal-safety'
import { invalidateAuthorizedRootsCache } from '../ipc/filesystem-auth'
import { closeLocalWatcherForWorktreePath } from '../ipc/filesystem-watcher'
import { killAllProcessesForWorktree } from './worktree-teardown'
import { getRemoteFilesystemProvider } from '../providers/ssh-filesystem-dispatch'
import { requireRemoteGitProvider } from '../providers/ssh-git-dispatch'
import { advertisedUrlWatcher } from '../ports/advertised-url-watcher'
import { serveSimStateWatcher } from '../emulator/serve-sim-state-watcher'
import { deleteWorktreeHistoryDir } from '../terminal-history'
import type { OrcaRuntimeService, RuntimeStore } from './orca-runtime'
import { getRuntimeFolderWorkspaceRootId } from './orca-runtime'
import type { Store } from '../persistence'

type RuntimeWorktreeRemovalTarget = {
  id: string
  repoId: string
  path: string
  pushTarget?: GitPushTarget
}

type RuntimeWorktreeRemovalInFlight = {
  optionsKey: string
  promise: Promise<RemoveWorktreeResult & { warning?: string }>
}

type PreservedBranchCleanupTarget = {
  branchName: string
  head: string
  pushTarget?: GitPushTarget
}

function getRuntimeWorktreeRemovalOptionsKey(force: boolean, runHooks: boolean): string {
  return `${force ? 'force' : 'normal'}:${runHooks ? 'run-hooks' : 'skip-hooks'}`
}

function parseExactWorktreeIdSelector(selector: string): RuntimeWorktreeRemovalTarget | null {
  const worktreeId = selector.startsWith('id:') ? selector.slice(3) : selector
  const parsed = splitWorktreeId(worktreeId)
  if (!parsed || !parsed.repoId || !parsed.worktreePath) {
    return null
  }
  return {
    id: worktreeId,
    repoId: parsed.repoId,
    path: parsed.worktreePath
  }
}

function gitStatusErrorMeansNotRepository(error: unknown): boolean {
  const message =
    error instanceof Error
      ? error.message
      : error && typeof error === 'object' && 'message' in error
        ? String((error as { message: unknown }).message)
        : typeof error === 'string'
          ? error
          : ''
  const stderr =
    error && typeof error === 'object' && 'stderr' in error
      ? String((error as { stderr: unknown }).stderr)
      : ''
  return /not a git repository/i.test(`${message}\n${stderr}`)
}

async function isRuntimeWorktreePathMissing(
  repo: Repo,
  worktreePath: string,
  localWorktreeGitOptions: { wslDistro?: string } = {}
): Promise<boolean> {
  const providerConnectionId = getRepoProviderConnectionKey(repo)
  if (!providerConnectionId) {
    const access = getLocalWorktreePathAccess(localWorktreeGitOptions)
    return isWorktreePathMissing(
      toLocalWorktreeRuntimePath(worktreePath, localWorktreeGitOptions),
      access.statPath
    )
  }

  const fsProvider = getRemoteFilesystemProvider(providerConnectionId)
  if (!fsProvider) {
    return false
  }
  return isWorktreePathMissing(worktreePath, (path) => fsProvider.stat(path))
}

async function isLocalRuntimeGitRepository(
  runtimeWorktreePath: string,
  localWorktreeGitOptions: { wslDistro?: string } = {}
): Promise<boolean> {
  try {
    await gitExecFileAsync(['status', '--short'], {
      cwd: runtimeWorktreePath,
      ...localWorktreeGitOptions
    })
    return true
  } catch (error) {
    return !gitStatusErrorMeansNotRepository(error)
  }
}

export type RuntimeBranchCleanupCommandHost = {
  getStore(): RuntimeStore | null
  requireStore(): Store
  resolveWorktreeSelector(
    selector: string
  ): Promise<{ id: string; repoId: string; path: string; pushTarget?: GitPushTarget }>
  getAgentBrowserBridge(): AgentBrowserBridge | null
  getOffscreenBrowserBackend(): BrowserBackend | null
  getLocalProvider(): IPtyProvider | null
  getOnPtyStopped(): ((ptyId: string) => void) | null
  clearOptimisticReconcileToken(worktreeId: string): void
  invalidateResolvedWorktreeCache(): void
  notifyWorktreesChanged(repoId: string): void
  // Why: worktree-teardown.ts's killAllProcessesForWorktree needs the real
  // OrcaRuntimeService instance (calls .stopTerminalsForWorktree on it) — `this`
  // inside this class would be the wrong object, so the host provides it.
  getRuntimeForTeardown(): OrcaRuntimeService
}

export class RuntimeBranchCleanupCommands {
  private readonly removeManagedWorktreeInFlight = new Map<string, RuntimeWorktreeRemovalInFlight>()
  private readonly preservedBranchCleanupByWorktreeId = new Map<
    string,
    PreservedBranchCleanupTarget
  >()

  constructor(private readonly host: RuntimeBranchCleanupCommandHost) {}

  // Why: headless offscreen browser pages are main-process BrowserWindows that
  // outlive a worktree unless explicitly closed — removing a worktree without
  // closing its open panes leaks the windows for the life of the serve process.
  private closeHeadlessBrowserPagesForWorktree(worktreeId: string): void {
    const offscreenBrowserBackend = this.host.getOffscreenBrowserBackend()
    const agentBrowserBridge = this.host.getAgentBrowserBridge()
    if (!offscreenBrowserBackend || !agentBrowserBridge?.tabList) {
      return
    }
    for (const tab of agentBrowserBridge.tabList(worktreeId).tabs) {
      void offscreenBrowserBackend.closeTab(tab.browserPageId).catch(() => {})
    }
  }

  private removeWorktreeMetadataAndHistory(store: RuntimeStore, worktreeId: string): void {
    // Why: worktree IDs are path-derived and can be recreated, so removal must
    // purge history and process-local caches before the ID points at new state.
    store.removeWorktreeMeta(worktreeId)
    advertisedUrlWatcher.forgetWorktree(worktreeId)
    serveSimStateWatcher.forgetWorktree(worktreeId)
    deleteWorktreeHistoryDir(worktreeId)
    this.closeHeadlessBrowserPagesForWorktree(worktreeId)
  }

  private async resolveWorktreeRemovalTarget(
    worktreeSelector: string
  ): Promise<RuntimeWorktreeRemovalTarget> {
    try {
      const worktree = await this.host.resolveWorktreeSelector(worktreeSelector)
      const removalTarget = {
        id: worktree.id,
        repoId: worktree.repoId,
        path: worktree.path
      }
      return worktree.pushTarget
        ? { ...removalTarget, pushTarget: worktree.pushTarget }
        : removalTarget
    } catch (error) {
      if (!(error instanceof Error) || error.message !== 'selector_not_found') {
        throw error
      }
      const removalTarget = parseExactWorktreeIdSelector(worktreeSelector)
      const meta = removalTarget
        ? this.host.getStore()?.getWorktreeMeta(removalTarget.id)
        : undefined
      if (!removalTarget || !meta) {
        throw error
      }
      // Why: delete requests can arrive after Git no longer lists the worktree.
      // Only exact IDs with persisted Orca metadata are accepted here so
      // branch/path selectors cannot resolve to an arbitrary missing path.
      return meta.pushTarget ? { ...removalTarget, pushTarget: meta.pushTarget } : removalTarget
    }
  }

  private rememberPreservedBranchCleanupTarget(
    worktreeId: string,
    result: RemoveWorktreeResult | undefined,
    fallbackHead: string | undefined,
    pushTarget: GitPushTarget | undefined
  ): void {
    if (result?.preservedBranch) {
      const head = result.preservedBranch.head ?? fallbackHead
      if (!head) {
        throw new Error(
          `Cannot safely offer force-delete for preserved branch "${result.preservedBranch.branchName}" without its saved commit.`
        )
      }
      this.preservedBranchCleanupByWorktreeId.set(worktreeId, {
        branchName: result.preservedBranch.branchName,
        head,
        ...(pushTarget ? { pushTarget } : {})
      })
      return
    }
    this.preservedBranchCleanupByWorktreeId.delete(worktreeId)
  }

  private preserveBranchHeadFallback(
    result: RemoveWorktreeResult | undefined,
    fallbackHead: string | undefined
  ): RemoveWorktreeResult {
    if (!result?.preservedBranch || result.preservedBranch.head || !fallbackHead) {
      return result ?? {}
    }
    return {
      ...result,
      preservedBranch: {
        ...result.preservedBranch,
        head: fallbackHead
      }
    }
  }

  async forceDeletePreservedBranch(
    worktreeSelector: string,
    branchName: string,
    expectedHead: string
  ): Promise<ForceDeleteWorktreeBranchResult> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    const removalTarget = parseExactWorktreeIdSelector(worktreeSelector)
    const cleanupTarget = removalTarget
      ? this.preservedBranchCleanupByWorktreeId.get(removalTarget.id)
      : undefined
    if (
      !removalTarget ||
      !cleanupTarget ||
      cleanupTarget.branchName !== branchName ||
      cleanupTarget.head !== expectedHead
    ) {
      throw new Error(`No preserved branch cleanup is pending for "${branchName}".`)
    }

    const repo = store.getRepo(removalTarget.repoId)
    if (!repo) {
      throw new Error('repo_not_found')
    }
    if (isFolderRepo(repo)) {
      throw new Error('Folder workspaces do not have local Git branches.')
    }

    const providerConnectionId = getRepoProviderConnectionKey(repo)
    if (providerConnectionId) {
      const provider = requireRemoteGitProvider(providerConnectionId)
      // Why: SSH must use the write-capable relay RPC; the shared exec-based
      // helper routes through the read-only git.exec allowlist, which rejects
      // the worktree/update-ref/config writes this delete needs.
      await provider.forceDeletePreservedBranch!(
        repo.path,
        cleanupTarget.branchName,
        cleanupTarget.head
      )
      await cleanupUnusedWorktreePushTargetRemoteSsh(
        provider,
        repo.path,
        removalTarget.id,
        cleanupTarget.pushTarget,
        store
      )
    } else {
      const localWorktreeGitOptions = getLocalProjectWorktreeGitOptions(
        this.host.requireStore(),
        repo
      )
      await (Object.keys(localWorktreeGitOptions).length > 0
        ? forceDeleteLocalBranch(
            repo.path,
            cleanupTarget.branchName,
            cleanupTarget.head,
            (argv, cwd) => gitExecFileAsync(argv, { cwd, ...localWorktreeGitOptions })
          )
        : forceDeleteLocalBranch(repo.path, cleanupTarget.branchName, cleanupTarget.head))
      await cleanupUnusedWorktreePushTargetRemote(
        repo.path,
        removalTarget.id,
        cleanupTarget.pushTarget,
        store,
        localWorktreeGitOptions
      )
    }

    this.preservedBranchCleanupByWorktreeId.delete(removalTarget.id)
    return { deleted: true }
  }

  async removeManagedWorktree(
    worktreeSelector: string,
    force = false,
    runHooks = false
  ): Promise<RemoveWorktreeResult & { warning?: string }> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    const removalTarget = await this.resolveWorktreeRemovalTarget(worktreeSelector)
    const optionsKey = getRuntimeWorktreeRemovalOptionsKey(force, runHooks)
    const inFlightRemoval = this.removeManagedWorktreeInFlight.get(removalTarget.id)
    if (inFlightRemoval) {
      if (inFlightRemoval.optionsKey === optionsKey) {
        return inFlightRemoval.promise
      }
      throw new Error(`Worktree deletion already in progress: ${removalTarget.id}`)
    }

    // Why: runtime callers can race the same workspace through CLI/mobile
    // retries. Share one destructive Git/filesystem operation per worktree ID.
    const removal = (async (): Promise<RemoveWorktreeResult & { warning?: string }> => {
      const repo = store.getRepo(removalTarget.repoId)
      if (!repo) {
        throw new Error('repo_not_found')
      }
      if (isFolderRepo(repo)) {
        if (removalTarget.id === getRuntimeFolderWorkspaceRootId(repo)) {
          throw new Error(
            'Cannot delete the project root workspace. Remove the folder project instead.'
          )
        }
        const localProvider = this.host.getLocalProvider()
        if (localProvider) {
          // Why: folder workspace deletion has no Git removal phase where PTYs
          // would otherwise be swept; tear them down before hiding the workspace.
          await killAllProcessesForWorktree(removalTarget.id, {
            runtime: this.host.getRuntimeForTeardown(),
            localProvider,
            onPtyStopped: this.host.getOnPtyStopped() ?? undefined
          }).catch((err) => {
            console.warn(`[worktree-teardown] failed for ${removalTarget.id}:`, err)
          })
        }
        this.removeWorktreeMetadataAndHistory(store, removalTarget.id)
        this.preservedBranchCleanupByWorktreeId.delete(removalTarget.id)
        this.host.invalidateResolvedWorktreeCache()
        this.host.notifyWorktreesChanged(repo.id)
        return {}
      }
      const providerConnectionId = getRepoProviderConnectionKey(repo)
      const provider = providerConnectionId ? requireRemoteGitProvider(providerConnectionId) : null
      const fsProvider = providerConnectionId
        ? getRemoteFilesystemProvider(providerConnectionId)
        : null
      const localWorktreeGitOptions = providerConnectionId
        ? {}
        : getLocalProjectWorktreeGitOptions(this.host.requireStore(), repo)
      const hasLocalWorktreeGitOptions = Object.keys(localWorktreeGitOptions).length > 0
      const registeredWorktrees = providerConnectionId
        ? await provider!.listWorktrees(repo.path)
        : hasLocalWorktreeGitOptions
          ? await listWorktreesStrict(repo.path, localWorktreeGitOptions)
          : await listWorktreesStrict(repo.path)
      const removedMeta = store.getWorktreeMeta(removalTarget.id)
      const removedPushTarget = removedMeta?.pushTarget ?? removalTarget.pushTarget
      const registeredWorktree = findRegisteredDeletableWorktree(
        repo.path,
        removalTarget.path,
        registeredWorktrees
      )
      if (!registeredWorktree) {
        let canCleanOrphanedDirectory = false
        if (
          canCleanupUnregisteredOrcaWorktreeDirectory({
            meta: removedMeta
          })
        ) {
          if (providerConnectionId) {
            if (!fsProvider) {
              throw new Error('SSH filesystem provider unavailable')
            }
            if (!fsProvider.lstat) {
              throw new Error('SSH filesystem provider lstat unavailable')
            }
            canCleanOrphanedDirectory = await canSafelyRemoveOrphanedWorktreeDirectory(
              removalTarget.path,
              repo.path,
              (path) => fsProvider.lstat!(path),
              (path) => fsProvider.readFile(path)
            )
          } else {
            const access = getLocalWorktreePathAccess(localWorktreeGitOptions)
            canCleanOrphanedDirectory =
              !isDangerousWorktreeRemovalPath(removalTarget.path, repo.path) &&
              (await canSafelyRemoveOrphanedWorktreeDirectory(
                toLocalWorktreeRuntimePath(removalTarget.path, localWorktreeGitOptions),
                toLocalWorktreeRuntimePath(repo.path, localWorktreeGitOptions),
                access.statPath,
                access.readPath
              ))
          }
        }
        if (canCleanOrphanedDirectory) {
          assertWorktreeDoesNotContainRegisteredWorktree(removalTarget.path, registeredWorktrees)
          if (!force) {
            throw new Error(ORPHANED_WORKTREE_DIRECTORY_MESSAGE)
          }
          if (providerConnectionId) {
            await fsProvider!.deletePath(removalTarget.path, true)
            await cleanupUnusedWorktreePushTargetRemoteSsh(
              provider!,
              repo.path,
              removalTarget.id,
              removedPushTarget,
              store
            )
          } else {
            await removeLocalWorktreePath(removalTarget.path, localWorktreeGitOptions)
            await cleanupUnusedWorktreePushTargetRemote(
              repo.path,
              removalTarget.id,
              removedPushTarget,
              store,
              localWorktreeGitOptions
            )
          }
          this.host.clearOptimisticReconcileToken(removalTarget.id)
          this.removeWorktreeMetadataAndHistory(store, removalTarget.id)
          this.preservedBranchCleanupByWorktreeId.delete(removalTarget.id)
          this.host.invalidateResolvedWorktreeCache()
          invalidateAuthorizedRootsCache()
          this.host.notifyWorktreesChanged(repo.id)
          return {}
        }
        if (!providerConnectionId) {
          const access = getLocalWorktreePathAccess(localWorktreeGitOptions)
          const runtimeWorktreePath = toLocalWorktreeRuntimePath(
            removalTarget.path,
            localWorktreeGitOptions
          )
          if (
            await canCleanupUnregisteredOrcaLeftoverDirectory({
              meta: removedMeta,
              worktreePath: removalTarget.path,
              runtimeWorktreePath,
              repo,
              runtimeRepoPath: toLocalWorktreeRuntimePath(repo.path, localWorktreeGitOptions),
              registeredWorktrees,
              statPath: access.statPath,
              isGitRepository: (path) => isLocalRuntimeGitRepository(path, localWorktreeGitOptions)
            })
          ) {
            if (!force) {
              throw new Error(ORPHANED_WORKTREE_DIRECTORY_MESSAGE)
            }
            await closeLocalWatcherForWorktreePath(removalTarget.path).catch((err) => {
              console.warn(`[filesystem-watcher] failed to close ${removalTarget.path}:`, err)
            })
            await removeLocalWorktreePath(removalTarget.path, localWorktreeGitOptions)
            await cleanupUnusedWorktreePushTargetRemote(
              repo.path,
              removalTarget.id,
              removedPushTarget,
              store,
              localWorktreeGitOptions
            )
            this.host.clearOptimisticReconcileToken(removalTarget.id)
            this.removeWorktreeMetadataAndHistory(store, removalTarget.id)
            this.preservedBranchCleanupByWorktreeId.delete(removalTarget.id)
            this.host.invalidateResolvedWorktreeCache()
            invalidateAuthorizedRootsCache()
            this.host.notifyWorktreesChanged(repo.id)
            return {}
          }
        }
        if (await isRuntimeWorktreePathMissing(repo, removalTarget.path, localWorktreeGitOptions)) {
          if (!force && !removedMeta) {
            // Why: without persisted metadata, require the renderer recovery
            // path before deleting Orca-only state for an unregistered path.
            throw new Error(UNREGISTERED_MISSING_WORKTREE_MESSAGE)
          }
          // Why: a manually deleted worktree is already gone from Git and disk.
          // Finish runtime metadata cleanup without requiring force or touching
          // any unregistered path that still exists.
          await (providerConnectionId
            ? cleanupUnusedWorktreePushTargetRemoteSsh(
                provider!,
                repo.path,
                removalTarget.id,
                removedPushTarget,
                store
              )
            : cleanupUnusedWorktreePushTargetRemote(
                repo.path,
                removalTarget.id,
                removedPushTarget,
                store,
                localWorktreeGitOptions
              ))
          this.host.clearOptimisticReconcileToken(removalTarget.id)
          this.removeWorktreeMetadataAndHistory(store, removalTarget.id)
          this.preservedBranchCleanupByWorktreeId.delete(removalTarget.id)
          this.host.invalidateResolvedWorktreeCache()
          invalidateAuthorizedRootsCache()
          this.host.notifyWorktreesChanged(repo.id)
          return {}
        }
        throw new Error(`Refusing to delete unregistered worktree path: ${removalTarget.path}`)
      }
      const canonicalWorktreePath = registeredWorktree.path
      const deleteBranch = removedMeta?.preserveBranchOnDelete !== true

      // Why: a Git lock must block before archive hooks or linked-path cleanup
      // mutate the workspace; dirty-file force is a separate permission.
      try {
        assertWorktreeUnlockedForRemoval(registeredWorktree)
      } catch (error) {
        throw new Error(formatWorktreeRemovalError(error, canonicalWorktreePath, force))
      }

      // Why: a prior forced Windows recovery can delete the directory but leave
      // Git's stale registration; recover and verify it before clearing metadata.
      if (
        !providerConnectionId &&
        force === true &&
        process.platform === 'win32' &&
        (isWindowsAbsolutePathLike(canonicalWorktreePath) || !!localWorktreeGitOptions.wslDistro) &&
        removedMeta &&
        (await isRuntimeWorktreePathMissing(repo, canonicalWorktreePath, localWorktreeGitOptions))
      ) {
        const removalResult = await removeStaleLocalWorktreeRegistrationAfterFilesystemRemoval({
          canonicalWorktreePath,
          repoPath: repo.path,
          localWorktreeGitOptions,
          registeredWorktree,
          deleteBranch
        })
        await cleanupUnusedWorktreePushTargetRemote(
          repo.path,
          removalTarget.id,
          removedPushTarget,
          store,
          localWorktreeGitOptions
        )
        this.rememberPreservedBranchCleanupTarget(
          removalTarget.id,
          removalResult,
          registeredWorktree.head,
          removedPushTarget
        )
        this.host.clearOptimisticReconcileToken(removalTarget.id)
        this.removeWorktreeMetadataAndHistory(store, removalTarget.id)
        this.host.invalidateResolvedWorktreeCache()
        invalidateAuthorizedRootsCache()
        this.host.notifyWorktreesChanged(repo.id)
        return removalResult ?? {}
      }
      if (providerConnectionId) {
        const remoteRemoveOptions = !deleteBranch ? { deleteBranch } : {}
        const rawRemovalResult = await (Object.keys(remoteRemoveOptions).length > 0
          ? provider!.removeWorktree(canonicalWorktreePath, force, remoteRemoveOptions)
          : provider!.removeWorktree(canonicalWorktreePath, force))
        const removalResult = this.preserveBranchHeadFallback(
          rawRemovalResult,
          registeredWorktree.head
        )
        await cleanupUnusedWorktreePushTargetRemoteSsh(
          provider!,
          repo.path,
          removalTarget.id,
          removedPushTarget,
          store
        )
        this.rememberPreservedBranchCleanupTarget(
          removalTarget.id,
          removalResult,
          registeredWorktree.head,
          removedPushTarget
        )
        this.host.clearOptimisticReconcileToken(removalTarget.id)
        this.removeWorktreeMetadataAndHistory(store, removalTarget.id)
        this.host.invalidateResolvedWorktreeCache()
        invalidateAuthorizedRootsCache()
        this.host.notifyWorktreesChanged(repo.id)
        return removalResult ?? {}
      }

      const hooks = getEffectiveHooks(repo)
      let warning: string | undefined
      if (hooks?.scripts.archive && runHooks) {
        const result = await runHook(
          'archive',
          canonicalWorktreePath,
          repo,
          undefined,
          hasLocalWorktreeGitOptions ? localWorktreeGitOptions : undefined
        )
        if (!result.success) {
          console.error(`[hooks] archive hook failed for ${canonicalWorktreePath}:`, result.output)
        }
      } else if (hooks?.scripts.archive) {
        // Runtime RPC calls have no renderer trust prompt, so hooks require explicit CLI opt-in.
        warning = `orca.yaml archive hook skipped for ${canonicalWorktreePath}; pass --run-hooks to run it.`
        console.warn(`[hooks] ${warning}`)
      }

      const refreshedWorktrees = hasLocalWorktreeGitOptions
        ? await listWorktreesStrict(repo.path, localWorktreeGitOptions)
        : await listWorktreesStrict(repo.path)
      const refreshedRegisteredWorktree = findRegisteredDeletableWorktree(
        repo.path,
        canonicalWorktreePath,
        refreshedWorktrees
      )
      if (!refreshedRegisteredWorktree) {
        throw new Error(
          `Worktree registration changed during deletion: ${canonicalWorktreePath}. Retry deletion.`
        )
      }
      try {
        // Why: an archive hook can race another Git client that locks the row;
        // recheck before linked-path, watcher, or terminal teardown side effects.
        assertWorktreeUnlockedForRemoval(refreshedRegisteredWorktree)
      } catch (error) {
        throw new Error(formatWorktreeRemovalError(error, canonicalWorktreePath, force))
      }

      let shouldTearDownPtys = true
      if (repo.symlinkPaths && repo.symlinkPaths.length > 0) {
        await removeWorktreeLinkedPaths(canonicalWorktreePath, repo.symlinkPaths)
      }
      try {
        await (hasLocalWorktreeGitOptions
          ? assertWorktreeCleanForRemoval(canonicalWorktreePath, force, localWorktreeGitOptions)
          : assertWorktreeCleanForRemoval(canonicalWorktreePath, force))
      } catch (error) {
        if (!isOrphanCompatiblePreflightError(error)) {
          throw new Error(formatWorktreeRemovalError(error, canonicalWorktreePath, force))
        }
        // Why: orphan cleanup does not need live shells to be killed first,
        // and preflight did not prove the worktree is cleanly removable.
        shouldTearDownPtys = false
      }

      const localProvider = this.host.getLocalProvider()
      await closeLocalWatcherForWorktreePath(canonicalWorktreePath).catch((err) => {
        console.warn(`[filesystem-watcher] failed to close ${canonicalWorktreePath}:`, err)
      })
      if (localProvider && shouldTearDownPtys) {
        // Why: once preflight proves normal deletion is clean, kill PTYs before
        // git-level removal so Windows handles cannot keep the directory busy. This also
        // closes the headless-CLI leak for confirmed-removable worktrees.
        await killAllProcessesForWorktree(removalTarget.id, {
          runtime: this.host.getRuntimeForTeardown(),
          localProvider,
          onPtyStopped: this.host.getOnPtyStopped() ?? undefined
        })
          .then((r) => {
            const total = r.runtimeStopped + r.providerStopped + r.registryStopped
            if (total > 0) {
              // Why (design §4.4 observability): breadcrumb lets ops
              // distinguish a renderer-state-induced leak (diff-path purge
              // non-empty) from a backend-induced one (nothing to kill but
              // memory still pinned). Emit only when the sweep actually did
              // work so steady-state logs stay quiet.
              console.info(
                `[worktree-teardown] ${removalTarget.id} killed runtime=${r.runtimeStopped} provider=${r.providerStopped} registry=${r.registryStopped}`
              )
            }
          })
          .catch((err) => {
            console.warn(`[worktree-teardown] failed for ${removalTarget.id}:`, err)
          })
      }

      let removalResult: RemoveWorktreeResult | undefined
      try {
        const removeOptions = {
          ...(!deleteBranch ? { deleteBranch } : {}),
          // Why: removal already validated the Git row under the selected
          // project runtime; keep branch cleanup on that same canonical row.
          knownRemovedWorktree: refreshedRegisteredWorktree,
          ...localWorktreeGitOptions
        }
        removalResult = this.preserveBranchHeadFallback(
          await removeWorktree(repo.path, canonicalWorktreePath, force, removeOptions),
          refreshedRegisteredWorktree.head
        )
      } catch (error) {
        // Why: Git for Windows can deregister a clean worktree before its
        // recursive filesystem deletion fails transiently.
        const recoveredRemovalResult = await recoverLocalWindowsWorktreeRemoval({
          error,
          force,
          canonicalWorktreePath,
          repoPath: repo.path,
          localWorktreeGitOptions,
          registeredWorktree: refreshedRegisteredWorktree,
          deleteBranch,
          closeWatcher: (worktreePath) =>
            closeLocalWatcherForWorktreePath(worktreePath).catch((err) => {
              console.warn(`[filesystem-watcher] failed to close ${worktreePath}:`, err)
            })
        })
        if (recoveredRemovalResult) {
          removalResult = recoveredRemovalResult
        } else if (isOrphanedWorktreeError(error)) {
          const access = getLocalWorktreePathAccess(localWorktreeGitOptions)
          if (
            await canSafelyRemoveOrphanedWorktreeDirectory(
              toLocalWorktreeRuntimePath(canonicalWorktreePath, localWorktreeGitOptions),
              toLocalWorktreeRuntimePath(repo.path, localWorktreeGitOptions),
              access.statPath,
              access.readPath
            )
          ) {
            await closeLocalWatcherForWorktreePath(canonicalWorktreePath).catch((err) => {
              console.warn(`[filesystem-watcher] failed to close ${canonicalWorktreePath}:`, err)
            })
            await removeLocalWorktreePath(canonicalWorktreePath, localWorktreeGitOptions).catch(
              () => {}
            )
          } else {
            console.warn(
              `[worktrees] Refusing recursive cleanup for unproven worktree directory: ${canonicalWorktreePath}`
            )
          }
          // Why: `git worktree remove` failed, so git's internal worktree tracking
          // (`.git/worktrees/<name>`) is still intact. Without pruning, `git worktree
          // list` continues to show the stale entry and the branch it had checked out
          // remains locked — other worktrees cannot check it out.
          await gitExecFileAsync(['worktree', 'prune'], {
            cwd: repo.path,
            ...localWorktreeGitOptions
          }).catch(() => {})
          await cleanupUnusedWorktreePushTargetRemote(
            repo.path,
            removalTarget.id,
            removedPushTarget,
            store,
            localWorktreeGitOptions
          )
          this.host.clearOptimisticReconcileToken(removalTarget.id)
          this.removeWorktreeMetadataAndHistory(store, removalTarget.id)
          this.preservedBranchCleanupByWorktreeId.delete(removalTarget.id)
          this.host.invalidateResolvedWorktreeCache()
          invalidateAuthorizedRootsCache()
          this.host.notifyWorktreesChanged(repo.id)
          return warning ? { warning } : {}
        } else {
          throw new Error(formatWorktreeRemovalError(error, canonicalWorktreePath, force))
        }
      }

      await cleanupUnusedWorktreePushTargetRemote(
        repo.path,
        removalTarget.id,
        removedPushTarget,
        store,
        localWorktreeGitOptions
      )
      this.rememberPreservedBranchCleanupTarget(
        removalTarget.id,
        removalResult,
        refreshedRegisteredWorktree.head,
        removedPushTarget
      )
      this.host.clearOptimisticReconcileToken(removalTarget.id)
      this.removeWorktreeMetadataAndHistory(store, removalTarget.id)
      this.host.invalidateResolvedWorktreeCache()
      invalidateAuthorizedRootsCache()
      this.host.notifyWorktreesChanged(repo.id)
      return {
        ...removalResult,
        ...(warning ? { warning } : {})
      }
    })()
    this.removeManagedWorktreeInFlight.set(removalTarget.id, { optionsKey, promise: removal })
    try {
      return await removal
    } finally {
      if (this.removeManagedWorktreeInFlight.get(removalTarget.id)?.promise === removal) {
        this.removeManagedWorktreeInFlight.delete(removalTarget.id)
      }
    }
  }
}
