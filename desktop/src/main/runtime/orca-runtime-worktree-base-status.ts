/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
worktree base-status/drift-reconciliation and PR/MR-base-resolution method
block, already covered by orca-runtime.ts's own grandfathered max-lines
disable before this move. Registered in config/max-lines-baseline.txt per
AGENTS.md — NEEDS PR REVIEW. */
// frontend/src/main/runtime/orca-runtime-worktree-base-status.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-047): worktree base-status/drift
// reconciliation and managed PR/MR base-resolution commands extracted from
// OrcaRuntimeService via the composition pattern. Deliberately excludes the
// PTY-adjacent worktree cluster (stopTerminalsForWorktree and friends) and
// the scattered managed-worktree list/create/activate cluster — per the
// user's explicit choice to avoid PTY-lifecycle-entangled code for now.
import type {
  GitHubPrStartPoint,
  GitPushTarget,
  Repo,
  WorktreeBaseStatusEvent,
  WorktreeMeta,
  WorktreeRemoteBranchConflictEvent
} from '../../shared/types'
import type { RemoteFetchResult, RemoteTrackingBase } from './orca-runtime-types'
import type { Store } from '../persistence'
import { randomUUID } from 'node:crypto'
import { gitExecFileAsync } from '../git/runner'
import {
  getBaseRefDefault,
  getDefaultRemote,
  getRemoteDrift,
  getRecentDriftSubjects
} from '../git/repo'
import {
  getLocalProjectGitExecOptions,
  getLocalProjectWorktreeGitOptions
} from '../project-runtime-git-options'
import { isFolderRepo } from '../../shared/repo-kind'
import { getRepoProviderConnectionKey } from '../../shared/execution-host'
import { requireRemoteGitProvider } from '../providers/ssh-git-dispatch'
import { worktreeWorkspaceKey } from '../../shared/workspace-scope'
import { stripOrcaProvenanceMetaUpdates } from '../worktree-removal-safety'
import {
  getProjectRefForRemote as getGitLabProjectRefForRemote,
  getWorkItemByProjectRef as getGitLabWorkItemByProjectRef
} from '../gitlab/client'
import { getGlabKnownHosts } from '../gitlab/gl-utils'
import { resolveGitHubPrStartPoint } from '../github/pr-start-point'
import { fetchPrHeadTrackingRef } from '../github/pr-head-tracking-ref'
import {
  omitUndefinedProperties,
  RuntimeLineageError,
  type ResolvedWorktree,
  type RuntimeStore
} from './orca-runtime'

// Why: probeWorktreeDrift's recent-commit-subjects preview is UI-facing —
// bounded independently of any git log limit elsewhere. Only this domain
// reads it.
const DRIFT_PROBE_SUBJECT_LIMIT = 5

type RuntimeWorktreeBaseNotifier = {
  worktreeBaseStatus?(event: WorktreeBaseStatusEvent): void
  worktreeRemoteBranchConflict?(event: WorktreeRemoteBranchConflictEvent): void
}

export type RuntimeWorktreeBaseStatusCommandHost = {
  getStore(): RuntimeStore | null
  requireStore(): Store
  resolveWorktreeSelector(selector: string): Promise<ResolvedWorktree>
  resolveRepoSelector(selector: string): Promise<Repo>
  showManagedWorktree(worktreeSelector: string): Promise<ResolvedWorktree>
  notifyWorktreesChanged(repoId: string): void
  notifyReposChanged(): void
  invalidateResolvedWorktreeCache(): void
  validateLineageParent(child: ResolvedWorktree, parent: ResolvedWorktree): void
  getOrStartRemoteFetch(
    repoPath: string,
    remote: string,
    gitOptions?: { wslDistro?: string }
  ): Promise<RemoteFetchResult>
  fetchRemoteWithCache(
    repoPath: string,
    remote: string,
    gitOptions?: { wslDistro?: string }
  ): Promise<void>
  resolveRemoteTrackingBase(
    repoPath: string,
    baseBranch: string,
    gitOptions?: { wslDistro?: string }
  ): Promise<RemoteTrackingBase | null>
  getNotifier(): RuntimeWorktreeBaseNotifier | null
}

