// frontend/src/main/runtime/orca-runtime-resolved-worktree-cache.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-040): resolved-worktree cache +
// lineage-attachment commands extracted from OrcaRuntimeService via the
// composition pattern (host interface, lazy closures — see
// orca-runtime-automation.ts for the established convention). More
// entangled than originally scoped: invalidateResolvedWorktreeCache alone
// has ~29 call sites scattered across the whole class, and the cache field
// itself is peeked at directly from 2 sites outside this block — both kept
// working via forwarded fields / a peekCache() accessor.
import { randomUUID } from 'node:crypto'
import type { GitWorktreeInfo, Repo, WorktreeLineage } from '../../shared/types'
import type { Store } from '../persistence'
import { isFolderRepo } from '../../shared/repo-kind'
import { getRepoProviderConnectionKey } from '../../shared/execution-host'
import {
  isWorkspaceKey,
  parseWorkspaceKey,
  worktreeWorkspaceKey
} from '../../shared/workspace-scope'
import { splitWorktreeId } from '../../shared/worktree-id'
import { areWorktreePathsEqual, mergeWorktree } from '../ipc/worktree-logic'
import { listRepoWorktrees } from '../repo-worktrees'
import { getLocalProjectWorktreeGitOptions } from '../project-runtime-git-options'
import { getRemoteGitProvider } from '../providers/ssh-git-dispatch'
import type { ResolvedWorktree, RuntimeStore, RuntimeWorktreeScanResult } from './orca-runtime'
import { listRuntimeFolderWorkspaces, withTimeout } from './orca-runtime'

const RESOLVED_WORKTREE_CACHE_TTL_MS = 1000
const RESOLVED_WORKTREE_REPO_TIMEOUT_MS = 5000

type ResolvedWorktreeCacheEntry = {
  expiresAt: number
  worktrees: ResolvedWorktree[]
}

type ResolvedWorktreeInFlight = {
  generation: number
  promise: Promise<ResolvedWorktree[]>
}

export type RuntimeResolvedWorktreeCommandHost = {
  getStore(): RuntimeStore | null
  requireStore(): Store
  notifyWorktreesChanged(repoId: string): void
  notifierWorktreesChanged(
    repoId: string,
    renamed?: { oldWorktreeId: string; newWorktreeId: string }
  ): void
  emitWorktreesChangedClientEvent(repoId: string): void
}

export class RuntimeResolvedWorktreeCommands {
  private resolvedWorktreeCache: ResolvedWorktreeCacheEntry | null = null
  private resolvedWorktreeInFlight: ResolvedWorktreeInFlight | null = null
  private resolvedWorktreeGeneration = 0

  constructor(private readonly host: RuntimeResolvedWorktreeCommandHost) {}

  // Why: 2 call sites in OrcaRuntimeService (listTerminals's fast-path
  // classification) peek at the cache snapshot directly without awaiting a
  // full resolve — kept working via this accessor instead of exposing the
  // private field itself.
  peekCache(): ResolvedWorktreeCacheEntry | null {
    return this.resolvedWorktreeCache
  }

  async listResolvedWorktrees(): Promise<ResolvedWorktree[]> {
    const store = this.host.getStore()
    if (!store) {
      return []
    }
    const now = Date.now()
    if (this.resolvedWorktreeCache && this.resolvedWorktreeCache.expiresAt > now) {
      return this.resolvedWorktreeCache.worktrees
    }
    const generation = this.resolvedWorktreeGeneration
    if (this.resolvedWorktreeInFlight?.generation === generation) {
      return this.resolvedWorktreeInFlight.promise
    }

    const promise = this.computeResolvedWorktrees(generation)
    this.resolvedWorktreeInFlight = { generation, promise }
    try {
      return await promise
    } finally {
      if (this.resolvedWorktreeInFlight?.promise === promise) {
        this.resolvedWorktreeInFlight = null
      }
    }
  }

