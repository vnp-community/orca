/**
 * Branch/worktree mutation operations extracted from git-handler.ts
 * (GitHandler.checkout/localBranches/abortRebase/abortMerge/rebaseFromBase/
 * discard/bulkDiscard) so the Dev Server WS agent's Part A dispatcher
 * (agent-git-handler-extended.ts) can re-export them without depending on
 * the GitHandler class.
 *
 * Why the cache-clearing calls are gone: see git-handler-staging-ops.ts —
 * Part A has no per-connection read cache to invalidate.
 */
import * as path from 'node:path'
import type { GitExec } from './git-handler-ops'
import { normalizeGitErrorMessage } from '../shared/git-remote-error'
import {
  removeSafeUntrackedDiscardTarget,
  removeSafeUntrackedDiscardTargets
} from '../shared/git-discard-path-safety'
import { resolveGitRemoteRebaseSource, type GitCommandRunner } from '../shared/git-rebase-source'

const BULK_CHUNK_SIZE = 100

function literalPathspec(filePath: string): string {
  return `:(literal)${filePath}`
}

function normalizeGitPathForCompare(filePath: string): string {
  return filePath.replace(/\\/g, '/').replace(/\/+$/, '')
}

function isTrackedPathSpec(filePath: string, trackedPaths: readonly string[]): boolean {
  const normalized = normalizeGitPathForCompare(filePath)
  return trackedPaths.some((trackedPath) => {
    const normalizedTracked = normalizeGitPathForCompare(trackedPath)
    return normalizedTracked === normalized || normalizedTracked.startsWith(`${normalized}/`)
  })
}

function assertInWorktree(worktreePath: string, filePath: string): string {
  const resolved = path.resolve(worktreePath, filePath)
  const rel = path.relative(path.resolve(worktreePath), resolved)
  // Why: empty rel or '.' means the path IS the worktree root — rm -rf would
  // delete the entire worktree. Reject along with parent-escaping paths.
  if (
    !rel ||
    rel === '.' ||
    rel === '..' ||
    rel.startsWith(`..${path.sep}`) ||
    path.isAbsolute(rel)
  ) {
    throw new Error(`Path "${filePath}" resolves outside the worktree`)
  }
  return resolved
}

async function cleanUntrackedPaths(
  git: GitExec,
  worktreePath: string,
  filePaths: readonly string[]
): Promise<void> {
  for (let i = 0; i < filePaths.length; i += BULK_CHUNK_SIZE) {
    const chunk = filePaths.slice(i, i + BULK_CHUNK_SIZE)
    if (chunk.length > 0) {
      // Why: Git pathspec cleanup avoids raw recursive deletion through symlinked parents.
      await git(['clean', '-ffdx', '--', ...chunk.map((p) => literalPathspec(p))], worktreePath)
    }
  }
}

export async function checkout(
  git: GitExec,
  params: Record<string, unknown>
): Promise<{ ok: true; branch: string }> {
  const worktreePath = params.worktreePath as string
  const branch = params.branch as string
  // Defense-in-depth: reject option-like branch tokens (the RPC schema also
  // validates, but this handler is reachable independently). The
  // `startsWith('-')` guard is what prevents flag injection; the trailing `--`
  // marks that no pathspecs follow so the token is treated as a branch ref.
  if (typeof branch !== 'string' || branch.length === 0 || branch.startsWith('-')) {
    throw new Error('invalid_branch_name')
  }
  await git(['checkout', branch, '--'], worktreePath)
  return { ok: true as const, branch }
}

export async function localBranches(
  git: GitExec,
  params: Record<string, unknown>
): Promise<{ current: string | null; branches: string[] }> {
  const worktreePath = params.worktreePath as string
  const { stdout } = await git(
    ['for-each-ref', '--format=%(HEAD)%09%(refname:short)', 'refs/heads/'],
    worktreePath
  )
  let current: string | null = null
  const branches: string[] = []
  for (const line of stdout.split('\n')) {
    if (line.length === 0) {
      continue
    }
    const [marker, name] = line.split('\t')
    if (!name) {
      continue
    }
    if (marker === '*') {
      current = name
    }
    branches.push(name)
  }
  branches.sort((a, b) => (a === current ? -1 : b === current ? 1 : 0))
  return { current, branches }
}