export class RuntimeWorktreeBaseStatusCommands {
  constructor(private readonly host: RuntimeWorktreeBaseStatusCommandHost) {}

  // Why: reconcileWorktreeBaseStatus is fire-and-forget from the caller's
  // perspective (started right after worktree creation) — the token lets a
  // stale reconcile started for a worktree that's since been removed/
  // recreated detect it's no longer current and stop emitting.
  private optimisticReconcileTokens = new Map<string, string>()

  recordOptimisticReconcileToken(worktreeId: string): string {
    const token = randomUUID()
    this.optimisticReconcileTokens.set(worktreeId, token)
    return token
  }

  clearOptimisticReconcileToken(worktreeId: string): void {
    this.optimisticReconcileTokens.delete(worktreeId)
  }

  emitWorktreeBaseStatus(event: WorktreeBaseStatusEvent): void {
    this.host.getNotifier()?.worktreeBaseStatus?.(event)
  }

  async reconcileWorktreeBaseStatus(args: {
    repoId: string
    repoPath: string
    worktreeId: string
    base: RemoteTrackingBase
    branchName: string
    createdBaseSha: string
    token: string
    fetchPromise: Promise<RemoteFetchResult>
  }): Promise<void> {
    const stillCurrent = (): boolean =>
      this.optimisticReconcileTokens.get(args.worktreeId) === args.token
    const emit = (event: Omit<WorktreeBaseStatusEvent, 'repoId' | 'worktreeId' | 'base'>): void => {
      if (!stillCurrent()) {
        return
      }
      this.host.getNotifier()?.worktreeBaseStatus?.({
        repoId: args.repoId,
        worktreeId: args.worktreeId,
        base: args.base.base,
        remote: args.base.remote,
        ...event
      })
    }
    const resolvePublishRemote = async (): Promise<string> => {
      // Why: repos whose canonical publish remote is named differently (e.g.
      // `upstream`, a forked `myfork`, or any non-`origin` configuration —
      // including multi-segment names like `foo/bar` that this PR's resolver
      // explicitly supports) would otherwise silently skip the conflict
      // signal. Resolve from git config in priority order:
      //   1) branch.<name>.pushRemote (explicit per-branch override)
      //   2) remote.pushDefault (workspace-wide override)
      //   3) branch.<name>.remote (tracked remote)
      //   4) the base ref's own remote (matches resolveRemoteTrackingBase)
      //   5) `origin` as a final fallback.
      const tryConfig = async (key: string): Promise<string | null> => {
        try {
          const { stdout } = await gitExecFileAsync(['config', '--get', key], {
            cwd: args.repoPath
          })
          const value = stdout.trim()
          return value || null
        } catch {
          return null
        }
      }
      return (
        (await tryConfig(`branch.${args.branchName}.pushRemote`)) ??
        (await tryConfig('remote.pushDefault')) ??
        (await tryConfig(`branch.${args.branchName}.remote`)) ??
        args.base.remote ??
        'origin'
      )
    }
    const checkPublishRemoteConflict = async (): Promise<void> => {
      const publishRemote = await resolvePublishRemote()
      try {
        if (publishRemote !== args.base.remote) {
          const result = await this.host.getOrStartRemoteFetch(args.repoPath, publishRemote)
          if (!result.ok) {
            return
          }
        }
        await gitExecFileAsync(
          ['rev-parse', '--verify', `refs/remotes/${publishRemote}/${args.branchName}^{commit}`],
          { cwd: args.repoPath }
        )
        if (stillCurrent()) {
          this.host.getNotifier()?.worktreeRemoteBranchConflict?.({
            repoId: args.repoId,
            worktreeId: args.worktreeId,
            remote: publishRemote,
            branchName: args.branchName
          })
        }
      } catch {
        // No publish-remote conflict is the common case; stay quiet.
      }
    }

    try {
      const fetchResult = await args.fetchPromise
      if (!stillCurrent()) {
        return
      }
      if (!fetchResult.ok) {
        emit({ status: 'unknown' })
        return
      }

      const { stdout } = await gitExecFileAsync(
        ['rev-parse', '--verify', `${args.base.ref}^{commit}`],
        { cwd: args.repoPath }
      )
      const postFetchSha = stdout.trim()
      if (postFetchSha === args.createdBaseSha) {
        emit({ status: 'current' })
        await checkPublishRemoteConflict()
        return
      }

      try {
        await gitExecFileAsync(['merge-base', '--is-ancestor', args.createdBaseSha, postFetchSha], {
          cwd: args.repoPath
        })
      } catch {
        emit({ status: 'base_changed' })
        await checkPublishRemoteConflict()
        return
      }

      const { stdout: countStdout } = await gitExecFileAsync(
        ['rev-list', '--count', `${args.createdBaseSha}..${postFetchSha}`],
        { cwd: args.repoPath }
      )
      const behind = Number(countStdout.trim())
      if (!Number.isFinite(behind) || behind <= 0) {
        emit({ status: 'current' })
        await checkPublishRemoteConflict()
        return
      }
      const { stdout: logStdout } = await gitExecFileAsync(
        ['log', '--format=%s', '-n', '5', `${args.createdBaseSha}..${postFetchSha}`],
        { cwd: args.repoPath }
      )
      emit({
        status: 'drift',
        behind,
        recentSubjects: logStdout.split('\n').filter((line) => line.trim().length > 0)
      })
      await checkPublishRemoteConflict()
    } catch (err) {
      console.warn(`[worktree-base-status] reconcile failed for ${args.worktreeId}:`, err)
      emit({ status: 'unknown' })
    } finally {
      // Why: reconcile is one-shot; clear the token so long-lived sessions
      // that create many worktrees without removing them don't grow the
      // optimisticReconcileTokens map monotonically. Removal still no-ops
      // because the entry is already gone.
      if (this.optimisticReconcileTokens.get(args.worktreeId) === args.token) {
        this.optimisticReconcileTokens.delete(args.worktreeId)
      }
    }
  }

