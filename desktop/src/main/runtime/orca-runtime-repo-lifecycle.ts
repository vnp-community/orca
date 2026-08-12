/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
repo lifecycle command block, already covered by orca-runtime.ts's own
grandfathered max-lines disable before this move. Registered in
config/max-lines-baseline.txt per AGENTS.md — NEEDS PR REVIEW. */
// frontend/src/main/runtime/orca-runtime-repo-lifecycle.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-050): repo lifecycle commands
// (add/create/clone/show/update/remove/reorder + base-ref search/default)
// extracted from OrcaRuntimeService via the composition pattern.
// inspectTerminalProcess sits textually in the middle of this range
// (removeProject..reorderRepos) but is PTY-controller-adjacent, not repo
// lifecycle — deliberately excluded, stays in orca-runtime.ts.
import { randomUUID } from 'node:crypto'
import { isAbsolute, join } from 'node:path'
import { mkdir, readdir, rm, stat } from 'node:fs/promises'
import type { BaseRefSearchResult, Repo } from '../../shared/types'
import type { RuntimeRepoSearchRefs } from '../../shared/runtime-types'
import type { ExecutionHostId } from '../../shared/execution-host'
import { getRepoProviderConnectionKey, parseExecutionHostId } from '../../shared/execution-host'
import { DEFAULT_REPO_BADGE_COLOR } from '../../shared/constants'
import { isFolderRepo } from '../../shared/repo-kind'
import { getGitCloneFailureMessage } from '../../shared/git-clone-failure-message'
import { gitExecFileAsync, gitSpawn } from '../git/runner'
import { runWithGitReadCacheInvalidation } from '../git/status'
import {
  cleanupClaimedCloneTarget,
  claimCloneTarget,
  deriveValidatedClonePath,
  getClonePathComparisonKey
} from '../git/repo-clone-path'
import {
  getBaseRefDefault,
  getRemoteCount,
  isGitRepo,
  getRepoName,
  normalizeRefSearchQuery,
  parseRemoteCount,
  resolveDefaultBaseRefViaExec,
  searchBaseRefDetails,
  buildSearchBaseRefsArgv,
  isForEachRefExcludeUnsupportedError,
  mergeBaseRefSearchResultGroups,
  parseAndFilterSearchRefDetails
} from '../git/repo'
import { getSshGitCapabilityCache } from '../git/git-capability-state'
import { getRemoteGitProvider } from '../providers/ssh-git-dispatch'
import { detectRepoIconAndUpstream } from '../repo-icon-autodetect'
import { isENOENT, invalidateAuthorizedRootsCache } from '../ipc/filesystem-auth'
import { prepareLocalWorktreeRootForRepo } from '../worktree-root-preparation'
import { runtimePathsEqual } from './orca-runtime-tail-buffer'
import { omitUndefinedProperties, type RuntimeStore } from './orca-runtime'

// Why: repo-search default limit — only this domain's searchRepoRefs uses it.
const DEFAULT_REPO_SEARCH_REFS_LIMIT = 25

// Null executionHostId means host-unaware: path-only callers match any repo, and the first runtime
// host can adopt a legacy (unstamped) repo. But an unstamped repo with a connectionId is an SSH repo
// (resolves to ssh:<id>), so it must not be adopted/matched by a runtime host at the same path.
function runtimeRepoMatchesExecutionHost(
  repo: Pick<Repo, 'connectionId' | 'executionHostId'>,
  executionHostId?: ExecutionHostId | null
): boolean {
  if (executionHostId == null) {
    return true
  }
  if (repo.executionHostId != null) {
    return repo.executionHostId === executionHostId
  }
  return repo.connectionId == null
}

export type RuntimeRepoLifecycleCommandHost = {
  getStore(): RuntimeStore | null
  notifyReposChanged(): void
  invalidateResolvedWorktreeCache(): void
  resolveRepoSelector(selector: string): Promise<Repo>
}

export class RuntimeRepoLifecycleCommands {
  constructor(private readonly host: RuntimeRepoLifecycleCommandHost) {}

