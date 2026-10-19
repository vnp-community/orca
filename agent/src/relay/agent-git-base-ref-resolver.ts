// src/relay/agent-git-base-ref-resolver.ts
// Default-branch resolution for git.baseRefDefault. Mirrors
// backend-go/services/git-gateway-service/internal/adapter/localgit/base_ref_default.go
// so the local and relay host paths agree. Order:
//   (a) refs/remotes/origin/HEAD target (set by `git clone`), if that branch exists
//   (b) `git ls-remote --symref origin HEAD` (repos from `git init` + `remote add`)
//   (c) preferredBranch (the project's configured default) when it is a local branch
//   (d) a conventional local `main`, then `master`
//   (e) the checked-out branch, only for a fresh repo with none of the above
// (c)/(d) keep a local-only repo from inheriting whatever feature branch is
// checked out: a Request without base_ref must start from the default branch.

import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

const execAsync = promisify(execFile)
const GIT_TIMEOUT_MS = 10_000

async function git(cwd: string, args: string[]): Promise<string> {
  const { stdout } = await execAsync('git', args, { cwd, timeout: GIT_TIMEOUT_MS })
  return stdout
}

// Strip only the known ref prefixes so slashed names survive: both
// refs/heads/feature/x and refs/remotes/origin/feature/x give "feature/x".
// Remote names never contain "/", so the first segment after refs/remotes/ is the remote.
export function branchFromRef(ref: string): string {
  const trimmed = ref.trim()
  if (trimmed.startsWith('refs/heads/')) {
    return trimmed.slice('refs/heads/'.length)
  }
  if (trimmed.startsWith('refs/remotes/')) {
    const rest = trimmed.slice('refs/remotes/'.length)
    const slash = rest.indexOf('/')
    return slash < 0 ? '' : rest.slice(slash + 1)
  }
  return trimmed
}

// A fully-qualified ref with --verify can never be read as an option.
async function refExists(cwd: string, fullRef: string): Promise<boolean> {
  try {
    await git(cwd, ['show-ref', '--verify', '--quiet', fullRef])
    return true
  } catch {
    return false
  }
}

const localBranchExists = (cwd: string, branch: string): Promise<boolean> =>
  refExists(cwd, `refs/heads/${branch}`)

async function branchExists(cwd: string, branch: string): Promise<boolean> {
  return (
    (await localBranchExists(cwd, branch)) ||
    (await refExists(cwd, `refs/remotes/origin/${branch}`))
  )
}

// `ref: refs/heads/main\tHEAD` — a repo with no origin/HEAD (never set by
// `git init` + `remote add`) still has a remote that knows its own HEAD.
async function defaultBranchFromRemoteHead(cwd: string): Promise<string> {
  const stdout = await git(cwd, ['ls-remote', '--symref', 'origin', 'HEAD'])
  const prefix = 'ref: refs/heads/'
  for (const line of stdout.split('\n')) {
    if (!line.startsWith(prefix)) {
      continue
    }
    const rest = line.slice(prefix.length).split('\t')[0]?.trim()
    if (rest) {
      return rest
    }
  }
  return ''
}

export async function resolveBaseRefDefault(cwd: string, preferredBranch: string): Promise<string> {
  let originHeadError: unknown = null
  try {
    const branch = branchFromRef(await git(cwd, ['symbolic-ref', 'refs/remotes/origin/HEAD']))
    if (branch && (await branchExists(cwd, branch))) {
      return branch
    }
  } catch (err: unknown) {
    originHeadError = err
  }
  try {
    const branch = await defaultBranchFromRemoteHead(cwd)
    if (branch) {
      return branch
    }
  } catch {
    /* no origin, unreachable or empty remote: try the local candidates */
  }
  const preferred = branchFromRef(preferredBranch)
  if (preferred && (await localBranchExists(cwd, preferred))) {
    return preferred
  }
  for (const conventional of ['main', 'master']) {
    if (await localBranchExists(cwd, conventional)) {
      return conventional
    }
  }
  try {
    // Resolves with zero commits; fails on a detached HEAD, which has no branch name.
    const current = branchFromRef(await git(cwd, ['symbolic-ref', 'HEAD']))
    if (current) {
      return current
    }
  } catch (err: unknown) {
    originHeadError ??= err
  }
  throw originHeadError ?? new Error('repository has no resolvable default branch')
}