  /**
   * Probe how far the worktree's HEAD is behind its tracking remote. Returns
   * null when the probe cannot establish a signal (no default base ref, or
   * git failure). Dispatch treats null as "unknown — proceed" (§3.1); only
   * knowing-and-stale refuses.
   */
  async probeWorktreeDrift(worktreeSelector: string): Promise<{
    base: string
    behind: number
    recentSubjects: string[]
  } | null> {
    const wt = await this.host.resolveWorktreeSelector(worktreeSelector)
    const store = this.host.getStore()
    if (!store) {
      return null
    }
    const repo = store.getRepos().find((r) => r.id === wt.repoId)
    if (!repo) {
      return null
    }
    if (repo.connectionId) {
      // Why: the drift probe uses local git helpers. Until the SSH provider
      // exposes equivalent remote refs/log plumbing, fail closed to "unknown"
      // instead of probing a server path on the desktop filesystem.
      return null
    }
    const localGitExecOptions = getLocalProjectGitExecOptions(this.host.requireStore(), repo)
    const localWorktreeGitOptions = getLocalProjectWorktreeGitOptions(
      this.host.requireStore(),
      repo
    )
    const meta = store.getWorktreeMeta(wt.id)
    const base =
      meta?.baseRef ||
      meta?.sparseBaseRef ||
      repo.worktreeBaseRef ||
      (await getBaseRefDefault(repo.path, localWorktreeGitOptions))
    if (!base) {
      // Why: brand-new repo with no remote primary — nothing to compare
      // against, so there's no meaningful drift to report. Dispatch should
      // not block on a probe that cannot form an opinion.
      return null
    }
    const remoteTrackingBase = await this.host.resolveRemoteTrackingBase(
      repo.path,
      base,
      localWorktreeGitOptions
    )
    if (!remoteTrackingBase) {
      return null
    }
    const remote = remoteTrackingBase.remote
    // Why: fetch failures are non-fatal; we proceed with whatever the
    // last-known remote ref points at. `fetchRemoteWithCache` never throws.
    await this.host.fetchRemoteWithCache(repo.path, remote, localWorktreeGitOptions)
    const drift = getRemoteDrift(wt.path, 'HEAD', base, localGitExecOptions)
    if (!drift) {
      return null
    }
    const recentSubjects = getRecentDriftSubjects(
      wt.path,
      'HEAD',
      base,
      DRIFT_PROBE_SUBJECT_LIMIT,
      localGitExecOptions
    )
    return { base, behind: drift.behind, recentSubjects }
  }