  private async computeResolvedWorktrees(generation: number): Promise<ResolvedWorktree[]> {
    const store = this.host.getStore()
    if (!store) {
      return []
    }
    const now = Date.now()
    const metaById = store.getAllWorktreeMeta() ?? {}
    const perRepoWorktrees = await Promise.all(
      store.getRepos().map(async (repo) => {
        if (isFolderRepo(repo)) {
          return listRuntimeFolderWorkspaces(this.host.requireStore(), repo).map((worktree) => ({
            ...worktree,
            parentWorktreeId: null,
            childWorktreeIds: [],
            lineage: null,
            git: {
              path: worktree.path,
              head: worktree.head,
              branch: worktree.branch,
              isBare: worktree.isBare,
              isMainWorktree: worktree.isMainWorktree
            },
            displayName: worktree.displayName,
            comment: worktree.comment
          }))
        }
        // Why: mobile startup RPCs share this path. A slow repo scan should
        // degrade one repo's metadata, not block all terminal/session loading.
        const scan = await withTimeout(
          this.listRepoWorktreesForResolution(repo),
          RESOLVED_WORKTREE_REPO_TIMEOUT_MS,
          { ok: false, worktrees: [] }
        )
        const gitWorktrees = scan.worktrees
        if (scan.ok) {
          this.pruneLineageForMissingRepoWorktrees(repo, gitWorktrees)
        }
        return gitWorktrees.map((gitWorktree) => {
          const worktreeId = `${repo.id}::${gitWorktree.path}`
          // Why: lineage validation needs a durable instance ID even when the
          // runtime sees a workspace before the renderer's discovery-stamp path.
          const existingMeta = metaById[worktreeId]
          const meta =
            existingMeta && existingMeta.instanceId
              ? existingMeta
              : store.setWorktreeMeta(worktreeId, {})
          const merged = mergeWorktree(repo.id, gitWorktree, meta, repo.displayName)
          return {
            ...merged,
            parentWorktreeId: null,
            childWorktreeIds: [],
            lineage: null,
            git: {
              path: gitWorktree.path,
              head: gitWorktree.head,
              branch: gitWorktree.branch,
              isBare: gitWorktree.isBare,
              isMainWorktree: gitWorktree.isMainWorktree
            },
            displayName: merged.displayName,
            comment: merged.comment
          }
        })
      })
    )
    const worktrees = this.attachLineageToResolvedWorktrees(perRepoWorktrees.flat())
    // Why: terminal polling can be frequent, but git worktree state is still
    // allowed to change outside Orca. A short TTL avoids shelling out on every
    // read without pretending the cache is authoritative for long.
    if (generation === this.resolvedWorktreeGeneration) {
      this.resolvedWorktreeCache = {
        worktrees,
        expiresAt: now + RESOLVED_WORKTREE_CACHE_TTL_MS
      }
    }
    return worktrees
  }

  private attachLineageToResolvedWorktrees(worktrees: ResolvedWorktree[]): ResolvedWorktree[] {
    const lineageById = this.host.getStore()?.getAllWorktreeLineage?.() ?? {}
    const worktreeById = new Map(worktrees.map((worktree) => [worktree.id, worktree]))
    const validLineageByChildId = new Map<string, WorktreeLineage>()
    const childIdsByParentId = new Map<string, string[]>()

    for (const [childId, lineage] of Object.entries(lineageById)) {
      const child = worktreeById.get(childId)
      const parent = worktreeById.get(lineage.parentWorktreeId)
      if (
        !child ||
        !parent ||
        child.instanceId !== lineage.worktreeInstanceId ||
        parent.instanceId !== lineage.parentWorktreeInstanceId
      ) {
        // Why: worktree IDs are path-derived. Instance checks keep replacement
        // checkouts from appearing as children of stale same-path lineage.
        continue
      }
      validLineageByChildId.set(childId, lineage)
      const children = childIdsByParentId.get(lineage.parentWorktreeId) ?? []
      children.push(childId)
      childIdsByParentId.set(lineage.parentWorktreeId, children)
    }

    return worktrees.map((worktree) => {
      const lineage = validLineageByChildId.get(worktree.id) ?? null
      return {
        ...worktree,
        parentWorktreeId: lineage?.parentWorktreeId ?? null,
        childWorktreeIds: childIdsByParentId.get(worktree.id) ?? [],
        lineage
      }
    })
  }

