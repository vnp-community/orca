import { useAppStore } from '@/store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import { findRegisteredRepo } from '@/lib/registered-repo-lookup'

export type RegisteredRepoMatch = { repoId: string; projectId: string; projectName: string }

/**
 * Add Project pre-check on the repos already loaded in the store (every project the caller sees):
 * a hit blocks the add before any write. The backend and registerRepoInOwnProject re-check.
 */
export async function lookupRegisteredRepoForAdd(args: {
  devServerId: string
  path: string
}): Promise<RegisteredRepoMatch | null> {
  const repo = findRegisteredRepo(useAppStore.getState().repos, args)
  if (!repo) {
    return null
  }
  const projectId = repo.projectId ?? ''
  let projectName = repo.displayName
  if (projectId) {
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const projects = await callRuntimeRpc<{ id: string; name: string }[] | null>(
        target,
        'project.list',
        null,
        { timeoutMs: 15_000 }
      )
      projectName = projects?.find((project) => project.id === projectId)?.name || projectName
    } catch {
      // Why tolerated: the name is a courtesy; the repo's own name still identifies the match.
    }
  }
  return { repoId: repo.id, projectId, projectName }
}