  async updateManagedWorktreeMeta(
    worktreeSelector: string,
    updates: Omit<Partial<WorktreeMeta>, 'pushTarget'> & {
      pushTarget?: GitPushTarget | null
      lineage?: {
        parentWorktree?: string
        noParent?: boolean
      }
    }
  ) {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    const worktree = await this.host.resolveWorktreeSelector(worktreeSelector)
    const { lineage, ...metaUpdates } = updates
    const shouldClearPushTarget =
      Object.prototype.hasOwnProperty.call(metaUpdates, 'pushTarget') &&
      metaUpdates.pushTarget === null
    const normalizedMetaUpdates: Partial<WorktreeMeta> = shouldClearPushTarget
      ? { ...metaUpdates, pushTarget: undefined }
      : (metaUpdates as Partial<WorktreeMeta>)
    const persistedMetaUpdates: Partial<WorktreeMeta> = omitUndefinedProperties(
      normalizedMetaUpdates.displayName !== undefined
        ? {
            ...normalizedMetaUpdates,
            pendingFirstAgentMessageRename: false,
            firstAgentMessageRenameError: null
          }
        : normalizedMetaUpdates
    )
    if (shouldClearPushTarget) {
      // Why: omitUndefinedProperties protects ordinary optional RPC fields, but
      // pushTarget:null is an explicit request to remove persisted target metadata.
      persistedMetaUpdates.pushTarget = undefined
    }
    if (lineage?.noParent === true) {
      store.removeWorktreeLineage?.(worktree.id)
      store.removeWorkspaceLineage?.(worktreeWorkspaceKey(worktree.id))
    } else if (lineage?.parentWorktree) {
      const parent = await this.host.resolveWorktreeSelector(lineage.parentWorktree)

      this.host.validateLineageParent(worktree, parent)
      if (!worktree.instanceId || !parent.instanceId) {
        throw new RuntimeLineageError(
          'LINEAGE_PARENT_CONTEXT_MISSING',
          'Worktree instance identity was unavailable.'
        )
      }
      if (!store.setWorktreeLineage) {
        throw new RuntimeLineageError(
          'LINEAGE_PARENT_CONTEXT_MISSING',
          'Worktree lineage storage was unavailable.'
        )
      }
      const createdAt = Date.now()
      store.setWorktreeLineage(worktree.id, {
        worktreeId: worktree.id,
        worktreeInstanceId: worktree.instanceId,
        parentWorktreeId: parent.id,
        parentWorktreeInstanceId: parent.instanceId,
        origin: 'manual',
        capture: { source: 'manual-action', confidence: 'explicit' },
        createdAt
      })
      store.setWorkspaceLineage?.({
        childWorkspaceKey: worktreeWorkspaceKey(worktree.id),
        childInstanceId: worktree.instanceId,
        parentWorkspaceKey: worktreeWorkspaceKey(parent.id),
        parentInstanceId: parent.instanceId,
        origin: 'manual',
        capture: { source: 'manual-action', confidence: 'explicit' },
        createdAt
      })
    }
    store.setWorktreeMeta(worktree.id, stripOrcaProvenanceMetaUpdates(persistedMetaUpdates))
    // Why: unlike renderer-initiated optimistic updates, CLI callers need an
    // explicit push so the editor refreshes metadata changed outside the UI.
    this.host.invalidateResolvedWorktreeCache()
    this.host.notifyWorktreesChanged(worktree.repoId)
    return await this.host.showManagedWorktree(`id:${worktree.id}`)
  }

