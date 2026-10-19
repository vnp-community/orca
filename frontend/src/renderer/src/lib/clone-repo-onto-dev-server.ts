import type { Repo } from '../../../shared/types'
import { registerRepoInOwnProject } from '@/lib/own-project-registration'
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
  args: { devServerId: string | null; url: string; destPath: string; projectName?: string }
): Promise<Repo> {
  const devServerId = args.devServerId?.trim() || null
  const cloned = await callRuntimeRpc<{ worktreePath?: string; defaultBranch?: string }>(
    target,
    'repo.clone',
    { devServerId: devServerId ?? '', url: args.url, destPath: args.destPath },
    { timeoutMs: 10 * 60_000 }
  )
  const registeredPath = cloned?.worktreePath || args.destPath
  // Why projectName: Add Project gives each repo its own project; without it keep the host default.
  if (devServerId && args.projectName !== undefined) {
    return mergeRepoViewIntoRepo(
      await registerRepoInOwnProject(
        (method, params, timeoutMs) => callRuntimeRpc(target, method, params, { timeoutMs }),
        { devServerId, path: registeredPath, projectName: args.projectName }
      )
    )
  }
  const projectId = await getOrCreateDefaultProject(target, devServerId)
  const view = await callRuntimeRpc<RemoteRepoView>(
    target,
    'repo.add',
    {
      projectId,
      url: registeredPath,
      displayName: repoDisplayNameFromUrl(args.url),
      ...(devServerId ? { devServerId } : {})
    },
    { timeoutMs: 15_000 }
  )
  return mergeRepoViewIntoRepo(view)
}