  // Why: also called directly from OrcaRuntimeService's worktree-detection
  // path (outside this domain's own listResolvedWorktrees flow) — public,
  // not private, despite being an implementation detail of this class.
  pruneLineageForMissingRepoWorktrees(repo: Repo, gitWorktrees: GitWorktreeInfo[]): void {
    const store = this.host.getStore()
    if (
      !store ||
      typeof store.getAllWorktreeLineage !== 'function' ||
      typeof store.removeWorktreeLineage !== 'function'
    ) {
      return
    }
    const liveIds = new Set(gitWorktrees.map((worktree) => `${repo.id}::${worktree.path}`))
    const repoPrefix = `${repo.id}::`
    for (const childWorkspaceKey of Object.keys(store.getAllWorkspaceLineage?.() ?? {})) {
      const childScope = parseWorkspaceKey(childWorkspaceKey)
      if (
        childScope?.type === 'worktree' &&
        childScope.worktreeId.startsWith(repoPrefix) &&
        !liveIds.has(childScope.worktreeId)
      ) {
        if (isWorkspaceKey(childWorkspaceKey)) {
          store.removeWorkspaceLineage?.(childWorkspaceKey)
        }
      }
    }
    for (const [childId, lineage] of Object.entries(store.getAllWorktreeLineage())) {
      if (childId.startsWith(repoPrefix) && !liveIds.has(childId)) {
        // Why: runtime selector scans can be the only scan before a path is
        // reused. Once a successful scan proves the child is gone, stale
        // lineage must not survive into the replacement checkout.
        store.removeWorktreeLineage(childId)
        store.removeWorkspaceLineage?.(worktreeWorkspaceKey(childId))
      }
      if (
        lineage.parentWorktreeId.startsWith(repoPrefix) &&
        !liveIds.has(lineage.parentWorktreeId)
      ) {
        const parentMeta = store.getWorktreeMeta(lineage.parentWorktreeId)
        if (!parentMeta || parentMeta.instanceId === lineage.parentWorktreeInstanceId) {
          // Why: preserving child lineage powers the repair UI, but a missing
          // parent path only needs one fresh identity to keep same-path
          // replacement checkouts from validating old lineage.
          store.setWorktreeMeta(lineage.parentWorktreeId, { instanceId: randomUUID() })
        }
      }
    }
  }

  // Why: also called directly from OrcaRuntimeService's worktree-detection
  // path — public for the same reason as pruneLineageForMissingRepoWorktrees.
  async listRepoWorktreesForResolution(repo: Repo): Promise<RuntimeWorktreeScanResult> {
    const providerConnectionId = getRepoProviderConnectionKey(repo)
    if (!providerConnectionId) {
      return {
        ok: true,
        worktrees: await listRepoWorktrees(
          repo,
          getLocalProjectWorktreeGitOptions(this.host.requireStore(), repo)
        )
      }
    }
    const provider = getRemoteGitProvider(providerConnectionId)
    if (!provider) {
      return { ok: false, worktrees: this.listStoredSshWorktreesForResolution(repo) }
    }
    try {
      return { ok: true, worktrees: await provider.listWorktrees(repo.path) }
    } catch {
      return { ok: false, worktrees: this.listStoredSshWorktreesForResolution(repo) }
    }
  }

  private listStoredSshWorktreesForResolution(repo: Repo): GitWorktreeInfo[] {
    const store = this.host.getStore()
    if (!store) {
      return []
    }
    const byWorktreeId = new Map<string, GitWorktreeInfo>()
    for (const [worktreeId, meta] of Object.entries(store.getAllWorktreeMeta())) {
      const parsed = splitWorktreeId(worktreeId)
      if (!parsed || parsed.repoId !== repo.id) {
        continue
      }
      // Why: this mirrors desktop worktrees:list's disconnected-SSH fallback.
      // Web clients should keep showing persisted SSH worktrees while the
      // provider is reconnecting instead of dropping the repo to zero rows.
      byWorktreeId.set(worktreeId, {
        path: parsed.worktreePath,
        head: '',
        branch: '',
        isBare: false,
        isMainWorktree: areWorktreePathsEqual(parsed.worktreePath, repo.path),
        ...(meta.sparseDirectories !== undefined ||
        meta.sparseBaseRef !== undefined ||
        meta.sparsePresetId !== undefined
          ? { isSparse: true }
          : {})
      })
    }
    return [...byWorktreeId.values()]
  }

  async getResolvedWorktreeMap(): Promise<Map<string, ResolvedWorktree>> {
    return new Map((await this.listResolvedWorktrees()).map((worktree) => [worktree.id, worktree]))
  }

  invalidateResolvedWorktreeCache(): void {
    this.resolvedWorktreeGeneration += 1
    this.resolvedWorktreeCache = null
  }

  /** Invalidate the worktree cache and tell the renderer to re-list, after an
   *  out-of-band branch change (e.g. auto-rename-from-work) so the new branch
   *  name surfaces without waiting for the next ambient refresh. */
  notifyBranchRenamed(repoId: string): void {
    this.invalidateResolvedWorktreeCache()
    this.host.notifyWorktreesChanged(repoId)
  }

  /** Like {@link notifyBranchRenamed}, but carries the old->new worktree id so the
   *  renderer re-keys its worktree-scoped state instead of treating the id change
   *  (from a folder rename) as a deletion. Same channel = guaranteed ordering. */
  notifyWorktreeFolderRenamed(repoId: string, oldWorktreeId: string, newWorktreeId: string): void {
    this.invalidateResolvedWorktreeCache()
    this.host.notifierWorktreesChanged(repoId, { oldWorktreeId, newWorktreeId })
    // Mirror notifyBranchRenamed so in-process onClientEvent listeners also see the rename.
    this.host.emitWorktreesChangedClientEvent(repoId)
  }
}
