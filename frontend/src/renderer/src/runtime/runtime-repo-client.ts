import type { BaseRefSearchResult, GlobalSettings, Repo } from '../../../shared/types'
import { legacyBaseRefSearchResult } from '../../../shared/base-ref-search-result'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'
import { isRuntimeRepoRefSearchQueryWithinLimit } from './runtime-repo-search-bounds'

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
    { repo: repoId },
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
    { repo: repoId, query, limit },
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
  }>(target, 'repo.searchRefs', { repo: repoId, query, limit }, { timeoutMs: 15_000 })
  return result.refDetails ?? result.refs.map(legacyBaseRefSearchResult)
}

// Why: on web there is no local SSH connectionId to route through — the
// "remote path" is already a path on the single paired runtime host, so
// adding it is identical to a normal `repo.add`. Mirrors
// web-preload-api.ts's createReposApi().addRemote, which drops connectionId
// the same way.
export async function addRuntimeRepoRemote(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  args: { connectionId: string; remotePath: string; displayName?: string; kind?: 'git' | 'folder' }
): Promise<{ repo: Repo } | { error: string }> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.repos.addRemote(args)
  }
  const result = await callRuntimeRpc<{ repo: Repo }>(
    target,
    'repo.add',
    { path: args.remotePath, kind: args.kind },
    { timeoutMs: 30_000 }
  )
  if (!args.displayName) {
    return result
  }
  const updated = await callRuntimeRpc<{ repo: Repo }>(
    target,
    'repo.update',
    { repo: result.repo.id, updates: { displayName: args.displayName } },
    { timeoutMs: 15_000 }
  )
  return updated
}
