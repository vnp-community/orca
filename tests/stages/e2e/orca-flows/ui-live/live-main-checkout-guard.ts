/**
 * The git side of the guard for the user's repo (rules: live-main-checkout-rules.ts): capture its state, and remove what a run
 * added (task worktrees, then their branches) once the run is over.
 */
import { execFileSync } from 'node:child_process'
import { existsSync, readdirSync, realpathSync, rmSync } from 'node:fs'
import { dirname } from 'node:path'
import { growthSince, type CheckoutState } from './live-main-checkout-rules'
import { fingerprintRepo, type RepoFingerprint } from './live-safety-invariants'

function git(repo: string, args: string[]): string {
  return execFileSync('git', ['-C', repo, ...args], {
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
    stdio: ['ignore', 'pipe', 'ignore']
  })
}

function listedWorktrees(repo: string): string[] {
  return git(repo, ['worktree', 'list', '--porcelain'])
    .split('\n')
    .filter((l) => l.startsWith('worktree '))
    .map((l) => l.slice('worktree '.length))
}

function refsOf(repo: string): Record<string, string> {
  const refs: Record<string, string> = {}
  for (const line of git(repo, ['for-each-ref', '--format=%(refname) %(objectname)'])
    .split('\n')
    .filter(Boolean)) {
    const [name, sha] = line.split(' ')
    refs[name] = sha
  }
  return refs
}

/** `cheap` hashes size+mtime instead of content (the watchdog polls every few seconds). */
export function captureCheckout(repoPath: string, cheap = false): CheckoutState {
  const repo = realpathSync(repoPath)
  const worktrees: Record<string, RepoFingerprint> = {}
  for (const w of listedWorktrees(repo)) {
    const real = existsSync(w) ? realpathSync(w) : w
    if (real !== repo) {
      worktrees[real] = existsSync(real)
        ? fingerprintRepo(real, cheap, [])
        : { ...fingerprintRepo(repo, true, []), head: 'missing', files: {} }
    }
  }
  return {
    repo,
    parent: dirname(repo),
    main: fingerprintRepo(repo, cheap, []),
    refs: refsOf(repo),
    worktrees,
    neighbours: readdirSync(dirname(repo)).sort()
  }
}

/**
 * Removes what the run added (task worktrees, then their branches) and returns what it did. Only things that did not exist in
 * `before` are touched, and a branch only while no worktree has it checked out any more.
 */
export function removeGrowth(before: CheckoutState): string[] {
  const done: string[] = []
  const now = captureCheckout(before.repo, true)
  const grown = growthSince(before, now)
  for (const w of grown.worktrees) {
    try {
      git(before.repo, ['worktree', 'remove', '--force', w])
      done.push(`removed worktree ${w}`)
    } catch {
      rmSync(w, { recursive: true, force: true })
      done.push(`deleted ${w} (git no longer listed it)`)
    }
  }
  git(before.repo, ['worktree', 'prune'])
  for (const ref of grown.branches) {
    try {
      git(before.repo, ['branch', '-D', ref.slice('refs/heads/'.length)])
      done.push(`deleted branch ${ref.slice('refs/heads/'.length)}`)
    } catch {
      done.push(`could not delete ${ref}`)
    }
  }
  return done
}
