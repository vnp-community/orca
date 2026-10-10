import type { BaseRefSearchResult, GlobalSettings, Repo } from '../../../shared/types'
import { legacyBaseRefSearchResult } from '../../../shared/base-ref-search-result'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'
import { isRuntimeRepoRefSearchQueryWithinLimit } from './runtime-repo-search-bounds'
import { requireDevServerIdForWeb } from '@/lib/repo-host-binding'
import {
  getOrCreateDefaultProject,
  mergeRepoViewIntoRepo,
  repoDisplayNameFromUrl,
  type RemoteRepoView
} from '@/store/slices/repos'

export type RuntimeRepoBaseRefDefault = {
  defaultBaseRef: string | null
  remoteCount: number
}

export async function getRuntimeRepoBaseRefDefault(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  repoId: string
): Promise<RuntimeRepoBaseRefDefault> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.repos.getBaseRefDefault({ repoId })
  }
  return callRuntimeRpc<RuntimeRepoBaseRefDefault>(
    target,
    'repo.baseRefDefault',
    { repoId },
    { timeoutMs: 15_000 }
  )
}

export async function searchRuntimeRepoBaseRefs(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  repoId: string,
  query: string,
  limit: number
): Promise<string[]> {
  if (!isRuntimeRepoRefSearchQueryWithinLimit(query)) {
    return []
  }
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.repos.searchBaseRefs({ repoId, query, limit })
  }
  const result = await callRuntimeRpc<{ refs: string[]; truncated: boolean }>(
    target,
    'repo.searchRefs',
    { repoId, query, limit },
    { timeoutMs: 15_000 }
  )
  return result.refs
}

export async function searchRuntimeRepoBaseRefDetails(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  repoId: string,
  query: string,
  limit: number
): Promise<BaseRefSearchResult[]> {
  if (!isRuntimeRepoRefSearchQueryWithinLimit(query)) {
    return []
  }
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.repos.searchBaseRefDetails({ repoId, query, limit })
  }
  const result = await callRuntimeRpc<{
    refs: string[]
    refDetails?: BaseRefSearchResult[]
    truncated: boolean
  }>(target, 'repo.searchRefs', { repoId, query, limit }, { timeoutMs: 15_000 })
  return result.refDetails ?? result.refs.map(legacyBaseRefSearchResult)
}

// Why: web has no SSH connectionId to route through; the "remote path" is a path on the dev server
// the caller picked, so adding it is a normal repo.add that must carry that host.
export async function addRuntimeRepoRemote(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  args: {
    connectionId: string
    remotePath: string
    displayName?: string
    kind?: 'git' | 'folder'
    devServerId?: string
  }
): Promise<{ repo: Repo } | { error: string }> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.repos.addRemote(args)
  }
  const devServerId = requireDevServerIdForWeb(args.devServerId)
  // Why the Go repo.add shape: it decodes {projectId, url, displayName, devServerId}; the old
  // {path, kind} + repo.update pair was ignored and never registered a repo.
  const projectId = await getOrCreateDefaultProject(target, devServerId)
  const view = await callRuntimeRpc<RemoteRepoView>(
    target,
    'repo.add',
    {
      projectId,
      url: args.remotePath,
      displayName: args.displayName || repoDisplayNameFromUrl(args.remotePath),
      ...(devServerId ? { devServerId } : {})
    },
    { timeoutMs: 30_000 }
  )
  return { repo: { ...mergeRepoViewIntoRepo(view), ...(args.kind ? { kind: args.kind } : {}) } }
}