  // Why: clone-target path locking is per-domain state — only cloneRepo/
  // cloneRepoAfterPathLock touch it.
  private cloneInFlightByPath = new Map<string, Promise<void>>()

  async addRepo(
    path: string,
    kind: 'git' | 'folder' = 'git',
    executionHostId?: ExecutionHostId | null
  ): Promise<Repo> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    if (!isAbsolute(path)) {
      // Why: remote clients may run in a different cwd than the server. Require
      // server-side repo paths to be explicit so `orca serve` cwd is irrelevant.
      throw new Error('Project path must be an absolute path')
    }
    if (kind === 'git' && !isGitRepo(path)) {
      throw new Error(`Not a valid git repository: ${path}`)
    }

    const existing = store.getRepos().find((repo) => {
      if (!runtimePathsEqual(repo.path, path)) {
        return false
      }
      return runtimeRepoMatchesExecutionHost(repo, executionHostId)
    })
    if (existing) {
      // Only a runtime host backfills a legacy unstamped repo. An unstamped repo is
      // indistinguishable from a genuine local repo (both have null executionHostId and
      // connectionId), so we never stamp local/ssh onto it — that would re-attribute a
      // real local project to the wrong host. Runtime is the only host that lost its
      // identity to the pre-#7018 path-only import and needs the backfill.
      if (
        existing.executionHostId == null &&
        parseExecutionHostId(executionHostId)?.kind === 'runtime'
      ) {
        const adopted =
          store.updateRepo(existing.id, { executionHostId }) ??
          ({ ...existing, executionHostId } as Repo)
        this.host.invalidateResolvedWorktreeCache()
        this.host.notifyReposChanged()
        return adopted
      }
      return existing
    }