  persistManagedWorktreeSortOrder(orderedIds: string[]): { updated: number } {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    const now = Date.now()
    let updated = 0
    for (let i = 0; i < orderedIds.length; i++) {
      store.setWorktreeMeta(orderedIds[i], { sortOrder: now - i * 1000 })
      updated++
    }
    this.host.invalidateResolvedWorktreeCache()
    this.host.notifyReposChanged()
    return { updated }
  }

  async resolveManagedPrBase(args: {
    repoSelector: string
    prNumber: number
    headRefName?: string
    baseRefName?: string
    isCrossRepository?: boolean
  }): Promise<GitHubPrStartPoint | { error: string }> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    let repo: Repo
    try {
      repo = await this.host.resolveRepoSelector(args.repoSelector)
    } catch {
      return { error: 'Repo not found' }
    }
    if (isFolderRepo(repo)) {
      return { error: 'Folder mode does not support creating worktrees.' }
    }
    const providerConnectionId = getRepoProviderConnectionKey(repo)
    const sshGitProvider = providerConnectionId
      ? requireRemoteGitProvider(providerConnectionId)
      : null
    const localGitExecOptions = sshGitProvider
      ? undefined
      : getLocalProjectGitExecOptions(this.host.requireStore(), repo)
    const localWorktreeGitOptions = sshGitProvider
      ? {}
      : getLocalProjectWorktreeGitOptions(this.host.requireStore(), repo)
    const gitExec = sshGitProvider
      ? (gitArgs: string[]) => sshGitProvider.exec(gitArgs, repo.path)
      : (gitArgs: string[]) => gitExecFileAsync(gitArgs, localGitExecOptions ?? { cwd: repo.path })
    const resolveRemote = sshGitProvider
      ? async () => {
          const { stdout } = await sshGitProvider.exec(['remote'], repo.path)
          const remotes = stdout
            .split('\n')
            .map((line) => line.trim())
            .filter(Boolean)
          if (remotes.includes('origin')) {
            return 'origin'
          }
          if (remotes.length === 1) {
            return remotes[0]!
          }
          if (remotes.length === 0) {
            throw new Error('Repo has no configured git remotes.')
          }
          throw new Error(
            `Repo has multiple remotes (${remotes.join(', ')}) and no default is configured.`
          )
        }
      : () => getDefaultRemote(repo.path, localWorktreeGitOptions)

    // Why: SSH repos can't fetch over the relay's read-only git.exec channel, so
    // route the PR head fetch through the write-capable helper instead of gitExec.
    const fetchRemoteTrackingRef = (remote: string, branch: string): Promise<void> =>
      fetchPrHeadTrackingRef(
        repo,
        sshGitProvider,
        remote,
        branch,
        localGitExecOptions ? { localGitExecOptions } : {}
      )

