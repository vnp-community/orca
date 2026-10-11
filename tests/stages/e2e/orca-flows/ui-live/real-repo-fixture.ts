/**
 * The throwaway git repo a real-AI spec points its project at. It lives in its own temp directory so everything the agent
 * creates (the repo, and the task worktrees git-gateway places next to it as `<repo>-<branch>`) stays under one root that
 * the tool guard (fixture-repo/template/.claude/tool-guard.mjs) allows and `dispose()` removes. Never a repo of the user.
 */
import { execFileSync, spawnSync } from 'node:child_process'
import {
  cpSync,
  existsSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  realpathSync,
  rmSync,
  writeFileSync
} from 'node:fs'
import { homedir, tmpdir } from 'node:os'
import { join, resolve } from 'node:path'

const TEMPLATE = resolve(__dirname, 'fixture-repo/template')

export type RealFixture = {
  /** Directory that owns the repo and its worktrees; the only place the agent may write. */
  base: string
  repoPath: string
  /** Lines of the guard's audit logs (calls the hook saw, calls it refused). */
  guardCalls: () => string[]
  guardDenials: () => string[]
  /** Runs the hook exactly as claude would under the agent's PATH; throws if it would not block an escape. */
  selfTestGuard: () => void
  /** `git status --porcelain` of the main checkout of the fixture. */
  repoStatus: () => string
  headSha: () => string
  /** Branches and worktrees that exist now, for the "something was done" assertions. */
  branches: () => string[]
  worktrees: () => string[]
  dispose: () => void
}

export function fixtureGit(cwd: string, args: string[]): string {
  return execFileSync('git', ['-c', 'user.name=e2e', '-c', 'user.email=e2e@example.com', ...args], {
    cwd,
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  })
}

export function guardLogLines(path: string): string[] {
  try {
    return readFileSync(path, 'utf8').split('\n').filter(Boolean)
  } catch {
    return []
  }
}

/**
 * Runs the repo's PreToolUse hook exactly as claude would under the agent's PATH (no /opt/homebrew/bin); throws if it would not
 * block an escape, i.e. if the hook would fail open. `logDir` is the hook's allowedRoot: its audit logs are removed afterwards.
 */
export function runToolGuardSelfTest(repoPath: string, settings: string, logDir: string): void {
  const hook = (
    JSON.parse(readFileSync(settings, 'utf8')) as {
      hooks: { PreToolUse: { hooks: { command: string }[] }[] }
    }
  ).hooks.PreToolUse[0].hooks[0].command
  // Same PATH the agent gives claude (agent-config.ts buildToolPath): no /opt/homebrew/bin.
  const env = {
    PATH: `${homedir()}/.local/bin:${homedir()}/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/snap/bin`,
    HOME: homedir(),
    CLAUDE_PROJECT_DIR: repoPath
  }
  const run = (payload: object): number | null =>
    spawnSync('/bin/sh', ['-c', hook], {
      cwd: repoPath,
      env,
      input: JSON.stringify(payload),
      timeout: 10_000
    }).status
  const inside = run({ tool_name: 'Read', tool_input: { file_path: join(repoPath, 'README.md') } })
  const outside = run({
    tool_name: 'Write',
    tool_input: { file_path: join(homedir(), 'orca-e2e-guard-selftest.txt') }
  })
  const escape = run({ tool_name: 'Bash', tool_input: { command: 'cat ../../etc/hosts' } })
  rmSync(join(logDir, 'tool-guard-calls.log'), { force: true })
  rmSync(join(logDir, 'tool-guard-denials.log'), { force: true })
  if (inside !== 0 || outside !== 2 || escape !== 2) {
    throw new Error(
      `tool guard self-test failed (read inside=${inside}, write outside=${outside}, ".." escape=${escape}; want 0/2/2): the hook would fail open`
    )
  }
}

export function createCalcFixture(prefix: string): RealFixture {
  // Why: realpath, because macOS tmpdir is a symlink and the guard compares real paths.
  const base = realpathSync(mkdtempSync(join(tmpdir(), `${prefix}-real-`)))
  const repoPath = join(base, 'repo')
  cpSync(TEMPLATE, repoPath, { recursive: true })
  // Why: the agent hands claude a PATH without /opt/homebrew/bin, so a bare `node` would fail open (hook errors do not block).
  const settings = join(repoPath, '.claude', 'settings.json')
  writeFileSync(settings, readFileSync(settings, 'utf8').replace('__NODE__', process.execPath))
  writeFileSync(
    join(repoPath, '.claude', 'tool-guard.config.json'),
    `${JSON.stringify({ allowedRoot: base })}\n`
  )
  fixtureGit(repoPath, ['init', '-q', '-b', 'main'])
  fixtureGit(repoPath, ['add', '-A'])
  fixtureGit(repoPath, ['commit', '-q', '-m', 'init calc-sandbox'])
  let disposed = false
  return {
    base,
    repoPath,
    selfTestGuard: () => runToolGuardSelfTest(repoPath, settings, base),
    guardCalls: () => guardLogLines(join(base, 'tool-guard-calls.log')),
    guardDenials: () => guardLogLines(join(base, 'tool-guard-denials.log')),
    repoStatus: () => fixtureGit(repoPath, ['status', '--porcelain']).trim(),
    headSha: () => fixtureGit(repoPath, ['rev-parse', 'HEAD']).trim(),
    branches: () =>
      fixtureGit(repoPath, ['for-each-ref', '--format=%(refname:short)', 'refs/heads'])
        .split('\n')
        .filter(Boolean),
    worktrees: () =>
      fixtureGit(repoPath, ['worktree', 'list', '--porcelain'])
        .split('\n')
        .filter((l) => l.startsWith('worktree '))
        .map((l) => l.slice('worktree '.length)),
    dispose: () => {
      if (disposed) {
        return
      }
      disposed = true
      rmSync(base, { recursive: true, force: true })
    }
  }
}

/** Names in the OS temp dir that still carry the run prefix: empty after a clean dispose. */
export function leftoversOf(prefix: string): string[] {
  return existsSync(tmpdir()) ? readdirSync(tmpdir()).filter((n) => n.startsWith(prefix)) : []
}
