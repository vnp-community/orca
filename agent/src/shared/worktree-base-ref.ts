export type WorktreeStartPointProbe = {
  /** Full commit SHA that `revision` resolves to, or null when it names no commit. */
  resolveCommit: (revision: string) => Promise<string | null>
  /** Names of the configured remotes (`git remote`). */
  listRemotes: () => Promise<string[]>
}

export type WorktreeBaseRefErrorCode = 'INVALID' | 'AMBIGUOUS' | 'NOT_FOUND'

export class WorktreeBaseRefError extends Error {
  constructor(
    readonly code: WorktreeBaseRefErrorCode,
    message: string
  ) {
    super(message)
    this.name = 'WorktreeBaseRefError'
  }
}

/**
 * Resolves the start point of `git worktree add -b <branch> <path> <start>` to
 * an unambiguous revision before git sees it.
 *
 * Why: git's DWIM turns a bare name that only exists as `origin/<name>` into a
 * new local branch `<name>` and checks THAT out, silently ignoring the `-b`
 * branch. Precedence: local branch, remote-tracking branch, tag, commit-ish.
 */
export async function resolveWorktreeStartPoint(
  baseRef: string,
  probe: WorktreeStartPointProbe
): Promise<string> {
  // Why: checked before any git call; a leading '-' would be parsed as a flag.
  if (!baseRef || baseRef.startsWith('-')) {
    throw new WorktreeBaseRefError('INVALID', 'Base ref must not be empty or start with "-"')
  }
  if (baseRef.startsWith('refs/')) {
    return baseRef
  }

  const exists = async (qualifiedRef: string): Promise<boolean> =>
    (await probe.resolveCommit(qualifiedRef)) !== null

  const localBranch = `refs/heads/${baseRef}`
  if (await exists(localBranch)) {
    return localBranch
  }

  const remotes = await probe.listRemotes()
  // Why: Orca's base picker shows remote branches as `<remote>/<branch>`.
  if (remotes.some((remote) => baseRef.startsWith(`${remote}/`))) {
    const remoteTracking = `refs/remotes/${baseRef}`
    if (await exists(remoteTracking)) {
      return remoteTracking
    }
  }

  const originTracking = `refs/remotes/origin/${baseRef}`
  if (await exists(originTracking)) {
    return originTracking
  }

  const otherRemoteMatches: string[] = []
  for (const remote of remotes) {
    if (remote !== 'origin' && (await exists(`refs/remotes/${remote}/${baseRef}`))) {
      otherRemoteMatches.push(remote)
    }
  }
  if (otherRemoteMatches.length > 1) {
    // Why: picking one remote's branch silently could base the task on the wrong code.
    throw new WorktreeBaseRefError(
      'AMBIGUOUS',
      `Base ref "${baseRef}" is ambiguous: it exists on remotes ${otherRemoteMatches.join(', ')}; ` +
        `use "${otherRemoteMatches[0]}/${baseRef}"`
    )
  }
  if (otherRemoteMatches.length === 1) {
    return `refs/remotes/${otherRemoteMatches[0]}/${baseRef}`
  }

  const tag = `refs/tags/${baseRef}`
  if (await exists(tag)) {
    return tag
  }

  // Why: a verified SHA is passed on so git cannot re-run name DWIM on it.
  const commit = await probe.resolveCommit(baseRef)
  if (commit) {
    return commit
  }

  // Why: wording kept in sync with git-gateway-service isBaseRefNotFoundErr.
  throw new WorktreeBaseRefError(
    'NOT_FOUND',
    `invalid reference: "${baseRef}" is not a local branch, remote-tracking branch, tag or commit`
  )
}