    return resolveGitHubPrStartPoint({
      repoPath: repo.path,
      prNumber: args.prNumber,
      headRefName: args.headRefName,
      baseRefName: args.baseRefName,
      isCrossRepository: args.isCrossRepository,
      connectionId: repo.connectionId ?? null,
      localGitOptions: localWorktreeGitOptions,
      gitExec,
      fetchRemoteTrackingRef,
      resolveRemote
    })
  }

  async resolveManagedMrBase(args: {
    repoSelector: string
    mrIid: number
    sourceBranch?: string
    targetBranch?: string
    isCrossRepository?: boolean
  }): Promise<
    { baseBranch: string; compareBaseRef?: string; pushTarget?: GitPushTarget } | { error: string }
  > {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    let repo: Repo
    try {
      repo = await this.host.resolveRepoSelector(args.repoSelector)
    } catch {
      return { error: 'Repo not found' }
    }
    if (isFolderRepo(repo)) {
      return { error: 'Folder mode does not support creating worktrees.' }
    }
    const providerConnectionId = getRepoProviderConnectionKey(repo)
    const sshGitProvider = providerConnectionId
      ? requireRemoteGitProvider(providerConnectionId)
      : null
    const localGitExecOptions = sshGitProvider
      ? undefined
      : getLocalProjectGitExecOptions(this.host.requireStore(), repo)
    const localWorktreeGitOptions = sshGitProvider
      ? {}
      : getLocalProjectWorktreeGitOptions(this.host.requireStore(), repo)
    const gitExec = sshGitProvider
      ? (gitArgs: string[]) => sshGitProvider.exec(gitArgs, repo.path)
      : (gitArgs: string[]) => gitExecFileAsync(gitArgs, localGitExecOptions ?? { cwd: repo.path })

    let sourceBranch = args.sourceBranch?.trim() ?? ''
    let targetBranch = args.targetBranch?.trim() ?? ''
    let isCrossRepository = args.isCrossRepository === true

    if (!sourceBranch) {
      let remote: string
      try {
        remote = await this.resolveGitLabIssueSourceRemote(
          repo.path,
          repo.issueSourcePreference,
          repo.connectionId ?? null,
          localWorktreeGitOptions
        )
      } catch (error) {
        return { error: error instanceof Error ? error.message : 'Could not resolve git remote.' }
      }
      const knownHosts = await getGlabKnownHosts(repo.connectionId ?? null)
      const projectRef = await getGitLabProjectRefForRemote(
        repo.path,
        remote,
        knownHosts,
        repo.connectionId ?? null,
        localWorktreeGitOptions
      )
      if (!projectRef) {
        return { error: 'No GitLab project found for this repository.' }
      }
      const item = await getGitLabWorkItemByProjectRef(
        repo.path,
        projectRef,
        args.mrIid,
        'mr',
        repo.connectionId ?? null,
        localWorktreeGitOptions
      )
      if (!item || item.type !== 'mr') {
        return { error: `MR !${args.mrIid} not found.` }
      }
      sourceBranch = (item.branchName ?? '').trim()
      targetBranch = (item.baseRefName ?? '').trim()
      if (!sourceBranch) {
        return { error: `MR !${args.mrIid} has no source branch.` }
      }
      if (item.isCrossRepository === true) {
        isCrossRepository = true
      }
    }

    let remote: string
    try {
      remote = await this.resolveGitLabIssueSourceRemote(
        repo.path,
        repo.issueSourcePreference,
        repo.connectionId ?? null,
        localWorktreeGitOptions
      )
    } catch (error) {
      return { error: error instanceof Error ? error.message : 'Could not resolve git remote.' }
    }
    const compareBaseRef = targetBranch ? `refs/remotes/${remote}/${targetBranch}` : undefined
    const fetchRemoteTrackingRef = async (branch: string, ref: string): Promise<void> => {
      await (sshGitProvider
        ? sshGitProvider.fetchRemoteTrackingRef!(repo.path, remote, branch, ref)
        : gitExec(['fetch', remote, `+refs/heads/${branch}:${ref}`]))
    }
    // Why: the target/compare branch is optional (it only powers the diff
    // base). A merged MR may have had its target ref deleted, so a fetch
    // failure must NOT abort the whole resolution — that would discard the
    // already-verified source-branch base and silently fall back to the repo
    // default branch. Degrade gracefully by dropping compareBaseRef instead.
    const fetchCompareBaseRef = async (): Promise<boolean> => {
      if (!targetBranch || !compareBaseRef) {
        return false
      }
      try {
        await fetchRemoteTrackingRef(targetBranch, compareBaseRef)
        return true
      } catch (error) {
        console.warn('[runtime:resolveManagedMrBase] optional compare-base fetch failed', {
          remote,
          targetBranch,
          mrIid: args.mrIid,
          error: error instanceof Error ? error.message.split('\n')[0] : String(error)
        })
        return false
      }
    }

    if (isCrossRepository) {
      const mrRef = `refs/merge-requests/${args.mrIid}/head`
      // Why: GitLab exposes fork MR heads on the target project, so mobile/SSH
      // can match desktop without adding the contributor fork as a remote.
      try {
        await (sshGitProvider
          ? sshGitProvider.fetchGitLabMergeRequestHead!(repo.path, remote, args.mrIid)
          : gitExec(['fetch', remote, mrRef]))
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error)
        return { error: `Failed to fetch ${mrRef}: ${message.split('\n')[0]}` }
      }
      let sha: string
      try {
        const { stdout } = await gitExec(['rev-parse', '--verify', 'FETCH_HEAD'])
        sha = stdout.trim()
      } catch {
        return { error: `Could not resolve fork MR !${args.mrIid} head after fetch.` }
      }
      if (!sha) {
        return { error: `Empty SHA resolving fork MR !${args.mrIid} head.` }
      }
      const compareBaseFetched = await fetchCompareBaseRef()
      return { baseBranch: sha, ...(compareBaseFetched ? { compareBaseRef } : {}) }
    }

    try {
      await fetchRemoteTrackingRef(sourceBranch, `refs/remotes/${remote}/${sourceBranch}`)
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      return { error: `Failed to fetch ${remote}/${sourceBranch}: ${message.split('\n')[0]}` }
    }

    const remoteRef = `${remote}/${sourceBranch}`
    try {
      await gitExec(['rev-parse', '--verify', remoteRef])
    } catch {
      return { error: `Remote ref ${remoteRef} does not exist after fetch.` }
    }
    const compareBaseFetched = await fetchCompareBaseRef()
    return {
      baseBranch: remoteRef,
      ...(compareBaseFetched ? { compareBaseRef } : {}),
      pushTarget: { remoteName: remote, branchName: sourceBranch }
    }
  }

  private async resolveGitLabIssueSourceRemote(
    repoPath: string,
    preference?: Repo['issueSourcePreference'],
    connectionId?: string | null,
    localGitOptions: { wslDistro?: string } = {}
  ): Promise<string> {
    const knownHosts = await getGlabKnownHosts(connectionId)
    const localGitOptionArgs =
      Object.keys(localGitOptions).length > 0 ? ([localGitOptions] as const) : []
    if (preference === 'origin') {
      const origin = await getGitLabProjectRefForRemote(
        repoPath,
        'origin',
        knownHosts,
        connectionId,
        ...localGitOptionArgs
      )
      if (origin) {
        return 'origin'
      }
      throw new Error('No GitLab project found for origin.')
    }
    if (preference === 'upstream') {
      const upstream = await getGitLabProjectRefForRemote(
        repoPath,
        'upstream',
        knownHosts,
        connectionId,
        ...localGitOptionArgs
      )
      if (upstream) {
        return 'upstream'
      }
      const origin = await getGitLabProjectRefForRemote(
        repoPath,
        'origin',
        knownHosts,
        connectionId,
        ...localGitOptionArgs
      )
      if (origin) {
        return 'origin'
      }
      throw new Error('No GitLab project found for upstream or origin.')
    }
    const upstream = await getGitLabProjectRefForRemote(
      repoPath,
      'upstream',
      knownHosts,
      connectionId,
      ...localGitOptionArgs
    )
    if (upstream) {
      return 'upstream'
    }
    const origin = await getGitLabProjectRefForRemote(
      repoPath,
      'origin',
      knownHosts,
      connectionId,
      ...localGitOptionArgs
    )
    if (origin) {
      return 'origin'
    }
    if (connectionId) {
      const provider = requireRemoteGitProvider(connectionId)
      const { stdout } = await provider.exec(['remote'], repoPath)
      const remotes = stdout
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean)
      if (remotes.includes('origin')) {
        return 'origin'
      }
      if (remotes.length === 1) {
        return remotes[0]!
      }
      if (remotes.length === 0) {
        throw new Error('Repo has no configured git remotes.')
      }
      throw new Error(
        `Repo has multiple remotes (${remotes.join(', ')}) and no default is configured.`
      )
    }
    return getDefaultRemote(repoPath, localGitOptions)
  }
}