    const detected = await detectRepoIconAndUpstream({ repoPath: path, kind })
    const repo: Repo = {
      id: randomUUID(),
      path,
      displayName: getRepoName(path),
      badgeColor: DEFAULT_REPO_BADGE_COLOR,
      ...(executionHostId != null ? { executionHostId } : {}),
      ...detected,
      addedAt: Date.now(),
      kind,
      ...(kind === 'git'
        ? {
            externalWorktreeVisibility: 'hide' as const,
            externalWorktreeVisibilityLegacy: false
          }
        : {})
    }
    store.addRepo(repo)
    await prepareLocalWorktreeRootForRepo(store, repo)
    this.host.invalidateResolvedWorktreeCache()
    this.host.notifyReposChanged()
    return store.getRepo(repo.id) ?? repo
  }

  async createRepo(
    parentPath: string,
    name: string,
    kind: 'git' | 'folder' = 'git'
  ): Promise<{ repo: Repo } | { error: string }> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    const trimmedName = name.trim()
    const trimmedParentPath = parentPath.trim()
    const repoKind: 'git' | 'folder' = kind === 'folder' ? 'folder' : 'git'
    if (!trimmedName) {
      return { error: 'Name cannot be empty' }
    }
    if (/[\\/]/.test(trimmedName) || trimmedName === '.' || trimmedName === '..') {
      return { error: 'Name cannot contain slashes or be "." / ".."' }
    }
    if (!trimmedParentPath) {
      return { error: 'Parent directory is required' }
    }
    if (!isAbsolute(trimmedParentPath)) {
      return { error: 'Parent directory must be an absolute path' }
    }

    const targetPath = join(trimmedParentPath, trimmedName)
    const existing = store.getRepos().find((repo) => runtimePathsEqual(repo.path, targetPath))
    if (existing) {
      return { repo: existing }
    }

    let createdDir = false
    try {
      // Why: default create-project parents are host-home based and may not exist
      // before the first project is created on a fresh runtime.
      await mkdir(trimmedParentPath, { recursive: true })
      const existingStat = await stat(targetPath).catch((error: unknown) => {
        if (isENOENT(error)) {
          return null
        }
        throw error
      })
      if (existingStat) {
        if (!existingStat.isDirectory()) {
          return { error: `"${trimmedName}" already exists at this location and is not a folder.` }
        }
        const entries = await readdir(targetPath)
        if (entries.length > 0) {
          return { error: `"${trimmedName}" already exists at this location and is not empty.` }
        }
      } else {
        await mkdir(targetPath, { recursive: false })
        createdDir = true
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      return { error: `Failed to prepare directory: ${message}` }
    }

    if (repoKind === 'git') {
      let step: 'init' | 'commit' = 'init'
      try {
        await gitExecFileAsync(['init'], { cwd: targetPath })
        step = 'commit'
        await gitExecFileAsync(['commit', '--allow-empty', '-m', 'Initial commit'], {
          cwd: targetPath
        })
      } catch (error) {
        if (createdDir) {
          await rm(targetPath, { recursive: true, force: true }).catch(() => {})
        } else if (step === 'commit') {
          await rm(join(targetPath, '.git'), { recursive: true, force: true }).catch(() => {})
        }
        const message = error instanceof Error ? error.message : String(error)
        if (
          step === 'commit' &&
          /Please tell me who you are|user\.name|user\.email/i.test(message)
        ) {
          return {
            error:
              'Git author identity is not configured. Run `git config --global user.name "Your Name"` and `git config --global user.email "you@example.com"`, then try again.'
          }
        }
        const stepLabel =
          step === 'init'
            ? 'Failed to initialize git repository'
            : 'Failed to create initial commit'
        return { error: `${stepLabel}: ${message}` }
      }
    }

    const raceWinner = store.getRepos().find((repo) => runtimePathsEqual(repo.path, targetPath))
    if (raceWinner) {
      return { repo: raceWinner }
    }

    const detected = await detectRepoIconAndUpstream({ repoPath: targetPath, kind: repoKind })
    const repo: Repo = {
      id: randomUUID(),
      path: targetPath,
      displayName: trimmedName,
      badgeColor: DEFAULT_REPO_BADGE_COLOR,
      ...detected,
      addedAt: Date.now(),
      kind: repoKind,
      ...(repoKind === 'git'
        ? {
            externalWorktreeVisibility: 'hide' as const,
            externalWorktreeVisibilityLegacy: false
          }
        : {})
    }
    store.addRepo(repo)
    await prepareLocalWorktreeRootForRepo(store, repo)
    invalidateAuthorizedRootsCache()
    this.host.invalidateResolvedWorktreeCache()
    this.host.notifyReposChanged()
    return { repo: store.getRepo(repo.id) ?? repo }
  }

  async cloneRepo(
    url: string,
    destination: string,
    executionHostId?: ExecutionHostId | null
  ): Promise<Repo> {
    if (!this.host.getStore()) {
      throw new Error('runtime_unavailable')
    }
    const trimmedUrl = url.trim()
    const trimmedDestination = destination.trim()
    if (!trimmedDestination) {
      throw new Error('Clone destination is required')
    }
    const clonePath = deriveValidatedClonePath({ url: trimmedUrl, destination: trimmedDestination })
    const clonePathKey = getClonePathComparisonKey(clonePath)
    const previous = this.cloneInFlightByPath.get(clonePathKey) ?? Promise.resolve()
    let release!: () => void
    const current = new Promise<void>((resolve) => {
      release = resolve
    })
    const tail = previous.then(
      () => current,
      () => current
    )
    this.cloneInFlightByPath.set(clonePathKey, tail)

    try {
      await previous
      return await runWithGitReadCacheInvalidation(() =>
        this.cloneRepoAfterPathLock(
          trimmedUrl,
          trimmedDestination,
          clonePath,
          clonePathKey,
          executionHostId
        )
      )
    } finally {
      release()
      if (this.cloneInFlightByPath.get(clonePathKey) === tail) {
        this.cloneInFlightByPath.delete(clonePathKey)
      }
    }
  }

  private async cloneRepoAfterPathLock(
    trimmedUrl: string,
    trimmedDestination: string,
    clonePath: string,
    clonePathKey: string,
    executionHostId?: ExecutionHostId | null
  ): Promise<Repo> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    const existingBeforeClone = store
      .getRepos()
      .find(
        (repo) =>
          getClonePathComparisonKey(repo.path) === clonePathKey &&
          runtimeRepoMatchesExecutionHost(repo, executionHostId)
      )
    if (existingBeforeClone && !isFolderRepo(existingBeforeClone)) {
      return existingBeforeClone
    }

    await mkdir(trimmedDestination, { recursive: true })
    const claimedTarget = await claimCloneTarget(clonePath)
    await new Promise<void>((resolve, reject) => {
      let proc: ReturnType<typeof gitSpawn>
      try {
        proc = gitSpawn(['clone', '--progress', '--', trimmedUrl, clonePath], {
          cwd: trimmedDestination,
          stdio: ['ignore', 'ignore', 'pipe']
        })
      } catch (err) {
        void cleanupClaimedCloneTarget(clonePath, claimedTarget).finally(() => {
          const message = err instanceof Error ? err.message : String(err)
          reject(new Error(`Clone failed: ${message}`))
        })
        return
      }
      let stderrTail = ''
      let settled = false
      proc.stderr?.on('data', (chunk: Buffer) => {
        stderrTail = (stderrTail + chunk.toString()).slice(-4096)
      })
      const finishClone = async (
        code: number | null,
        signal: NodeJS.Signals | null,
        error?: Error
      ) => {
        if (settled) {
          return
        }
        settled = true
        const cloneSucceeded = !error && code === 0 && !signal
        if (!cloneSucceeded) {
          await cleanupClaimedCloneTarget(clonePath, claimedTarget)
        }

        if (error) {
          reject(new Error(`Clone failed: ${error.message}`))
        } else if (signal === 'SIGTERM') {
          reject(new Error('Clone aborted'))
        } else if (code === 0) {
          resolve()
        } else {
          reject(new Error(`Clone failed: ${getGitCloneFailureMessage(stderrTail, { clonePath })}`))
        }
      }
      proc.on('error', (error) => {
        void finishClone(null, null, error)
      })
      proc.on('close', (code, signal) => {
        void finishClone(code, signal)
      })
    })

    const existing = store
      .getRepos()
      .find(
        (repo) =>
          getClonePathComparisonKey(repo.path) === clonePathKey &&
          runtimeRepoMatchesExecutionHost(repo, executionHostId)
      )
    if (existing) {
      if (isFolderRepo(existing)) {
        const updated = store.updateRepo(existing.id, { kind: 'git' })
        if (updated) {
          await prepareLocalWorktreeRootForRepo(store, updated)
          invalidateAuthorizedRootsCache()
          this.host.invalidateResolvedWorktreeCache()
          this.host.notifyReposChanged()
          return updated
        }
      }
      return existing
    }

    const detected = await detectRepoIconAndUpstream({ repoPath: clonePath, kind: 'git' })
    const repo: Repo = {
      id: randomUUID(),
      path: clonePath,
      displayName: getRepoName(clonePath),
      badgeColor: DEFAULT_REPO_BADGE_COLOR,
      ...(executionHostId != null ? { executionHostId } : {}),
      ...detected,
      addedAt: Date.now(),
      kind: 'git',
      externalWorktreeVisibility: 'hide',
      externalWorktreeVisibilityLegacy: false
    }
    store.addRepo(repo)
    await prepareLocalWorktreeRootForRepo(store, repo)
    invalidateAuthorizedRootsCache()
    this.host.invalidateResolvedWorktreeCache()
    this.host.notifyReposChanged()
    return store.getRepo(repo.id) ?? repo
  }

  async showRepo(repoSelector: string): Promise<Repo> {
    return await this.host.resolveRepoSelector(repoSelector)
  }

  async setRepoBaseRef(repoSelector: string, baseRef: string): Promise<Repo> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    const repo = await this.host.resolveRepoSelector(repoSelector)
    if (isFolderRepo(repo)) {
      throw new Error('Folder mode does not support base refs.')
    }
    const updated = store.updateRepo(repo.id, { worktreeBaseRef: baseRef })
    if (!updated) {
      throw new Error('repo_not_found')
    }
    this.host.invalidateResolvedWorktreeCache()
    this.host.notifyReposChanged()
    return updated
  }

  async updateRepo(
    repoSelector: string,
    updates: Partial<
      Pick<
        Repo,
        | 'displayName'
        | 'badgeColor'
        | 'repoIcon'
        | 'upstream'
        | 'hookSettings'
        | 'worktreeBaseRef'
        | 'worktreeBasePath'
        | 'kind'
        | 'symlinkPaths'
        | 'issueSourcePreference'
        | 'externalWorktreeVisibility'
        | 'externalWorktreeVisibilityPromptDismissedAt'
        | 'externalWorktreeInboxBaselinePaths'
        | 'importedExternalWorktreePaths'
        | 'projectGroupId'
        | 'projectGroupOrder'
      >
    > & {
      sourceControlAi?: Repo['sourceControlAi'] | null
      externalWorktreeDiscoverySuppressedAt?: Repo['externalWorktreeDiscoverySuppressedAt'] | null
    }
  ): Promise<Repo> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    const repo = await this.host.resolveRepoSelector(repoSelector)
    const sanitizedUpdates = omitUndefinedProperties(updates)
    if ('worktreeBasePath' in updates && updates.worktreeBasePath === undefined) {
      sanitizedUpdates.worktreeBasePath = undefined
    }
    if (
      'externalWorktreeDiscoverySuppressedAt' in updates &&
      updates.externalWorktreeDiscoverySuppressedAt === null
    ) {
      sanitizedUpdates.externalWorktreeDiscoverySuppressedAt = undefined
    }
    if ('sourceControlAi' in updates && updates.sourceControlAi === null) {
      sanitizedUpdates.sourceControlAi = null
    }
    const updated = store.updateRepo(repo.id, sanitizedUpdates)
    if (!updated) {
      throw new Error('repo_not_found')
    }
    if ('worktreeBasePath' in updates) {
      await prepareLocalWorktreeRootForRepo(store, updated)
      invalidateAuthorizedRootsCache()
    }
    this.host.invalidateResolvedWorktreeCache()
    this.host.notifyReposChanged()
    return updated
  }

  async removeProject(repoSelector: string): Promise<{ removed: true }> {
    const store = this.host.getStore()
    if (!store?.removeProject) {
      throw new Error('runtime_unavailable')
    }
    const repo = await this.host.resolveRepoSelector(repoSelector)
    store.removeProject(repo.id)
    this.host.invalidateResolvedWorktreeCache()
    invalidateAuthorizedRootsCache()
    this.host.notifyReposChanged()
    return { removed: true }
  }

  reorderRepos(orderedIds: string[]): { status: 'applied' | 'rejected' } {
    const store = this.host.getStore()
    if (!store?.reorderRepos) {
      throw new Error('runtime_unavailable')
    }
    // Why: remote clients can race repo add/remove on the server just like
    // local drag-reorder can race another window. Let the store validate the
    // full permutation and signal a resync-worthy rejection.
    const applied = store.reorderRepos(orderedIds)
    if (!applied) {
      return { status: 'rejected' }
    }
    this.host.invalidateResolvedWorktreeCache()
    this.host.notifyReposChanged()
    return { status: 'applied' }
  }

  async searchRepoRefs(
    repoSelector: string,
    query: string,
    limit = DEFAULT_REPO_SEARCH_REFS_LIMIT
  ): Promise<RuntimeRepoSearchRefs> {
    if (!Number.isInteger(limit) || limit <= 0) {
      throw new Error('invalid_limit')
    }
    const repo = await this.host.resolveRepoSelector(repoSelector)
    if (isFolderRepo(repo)) {
      return {
        refs: [],
        truncated: false
      }
    }
    const refDetails = repo.connectionId
      ? await this.searchRemoteRepoRefs(repo, query, limit + 1)
      : await searchBaseRefDetails(repo.path, query, limit + 1)
    return {
      refs: refDetails.slice(0, limit).map((entry) => entry.refName),
      refDetails: refDetails.slice(0, limit),
      truncated: refDetails.length > limit
    }
  }

  async getRepoBaseRefDefault(
    repoSelector: string
  ): Promise<{ defaultBaseRef: string | null; remoteCount: number }> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    if (isFolderRepo(repo)) {
      return { defaultBaseRef: null, remoteCount: 0 }
    }
    if (getRepoProviderConnectionKey(repo)) {
      return this.getRemoteRepoBaseRefDefault(repo)
    }
    const [defaultBaseRef, remoteCount] = await Promise.all([
      getBaseRefDefault(repo.path),
      getRemoteCount(repo.path)
    ])
    return { defaultBaseRef, remoteCount }
  }

  private async getRemoteRepoBaseRefDefault(
    repo: Repo
  ): Promise<{ defaultBaseRef: string | null; remoteCount: number }> {
    const providerConnectionId = getRepoProviderConnectionKey(repo)
    const provider = providerConnectionId ? getRemoteGitProvider(providerConnectionId) : null
    if (!provider) {
      return { defaultBaseRef: null, remoteCount: 0 }
    }
    const [defaultBaseRef, remoteCount] = await Promise.all([
      resolveDefaultBaseRefViaExec(async (argv) => {
        try {
          return await provider.exec(argv, repo.path)
        } catch (err) {
          if (argv[0] === 'symbolic-ref') {
            console.warn('[runtime:repo.baseRefDefault] SSH symbolic-ref failed', {
              path: repo.path,
              err
            })
          }
          throw err
        }
      }),
      provider
        .exec(['remote'], repo.path)
        .then((result) => parseRemoteCount(result.stdout))
        .catch((err) => {
          console.warn('[runtime:repo.baseRefDefault] SSH git remote count failed', {
            path: repo.path,
            err
          })
          return 0
        })
    ])
    return { defaultBaseRef, remoteCount }
  }

  private async searchRemoteRepoRefs(
    repo: Repo,
    query: string,
    limit: number
  ): Promise<BaseRefSearchResult[]> {
    const providerConnectionId = getRepoProviderConnectionKey(repo)
    const provider = providerConnectionId ? getRemoteGitProvider(providerConnectionId) : null
    if (!provider) {
      return []
    }
    const normalizedQuery = normalizeRefSearchQuery(query)
    try {
      const remotesResult = await provider.exec(['remote'], repo.path).catch(() => ({ stdout: '' }))
      const remotes = remotesResult.stdout
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean)
      const capabilities = getSshGitCapabilityCache(provider)
      const runSearch = async (patternGroup?: 'segmented' | 'branchRoot'): Promise<string> => {
        return capabilities.runWithFallback(
          'for-each-ref-exclude',
          async () =>
            (
              await provider.exec(
                buildSearchBaseRefsArgv(normalizedQuery, limit, {
                  remoteNames: remotes,
                  patternGroup
                }),
                repo.path
              )
            ).stdout,
          async () =>
            (
              await provider.exec(
                buildSearchBaseRefsArgv(normalizedQuery, limit, {
                  excludeRemoteHead: false,
                  remoteNames: remotes,
                  patternGroup
                }),
                repo.path
              )
            ).stdout,
          isForEachRefExcludeUnsupportedError
        )
      }
      const searchTokens = normalizedQuery.split('/').filter((token) => token.length > 0)
      if (searchTokens.length > 1) {
        const results = await Promise.all([runSearch('segmented'), runSearch('branchRoot')])
        return mergeBaseRefSearchResultGroups(
          results.map((stdout) => parseAndFilterSearchRefDetails(stdout, limit, remotes)),
          limit
        )
      }
      return parseAndFilterSearchRefDetails(await runSearch(), limit, remotes)
    } catch (err) {
      console.warn('[runtime:repo.searchRefs] SSH for-each-ref failed', {
        path: repo.path,
        err
      })
      return []
    }
  }
}
