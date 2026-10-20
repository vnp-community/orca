/**
 * The user's repo deepseek-harness as the working repo of the real-AI specs: the agent really reads and writes code in it.
 * Unlike the throwaway fixtures there is no copy: the folder IS the user's checkout, so the protection is different:
 *  - the main checkout is never written (HEAD, branch, every changed and untracked file, local config and every ref are fingerprinted
 *    and compared while the run goes and again after cleanup; rules in live-main-checkout-rules.ts);
 *  - the agent works only in the task worktree Orca makes next to the repo (`<repo>-<branch>`), which cleanup removes with its branch;
 *  - `git push` is impossible for the whole run: the agent is restarted with a push block in its environment (run_ui_flow.py), and
 *    the fixture refuses to start unless that block is live and verified, because the repo has a real GitHub remote.
 * A tool-guard hook cannot be installed: the repo's .claude/settings.json is gitignored, so a task worktree would not inherit it.
 */
import { execFileSync, spawnSync } from 'node:child_process'
import { existsSync, realpathSync } from 'node:fs'
import { basename } from 'node:path'
import { captureCheckout, removeGrowth } from './live-main-checkout-guard'
import { checkoutViolations, growthSince, restoredProblems } from './live-main-checkout-rules'
import type { RealFixture } from './real-repo-fixture'

export const DEEPSEEK_REPO =
  process.env.ORCA_E2E_DEEPSEEK_REPO || '/Users/datbeohbbh/Desktop/stuff/agentic/deepseek-harness'
/** Where the specs ask the agent to write: a directory that does not exist in the repo, so nothing of the user's code is edited. */
export const SANDBOX_DIR = 'scripts/e2e-sandbox'

export type DeepseekFixture = RealFixture & {
  repoName: string
  /** Task worktrees the run created next to the repo that still exist. */
  taskWorktrees: () => string[]
  /** Removes the task worktrees and branches now (what cleanup does at the end), e.g. to see how the UI reports a worktree that is gone. */
  removeTaskWorktrees: () => string[]
  /** What cleanup did and whether the repo is as it was; read after `dispose()`. */
  afterCleanup: () => string[]
}

function git(repo: string, args: string[]): string {
  return execFileSync('git', ['-C', repo, ...args], {
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
    stdio: ['ignore', 'pipe', 'ignore']
  })
}

/** The agent processes whose environment carries the push block that run_ui_flow.py gives them. */
export function agentsWithPushBlock(): { total: number; blocked: number } {
  const pids = spawnSync('pgrep', ['-f', 'agent/out/agent.js'], { encoding: 'utf8' })
    .stdout.split('\n')
    .filter((p) => /^\d+$/.test(p))
  let blocked = 0
  for (const pid of pids) {
    const env = spawnSync('ps', ['eww', '-p', pid], { encoding: 'utf8' }).stdout
    if (/\bGIT_CONFIG_COUNT=\d+/.test(env) && /nonexistent\/orca-e2e-push-blocked/.test(env)) {
      blocked++
    }
  }
  return { total: pids.length, blocked }
}

/** The environment run_ui_flow.py gives the agent: every push, to the origin URL or to the same URL typed out, goes nowhere. */
export const PUSH_BLOCK_ENV: Record<string, string> = {
  GIT_CONFIG_COUNT: '4',
  GIT_CONFIG_KEY_0: 'remote.origin.pushurl',
  GIT_CONFIG_VALUE_0: '/nonexistent/orca-e2e-push-blocked',
  GIT_CONFIG_KEY_1: 'url./nonexistent/orca-e2e-push-blocked/.pushInsteadOf',
  GIT_CONFIG_VALUE_1: 'git@github.com:',
  GIT_CONFIG_KEY_2: 'url./nonexistent/orca-e2e-push-blocked/.pushInsteadOf',
  GIT_CONFIG_VALUE_2: 'https://github.com/',
  GIT_CONFIG_KEY_3: 'url./nonexistent/orca-e2e-push-blocked/.pushInsteadOf',
  GIT_CONFIG_VALUE_3: 'ssh://git@github.com/'
}

/** Proves the block works the way the agent gets it: a dry-run push, by remote name and by typed URL, fails locally. */
function expectPushBlocked(repo: string): void {
  const remote = git(repo, ['remote', 'get-url', 'origin']).trim()
  for (const target of ['origin', remote]) {
    const r = spawnSync(
      'git',
      ['-C', repo, 'push', '--dry-run', target, 'HEAD:refs/heads/e2e-push-probe'],
      {
        encoding: 'utf8',
        timeout: 20_000,
        env: { ...process.env, ...PUSH_BLOCK_ENV, GIT_TERMINAL_PROMPT: '0' }
      }
    )
    if (r.status === 0 || !/orca-e2e-push-blocked/.test(`${r.stderr}${r.stdout}`)) {
      throw new Error(
        `the push block does not stop \`git push --dry-run ${target === 'origin' ? 'origin' : '<origin url>'}\` (exit ${r.status}): ${`${r.stderr}${r.stdout}`.slice(0, 200)}`
      )
    }
  }
}

