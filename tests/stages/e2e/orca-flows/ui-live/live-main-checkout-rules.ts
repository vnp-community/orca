/**
 * Rules for a repo that belongs to the USER and that a real-AI spec works on (deepseek-harness). Orca legitimately grows that repo
 * while a Request executes (a task branch and a `<repo>-<branch>` worktree next to it), so the exact-equality check of
 * live-safety-invariants cannot run while the run is going. These rules allow exactly that growth and nothing else; after
 * cleanup the repo must be what it was before, ref for ref and file for file. Pure: the git side is live-main-checkout-guard.ts.
 */
import { basename, dirname, join } from 'node:path'
import type { RepoFingerprint } from './live-safety-invariants.ts'

export type CheckoutState = {
  repo: string
  parent: string
  main: RepoFingerprint
  /** refname -> object id, every ref of the repo (heads, remotes, tags, stash). */
  refs: Record<string, string>
  /** worktree path -> fingerprint (HEAD, status) of every worktree git lists, the main checkout excluded. */
  worktrees: Record<string, RepoFingerprint>
  /** Entries of the parent directory (the repo's neighbours). */
  neighbours: string[]
}

/** What the run added: refs under refs/heads/ (task branches) and worktrees named `<repo>-*` next to the repo. */
export type CheckoutGrowth = { branches: string[]; worktrees: string[] }

export function growthSince(before: CheckoutState, now: CheckoutState): CheckoutGrowth {
  return {
    branches: Object.keys(now.refs).filter(
      (r) => !(r in before.refs) && r.startsWith('refs/heads/')
    ),
    worktrees: Object.keys(now.worktrees).filter((w) => !(w in before.worktrees))
  }
}

/** Violations while the run is going: anything but the allowed growth. Empty = the user's repo is as it should be. */
export function checkoutViolations(before: CheckoutState, now: CheckoutState): string[] {
  const problems: string[] = []
  const name = basename(before.repo)
  const m = before.main
  const a = now.main
  if (m.head !== a.head) {
    problems.push(`the main checkout moved HEAD (${m.head.slice(0, 10)} -> ${a.head.slice(0, 10)})`)
  }
  if (m.config !== a.config) {
    problems.push('the repo-local git config changed')
  }
  const names = new Set([...Object.keys(m.files), ...Object.keys(a.files)])
  const changed = [...names].filter((f) => m.files[f] !== a.files[f])
  if (changed.length > 0) {
    problems.push(
      `${changed.length} file(s) of the main checkout changed: ${changed.slice(0, 8).join(', ')}`
    )
  }
  for (const [ref, sha] of Object.entries(before.refs)) {
    if (now.refs[ref] !== sha) {
      problems.push(`ref ${ref} ${now.refs[ref] ? 'moved' : 'was deleted'}`)
    }
  }
  for (const ref of Object.keys(now.refs)) {
    if (!(ref in before.refs) && !ref.startsWith('refs/heads/')) {
      problems.push(`new ref outside refs/heads/: ${ref} (a push, a tag or a stash)`)
    }
  }
  for (const [path, was] of Object.entries(before.worktrees)) {
    const is = now.worktrees[path]
    if (!is) {
      problems.push(`worktree ${path} of the user disappeared`)
    } else if (was.head !== is.head || JSON.stringify(was.files) !== JSON.stringify(is.files)) {
      problems.push(`worktree ${path} of the user changed`)
    }
  }
  const grown = growthSince(before, now)
  for (const w of grown.worktrees) {
    if (dirname(w) !== before.parent || !basename(w).startsWith(`${name}-`)) {
      problems.push(`a worktree appeared outside the allowed place \`<parent>/${name}-*\`: ${w}`)
    }
  }
  const allowedDirs = new Set(grown.worktrees.map((w) => basename(w)))
  const before_ = new Set(before.neighbours)
  const gained = now.neighbours.filter((n) => !before_.has(n) && !allowedDirs.has(n))
  const lost = before.neighbours.filter((n) => !now.neighbours.includes(n))
  if (gained.length + lost.length > 0) {
    problems.push(
      `entries next to the repo changed (+${gained.join(',') || '-'} / -${lost.join(',') || '-'})`
    )
  }
  return problems
}

/** After cleanup: exactly the state before. */
export function restoredProblems(before: CheckoutState, after: CheckoutState): string[] {
  const problems = checkoutViolations(before, after)
  const grown = growthSince(before, after)
  for (const b of grown.branches) {
    problems.push(`branch ${b} was not removed`)
  }
  for (const w of grown.worktrees) {
    problems.push(`worktree ${w} was not removed`)
  }
  for (const n of after.neighbours.filter((x) => !before.neighbours.includes(x))) {
    problems.push(`${join(before.parent, n)} was not removed`)
  }
  return problems
}