export async function abortMerge(git: GitExec, params: Record<string, unknown>): Promise<void> {
  const worktreePath = params.worktreePath as string
  await git(['merge', '--abort'], worktreePath)
}

export async function abortRebase(git: GitExec, params: Record<string, unknown>): Promise<void> {
  const worktreePath = params.worktreePath as string
  await git(['rebase', '--abort'], worktreePath)
}

export async function rebaseFromBase(
  git: GitExec,
  params: Record<string, unknown>
): Promise<void> {
  const worktreePath = params.worktreePath as string
  const baseRef = params.baseRef as string
  try {
    const source = await resolveGitRemoteRebaseSource(
      ((args) => git(args, worktreePath)) as GitCommandRunner,
      baseRef
    )
    await git(['pull', '--rebase', source.remoteName, source.branchName], worktreePath)
  } catch (error) {
    throw new Error(normalizeGitErrorMessage(error, 'pull'))
  }
}

export async function discard(git: GitExec, params: Record<string, unknown>): Promise<void> {
  const worktreePath = params.worktreePath as string
  const filePath = params.filePath as string
  assertInWorktree(worktreePath, filePath)

  let tracked = false
  try {
    await git(['ls-files', '--error-unmatch', '--', literalPathspec(filePath)], worktreePath)
    tracked = true
  } catch {
    // untracked
  }

  if (tracked) {
    await git(['restore', '--worktree', '--source=HEAD', '--', literalPathspec(filePath)], worktreePath)
    return
  }

  await removeSafeUntrackedDiscardTarget(worktreePath, filePath, (targetPath) =>
    cleanUntrackedPaths(git, worktreePath, [targetPath])
  )
}

export async function bulkDiscard(git: GitExec, params: Record<string, unknown>): Promise<void> {
  const worktreePath = params.worktreePath as string
  const filePaths = params.filePaths as string[]
  if (filePaths.length === 0) {
    return
  }
  for (const filePath of filePaths) {
    assertInWorktree(worktreePath, filePath)
  }

  const trackedPathSpecs: string[] = []
  for (let i = 0; i < filePaths.length; i += BULK_CHUNK_SIZE) {
    const chunk = filePaths.slice(i, i + BULK_CHUNK_SIZE)
    const { stdout } = await git(
      ['ls-files', '-z', '--', ...chunk.map((p) => literalPathspec(p))],
      worktreePath
    )
    // Why: selecting a tracked directory can make `ls-files -z` return
    // enough descendants for push(...split) to exceed the argument limit.
    for (const trackedPathSpec of stdout.split('\0')) {
      if (trackedPathSpec) {
        trackedPathSpecs.push(trackedPathSpec)
      }
    }
  }

  const trackedPaths = filePaths.filter((filePath) => isTrackedPathSpec(filePath, trackedPathSpecs))
  const untrackedPaths = filePaths.filter(
    (filePath) => !isTrackedPathSpec(filePath, trackedPathSpecs)
  )
  await removeSafeUntrackedDiscardTargets(
    worktreePath,
    untrackedPaths,
    (targetPaths) => cleanUntrackedPaths(git, worktreePath, targetPaths),
    async () => {
      for (let i = 0; i < trackedPaths.length; i += BULK_CHUNK_SIZE) {
        const chunk = trackedPaths.slice(i, i + BULK_CHUNK_SIZE)
        await git(
          ['restore', '--worktree', '--source=HEAD', '--', ...chunk.map((p) => literalPathspec(p))],
          worktreePath
        )
      }
    }
  )
}