export type UserRepoOptions = {
  /** The user's repo to work in (default: the deepseek-harness one). */
  repo?: string
  /** Leave the task worktrees and branches the run created, for the owner to inspect (ci-cd-kit); everything else is still compared. */
  keepGrowth?: boolean
}

export function createDeepseekFixture(
  _prefix: string,
  options: UserRepoOptions = {}
): DeepseekFixture {
  const repo = options.repo ?? DEEPSEEK_REPO
  if (!existsSync(repo)) {
    throw new Error(
      `${repo} does not exist: set ORCA_E2E_DEEPSEEK_REPO to the repo the real specs may work in`
    )
  }
  const repoPath = realpathSync(repo)
  if (git(repoPath, ['rev-parse', '--is-inside-work-tree']).trim() !== 'true') {
    throw new Error(`${repoPath} is not a git work tree`)
  }
  // Why: set before the harness fingerprints its repos, so the generic exact-equality check does not see this repo's own growth.
  process.env.ORCA_E2E_GUARD_EXCLUDE = [process.env.ORCA_E2E_GUARD_EXCLUDE, repoPath]
    .filter(Boolean)
    .join(':')
  if (options.keepGrowth) {
    // Why: the kept worktrees are new entries of the repo's parent folder, which the generic listing comparison would call a leak.
    process.env.ORCA_E2E_GUARD_KEEP_NEW_PREFIXES = [
      process.env.ORCA_E2E_GUARD_KEEP_NEW_PREFIXES,
      `${basename(repoPath)}-`
    ]
      .filter(Boolean)
      .join(':')
  }
  const before = captureCheckout(repoPath)
  // Why: the watchdog polls in the cheap mode (size+mtime); a cheap reading is only comparable with a cheap baseline.
  const beforeCheap = captureCheckout(repoPath, true)
  console.log(
    `[deepseek] repo ${repoPath}: HEAD ${before.main.head.slice(0, 10)}, ${Object.keys(before.main.files).length} changed/untracked file(s) (the user's, never touched), ${Object.keys(before.worktrees).length} worktree(s) of the user`
  )
  let violations: string[] = []
  let disposed = false
  return {
    base: repoPath,
    repoPath,
    repoName: basename(repoPath),
    guardCalls: () => [],
    // Why: the harness's watchdog polls this; a non-empty list aborts the run and kills the agent.
    guardDenials: () => {
      if (disposed) {
        return []
      }
      const found = checkoutViolations(beforeCheap, captureCheckout(repoPath, true))
      if (found.length > 0) {
        violations = [...new Set([...violations, ...found])]
      }
      return violations
    },
    selfTestGuard: () => {
      const agents = agentsWithPushBlock()
      if (agents.total === 0 || agents.blocked < agents.total) {
        throw new Error(
          `${agents.blocked}/${agents.total} agent process(es) run with the push block: start the run with run_ui_flow.py --real (deepseek or ci-cd-kit; it restarts the agent with GIT_CONFIG_* so nothing can be pushed to ${git(repoPath, ['remote', 'get-url', 'origin']).trim()})`
        )
      }
      expectPushBlocked(repoPath)
    },
    workRoots: () => [repoPath, ...growthSince(before, captureCheckout(repoPath, true)).worktrees],
    repoStatus: () => git(repoPath, ['status', '--porcelain']).trim(),
    headSha: () => git(repoPath, ['rev-parse', 'HEAD']).trim(),
    branches: () =>
      git(repoPath, ['for-each-ref', '--format=%(refname:short)', 'refs/heads'])
        .split('\n')
        .filter(Boolean),
    // Why: only the main checkout and what this run added; the user's own worktrees must never be taken for the task's.
    worktrees: () => [repoPath, ...growthSince(before, captureCheckout(repoPath, true)).worktrees],
    removeTaskWorktrees: () => removeGrowth(before),
    taskWorktrees: () => growthSince(before, captureCheckout(repoPath, true)).worktrees,
    dispose: () => {
      if (disposed) {
        return
      }
      violations = [
        ...new Set([
          ...violations,
          ...checkoutViolations(beforeCheap, captureCheckout(repoPath, true))
        ])
      ]
      if (options.keepGrowth) {
        const kept = growthSince(before, captureCheckout(repoPath, true))
        console.log(
          `[deepseek] kept for inspection: ${kept.worktrees.join(', ') || 'no worktree'}; branches ${kept.branches.map((b) => b.replace('refs/heads/', '')).join(', ') || 'none'}`
        )
      } else {
        for (const done of removeGrowth(before)) {
          console.log(`[deepseek] cleanup: ${done}`)
        }
      }
      disposed = true
    },
    afterCleanup: () => [
      ...violations.map((v) => `the user's repo: ${v}`),
      // Why: kept growth is allowed by checkoutViolations (new heads and `<repo>-*` worktrees only); restoredProblems would demand it gone.
      ...(options.keepGrowth
        ? checkoutViolations(before, captureCheckout(repoPath))
        : restoredProblems(before, captureCheckout(repoPath))
      ).map((p) => `the user's repo after cleanup: ${p}`)
    ]
  }
}
