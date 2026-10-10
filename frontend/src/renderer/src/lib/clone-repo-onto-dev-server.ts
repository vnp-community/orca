import type { Repo } from '../../../shared/types'
import { callRuntimeRpc, type getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import {
  getOrCreateDefaultProject,
  mergeRepoViewIntoRepo,
  repoDisplayNameFromUrl,
  type RemoteRepoView
} from '@/store/slices/repos'

type EnvironmentTarget = Extract<ReturnType<typeof getActiveRuntimeTarget>, { kind: 'environment' }>

/**
 * Clone on a dev server, then register the checkout there. repo.clone only places files (it reads
 * {devServerId, url, destPath}); repo.add writes the catalog row and must carry the same host,
 * otherwise git-gateway would later run git inside its own container.
 */
export async function cloneRepoOntoDevServer(
  target: EnvironmentTarget,
  args: { devServerId: string | null; url: string; destPath: string }
): Promise<Repo> {
  const devServerId = args.devServerId?.trim() || null
  const cloned = await callRuntimeRpc<{ worktreePath?: string; defaultBranch?: string }>(
    target,
    'repo.clone',
    { devServerId: devServerId ?? '', url: args.url, destPath: args.destPath },
    { timeoutMs: 10 * 60_000 }
  )
  const projectId = await getOrCreateDefaultProject(target, devServerId)
  const view = await callRuntimeRpc<RemoteRepoView>(
    target,
    'repo.add',
    {
      projectId,
      url: cloned?.worktreePath || args.destPath,
      displayName: repoDisplayNameFromUrl(args.url),
      ...(devServerId ? { devServerId } : {})
    },
    { timeoutMs: 15_000 }
  )
  return mergeRepoViewIntoRepo(view)
}
