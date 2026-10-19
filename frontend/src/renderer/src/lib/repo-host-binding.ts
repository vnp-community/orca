import { parseExecutionHostId, type ExecutionHostId } from '../../../shared/execution-host'
import type { Repo } from '../../../shared/types'
import { translate } from '@/i18n/i18n'
import { isWebClientLocation } from '@/lib/web-client-location'

export const REPO_HOST_REQUIRED_CODE = 'REPO_HOST_REQUIRED'

export type RepoHostBinding =
  | { kind: 'devServer'; devServerId: string }
  | { kind: 'ssh'; connectionId: string }
  | { kind: 'runtime'; environmentId: string }
  | { kind: 'local' }

/** Web has no filesystem of its own, so a repo added there without a dev server cannot run git. */
export class RepoHostRequiredError extends Error {
  readonly code = REPO_HOST_REQUIRED_CODE

  constructor() {
    super(
      translate(
        'auto.lib.repoHostBinding.hostRequired',
        'Choose a dev server for this project first.'
      )
    )
    this.name = 'RepoHostRequiredError'
  }
}

export function resolveRepoHostBinding(
  hostId: ExecutionHostId | string | null | undefined
): RepoHostBinding {
  const parsed = parseExecutionHostId(hostId)
  switch (parsed?.kind) {
    case 'devServer':
      return { kind: 'devServer', devServerId: parsed.devServerId }
    case 'ssh':
      return { kind: 'ssh', connectionId: parsed.targetId }
    case 'runtime':
      return { kind: 'runtime', environmentId: parsed.environmentId }
    default:
      return { kind: 'local' }
  }
}

/** Throws before any RPC so a missing host never leaves an orphan project behind. */
export function requireRepoDevServerId(devServerId: string | null | undefined): string {
  const trimmed = devServerId?.trim()
  if (!trimmed) {
    throw new RepoHostRequiredError()
  }
  return trimmed
}

/** Store-level variant: web must carry a dev server, desktop keeps its local/SSH/runtime routing. */
export function requireDevServerIdForWeb(
  devServerId: string | null | undefined
): string | undefined {
  return isWebClientLocation()
    ? requireRepoDevServerId(devServerId)
    : devServerId?.trim() || undefined
}

/** A web repo with neither a dev server nor an SSH connection: git-gateway has nowhere to run git for it. */
export function isUnboundWebRepo(
  repo: Pick<Repo, 'devServerId' | 'connectionId'> | null | undefined
): boolean {
  if (!repo || !isWebClientLocation()) {
    return false
  }
  return !repo.devServerId?.trim() && !repo.connectionId?.trim()
}

type ProjectWithHost = { id: string; createdAt: number; devServerId?: string | null }

/** The caller's earliest project that is bound to this dev server, if any. */
export function pickProjectBoundToDevServer<T extends ProjectWithHost>(
  projects: readonly T[],
  devServerId: string
): T | undefined {
  return projects
    .filter((project) => project.devServerId?.trim() === devServerId)
    .sort((a, b) => a.createdAt - b.createdAt)[0]
}

/**
 * Project that holds repos added from one dev server: reuse the caller's project bound to it, else
 * create one. Each repo still carries devServerId itself, so if an older project-service cannot
 * create a host-bound project, adding degrades to the earliest project instead of blocking.
 */
export async function resolveProjectIdForDevServer(args: {
  devServerId: string
  listProjects: () => Promise<ProjectWithHost[]>
  createProject: (devServerId: string) => Promise<{ id: string }>
}): Promise<string> {
  const projects = (await args.listProjects()) ?? []
  const bound = pickProjectBoundToDevServer(projects, args.devServerId)
  if (bound) {
    return bound.id
  }
  try {
    return (await args.createProject(args.devServerId)).id
  } catch (err) {
    const earliest = projects.slice().sort((a, b) => a.createdAt - b.createdAt)[0]
    if (!earliest) {
      throw err
    }
    console.warn(
      '[repo-host-binding] host-bound project.create failed; using earliest project',
      err
    )
    return earliest.id
  }
}
