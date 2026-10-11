import { normalizeRuntimePathForComparison } from '../../../shared/cross-platform-path'
import { translate } from '@/i18n/i18n'

export const REPO_ALREADY_REGISTERED_CODE = 'PROJECT_REPO_ALREADY_REGISTERED'

type RepoLike = {
  id: string
  projectId?: string | null
  devServerId?: string | null
  /** Local-store repos carry `path`, wire views carry `url`. */
  path?: string | null
  url?: string | null
}

/** Same folder on the same host compares equal regardless of trailing or Windows-style separators. */
export function normalizeRepoPathKey(path: string): string {
  return normalizeRuntimePathForComparison(path.trim())
}

export function findRegisteredRepo<T extends RepoLike>(
  repos: readonly T[],
  query: { devServerId: string; path: string }
): T | undefined {
  const wanted = normalizeRepoPathKey(query.path)
  const devServerId = query.devServerId.trim()
  if (!wanted || !devServerId) {
    return undefined
  }
  return repos.find((repo) => {
    const repoPath = repo.path ?? repo.url ?? ''
    return (
      repo.devServerId?.trim() === devServerId &&
      repoPath !== '' &&
      normalizeRepoPathKey(repoPath) === wanted
    )
  })
}

/** The folder is already a repo of some project on that host; users are pointed at that project. */
export class RepoAlreadyRegisteredError extends Error {
  readonly code = REPO_ALREADY_REGISTERED_CODE

  constructor(
    readonly projectId: string,
    readonly projectName: string,
    readonly repoId?: string
  ) {
    super(
      projectName
        ? translate(
            'auto.lib.registeredRepoLookup.alreadyInProject',
            'This folder is already added in project "{{project}}" on this dev server.',
            { project: projectName }
          )
        : translate(
            'auto.lib.registeredRepoLookup.alreadyAdded',
            'This folder is already added on this dev server.'
          )
    )
    this.name = 'RepoAlreadyRegisteredError'
  }
}

// Backend text: `repo <path> on dev server <host> is already registered in project "<name>" (<id>)`,
// or `... in another project` when the caller may not see that project (no name, no id).
const ALREADY_IN_PROJECT = /already (?:registered )?in project "?(.+?)"? \(([^()\s]+)\)/

/** Maps the backend duplicate error (code PROJECT_REPO_ALREADY_REGISTERED) to a typed error. */
export function parseRepoAlreadyRegistered(error: unknown): RepoAlreadyRegisteredError | null {
  if (error instanceof RepoAlreadyRegisteredError) {
    return error
  }
  const message = error instanceof Error ? error.message : typeof error === 'string' ? error : ''
  // Why both: the gateway puts the code in the message, RuntimeRpcCallError also exposes it as `code`.
  const code = error instanceof Error ? (error as { code?: unknown }).code : undefined
  if (code !== REPO_ALREADY_REGISTERED_CODE && !message.includes(REPO_ALREADY_REGISTERED_CODE)) {
    return null
  }
  const match = ALREADY_IN_PROJECT.exec(message)
  return new RepoAlreadyRegisteredError(match?.[2] ?? '', match?.[1] ?? '')
}
