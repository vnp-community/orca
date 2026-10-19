import { getRuntimePathBasename } from '../../../shared/cross-platform-path'
import { translate } from '@/i18n/i18n'
import {
  findRegisteredRepo,
  parseRepoAlreadyRegistered,
  RepoAlreadyRegisteredError
} from './registered-repo-lookup'

/** Wire view of a repo row (repo.add / repo.list); structurally compatible with the store's copy. */
export type RegisteredRepoView = {
  id: string
  projectId: string
  url: string
  displayName: string
  position: number
  devServerId?: string
  hookSettings?: string
}

type ProjectRow = { id: string; name: string }

/** One RPC transport: the web preload and the store's runtime target both fit behind it. */
export type ProjectRpc = <TResult>(
  method: string,
  params?: unknown,
  timeoutMs?: number
) => Promise<TResult>

const PROJECT_CREATE_TIMEOUT_MS = 30_000
const CATALOG_TIMEOUT_MS = 15_000

export function deriveProjectNameFromPath(path: string, fallback: string): string {
  const base = getRuntimePathBasename(path.trim()).replace(/\.git$/, '')
  return base || fallback
}

async function listReposOfProject(
  rpc: ProjectRpc,
  projectId: string
): Promise<RegisteredRepoView[]> {
  try {
    const result = await rpc<{ repos?: RegisteredRepoView[] }>(
      'repo.list',
      { projectId },
      CATALOG_TIMEOUT_MS
    )
    return result?.repos ?? []
  } catch {
    // Why tolerated: one unreadable project must not block adding; the backend still guards duplicates.
    return []
  }
}

/** Looks the folder up across EVERY project the caller can see, not only the local store. */
export async function findRegisteredRepoRemote(
  rpc: ProjectRpc,
  query: { devServerId: string; path: string }
): Promise<{ repo: RegisteredRepoView; projectName: string } | null> {
  const projects = (await rpc<ProjectRow[] | null>('project.list', null, CATALOG_TIMEOUT_MS)) ?? []
  const perProject = await Promise.all(
    projects.map(async (project) => ({
      project,
      repos: await listReposOfProject(rpc, project.id)
    }))
  )
  for (const { project, repos } of perProject) {
    const repo = findRegisteredRepo(repos, query)
    if (repo) {
      return { repo, projectName: project.name }
    }
  }
  return null
}

/**
 * Add Project = one new project holding exactly this repo, bound to the chosen dev server.
 * Why: Requests run on the project's first repo, so sharing a host-wide project picks the wrong one.
 */
export async function registerRepoInOwnProject(
  rpc: ProjectRpc,
  args: { devServerId: string; path: string; projectName?: string }
): Promise<RegisteredRepoView> {
  const path = args.path.trim()
  const name = args.projectName?.trim() || deriveProjectNameFromPath(path, args.devServerId)

  const existing = await findRegisteredRepoRemote(rpc, { devServerId: args.devServerId, path })
  if (existing) {
    throw new RepoAlreadyRegisteredError(
      existing.repo.projectId,
      existing.projectName,
      existing.repo.id
    )
  }

  let project: { id: string }
  try {
    project = await rpc<{ id: string }>(
      'project.create',
      { name, devServerId: args.devServerId, repoPath: path, visibility: 'private' },
      PROJECT_CREATE_TIMEOUT_MS
    )
  } catch (err) {
    throw parseRepoAlreadyRegistered(err) ?? err
  }

  const created = findRegisteredRepo(await listReposOfProject(rpc, project.id), {
    devServerId: args.devServerId,
    path
  })
  if (created) {
    return created
  }
  // Why: an older project-service attaches the repo best-effort and may skip it silently.
  try {
    return await rpc<RegisteredRepoView>(
      'repo.add',
      { projectId: project.id, url: path, displayName: name, devServerId: args.devServerId },
      CATALOG_TIMEOUT_MS
    )
  } catch (err) {
    const duplicate = parseRepoAlreadyRegistered(err)
    if (duplicate) {
      throw duplicate
    }
    throw new Error(
      translate(
        'auto.lib.ownProjectRegistration.repoAddFailed',
        'Project "{{name}}" was created, but its repo could not be added: {{message}}',
        { name, message: err instanceof Error ? err.message : String(err) }
      )
    )
  }
}
