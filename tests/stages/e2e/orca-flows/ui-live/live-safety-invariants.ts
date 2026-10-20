/**
 * Safety net of the real-AI specs. The dev server's agent runs `claude --dangerously-skip-permissions` for task
 * execution (task-service always sends trustPreset "full"), so nothing prevents it from touching other paths. These
 * helpers fingerprint what must not change (the real repo, other repos, directory listings, stray claude processes)
 * before and after a run, so any leak fails the spec instead of going unnoticed.
 */
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { existsSync, readdirSync, readFileSync, realpathSync, statSync } from 'node:fs'
import { homedir } from 'node:os'
import { dirname, join, resolve } from 'node:path'

const REPO_ROOT = resolve(__dirname, '../../../../..')

export type RepoFingerprint = {
  path: string
  head: string
  refs: string
  worktrees: string
  config: string
  /** Raw lists behind `refs` and `worktrees`, to tell a foreign change from one that names this run. */
  refList: string[]
  worktreeList: string[]
  /** Every path git reports as changed (tracked or untracked) -> its content hash (or size+mtime in the cheap mode). */
  files: Record<string, string>
}

export type GuardedState = {
  repos: RepoFingerprint[]
  listings: Record<string, string[]>
  claudePrintPids: number[]
}

const sha = (data: string | Buffer): string =>
  createHash('sha256').update(data).digest('hex').slice(0, 16)

function git(repo: string, args: string[]): string {
  return execFileSync('git', ['-C', repo, ...args], {
    encoding: 'utf8',
    maxBuffer: 256 * 1024 * 1024,
    stdio: ['ignore', 'pipe', 'ignore']
  })
}

/** Path prefixes (relative to a repo, colon separated in ORCA_E2E_GUARD_IGNORE) that people legitimately edit during a run. */
function ignoredPrefixes(): string[] {
  return (process.env.ORCA_E2E_GUARD_IGNORE ?? '').split(':').filter(Boolean)
}

/** Everything about one repo that a stray write could change; `cheap` hashes size+mtime instead of content (watchdog polling). */
export function fingerprintRepo(
  path: string,
  cheap = false,
  // Why: the ignore list is for this checkout (people edit docs/ and specs/ during a run); a repo the agent works in ignores nothing.
  ignore: string[] = ignoredPrefixes()
): RepoFingerprint {
  const entries = git(path, ['status', '--porcelain=v1', '-z', '--untracked-files=all'])
    .split('\0')
    .filter(Boolean)
  const files: Record<string, string> = {}
  for (let i = 0; i < entries.length; i++) {
    const code = entries[i].slice(0, 2)
    const file = entries[i].slice(3)
    if (code[0] === 'R' || code[0] === 'C') {
      i++ // the next -z field is the rename source
    }
    if (ignore.some((p) => file.startsWith(p))) {
      continue
    }
    try {
      const abs = join(path, file)
      files[file] = cheap
        ? `s:${statSync(abs).size}:${statSync(abs).mtimeMs}`
        : `h:${sha(readFileSync(abs))}`
    } catch {
      files[file] = 'gone'
    }
  }
  const refs = git(path, ['for-each-ref', '--format=%(refname) %(objectname)'])
  const worktrees = git(path, ['worktree', 'list', '--porcelain'])
  return {
    path,
    head: git(path, ['rev-parse', 'HEAD']).trim(),
    refs: sha(refs),
    worktrees: sha(worktrees),
    config: sha(git(path, ['config', '--local', '--list'])),
    refList: refs.split('\n').filter(Boolean),
    worktreeList: worktrees.split('\n').filter((l) => l.startsWith('worktree ')),
    files
  }
}

function listing(dir: string): string[] {
  try {
    return readdirSync(dir).sort()
  } catch {
    return []
  }
}

/** pid of every `claude ... --print` process: the form the agent spawns for task execution and read-only analysis. */
export function claudePrintProcesses(): { pid: number; ppid: number; command: string }[] {
  const out = execFileSync('ps', ['-axo', 'pid=,ppid=,command='], { encoding: 'utf8' })
  const found: { pid: number; ppid: number; command: string }[] = []
  for (const line of out.split('\n')) {
    const m = /^\s*(\d+)\s+(\d+)\s+(.*)$/.exec(line)
    if (m && /\bclaude\b/.test(m[3].split(' ')[0] ?? '') && / --print( |$)/.test(m[3])) {
      found.push({ pid: Number(m[1]), ppid: Number(m[2]), command: m[3] })
    }
  }
  return found
}

/** The repos to protect: this checkout, plus ORCA_E2E_GUARD_PATHS (colon separated) and the agent's own work dir if known. */
export function guardedRepoPaths(): string[] {
  const extra = [process.env.ORCA_E2E_GUARD_PATHS ?? '', process.env.AGENT_WORK_DIR ?? '']
    .join(':')
    .split(':')
    .filter((p) => p && existsSync(join(p, '.git')))
  // Why: a repo the run is MEANT to work in has its own rules (live-main-checkout-rules.ts); exact equality would flag the task worktree.
  const excluded = (process.env.ORCA_E2E_GUARD_EXCLUDE ?? '')
    .split(':')
    .filter((p) => p && existsSync(p))
    .map((p) => realpathSync(p))
  return [...new Set([REPO_ROOT, ...extra].map((p) => realpathSync(p)))].filter(
    (p) => !excluded.includes(p)
  )
}

export function captureGuardedState(cheap = false): GuardedState {
  const repos = guardedRepoPaths().map((p) => fingerprintRepo(p, cheap))
  const dirs = [homedir(), join(homedir(), 'Desktop'), dirname(REPO_ROOT), REPO_ROOT]
  return {
    repos,
    listings: Object.fromEntries(dirs.map((d) => [d, listing(d)])),
    claudePrintPids: claudePrintProcesses().map((p) => p.pid)
  }
}

/** Human-readable differences; empty means the run left every guarded path exactly as it was (no tolerance for other writers). */
export function diffGuardedState(
  before: GuardedState,
  after: GuardedState,
  // Why: while the agent works a claude process is expected; only the end-of-run check treats a leftover as a leak.
  opts: { processes: boolean } = { processes: true }
): string[] {
  const problems: string[] = []
  for (const b of before.repos) {
    const a = after.repos.find((r) => r.path === b.path)
    if (!a) {
      problems.push(`${b.path}: missing after the run`)
      continue
    }
    for (const key of ['head', 'refs', 'worktrees', 'config'] as const) {
      if (b[key] !== a[key]) {
        const gained = [
          ...a.refList.filter((r) => !b.refList.includes(r)),
          ...a.worktreeList.filter((w) => !b.worktreeList.includes(w))
        ]
        problems.push(
          `${b.path}: ${key} changed (${b[key]} -> ${a[key]}${gained.length ? `; new: ${gained.slice(0, 3).join(', ')}` : ''})`
        )
      }
    }
    const names = new Set([...Object.keys(b.files), ...Object.keys(a.files)])
    const changed = [...names].filter((f) => b.files[f] !== a.files[f])
    if (changed.length > 0) {
      problems.push(
        `${b.path}: ${changed.length} file(s) changed: ${changed.slice(0, 8).join(', ')}`
      )
    }
  }
  for (const [dir, names] of Object.entries(before.listings)) {
    const now = new Set(after.listings[dir] ?? [])
    const was = new Set(names)
    const added = [...now].filter((n) => !was.has(n))
    const removed = [...was].filter((n) => !now.has(n))
    // Why: the repo root legitimately gains Playwright output (gitignored); dotfiles of $HOME churn by themselves.
    const ignorable = (n: string): boolean =>
      dir === REPO_ROOT
        ? n === 'test-results'
        : dir === homedir()
          ? n.startsWith('.') || n === 'Library'
          : false
    // Why: a spec that keeps the new worktrees for the owner to inspect (ci-cd-kit) names their prefix; only ADDED entries get the pass.
    const keptPrefixes = (process.env.ORCA_E2E_GUARD_KEEP_NEW_PREFIXES ?? '')
      .split(':')
      .filter(Boolean)
    const real = [
      ...added.filter((n) => !keptPrefixes.some((p) => n.startsWith(p))),
      ...removed
    ].filter((n) => !ignorable(n))
    if (real.length > 0) {
      problems.push(
        `${dir}: entries changed (+${added.join(',') || '-'} / -${removed.join(',') || '-'})`
      )
    }
  }
  const stray = after.claudePrintPids.filter((p) => !before.claudePrintPids.includes(p))
  if (opts.processes && stray.length > 0) {
    problems.push(`claude --print process(es) still running after the run: ${stray.join(',')}`)
  }
  return problems
}

/**
 * Waits until nobody has touched the guarded paths for `quietMs`. The comparison is exact, so another person (or agent) editing
 * the checkout during a run would abort it; starting in a quiet moment keeps that rare without loosening the comparison.
 */
export async function waitForQuietTree(quietMs: number, maxWaitMs: number): Promise<number> {
  const started = Date.now()
  let last = captureGuardedState(true)
  let quietSince = Date.now()
  while (Date.now() - quietSince < quietMs) {
    if (Date.now() - started > maxWaitMs) {
      throw new Error(
        `the guarded checkout kept changing for ${Math.round(maxWaitMs / 1000)}s (someone is editing it); real-AI specs need a quiet tree: ${diffGuardedState(captureGuardedState(true), last).join('; ') || 'unknown'}`
      )
    }
    await new Promise((r) => setTimeout(r, 5_000))
    const now = captureGuardedState(true)
    if (diffGuardedState(last, now, { processes: false }).length > 0) {
      quietSince = Date.now()
    }
    last = now
  }
  return Math.round((Date.now() - started) / 1000)
}

function descendants(pid: number, table: { pid: number; ppid: number }[]): number[] {
  const kids = table.filter((p) => p.ppid === pid).map((p) => p.pid)
  return kids.flatMap((k) => [k, ...descendants(k, table)])
}

function claudeCwd(pid: number): string {
  try {
    return (
      /\nn(.*)\n?$/m.exec(
        execFileSync('lsof', ['-a', '-p', String(pid), '-d', 'cwd', '-Fn'], {
          encoding: 'utf8',
          stdio: ['ignore', 'pipe', 'ignore']
        })
      )?.[1] ?? ''
    )
  } catch {
    return ''
  }
}

/** True while a claude --print process works somewhere inside `base` (the agent is really running a task there). */
export function claudeRunningUnder(base: string | string[]): boolean {
  const roots = Array.isArray(base) ? base : [base]
  return claudePrintProcesses().some((p) => {
    const cwd = claudeCwd(p.pid)
    return roots.some((root) => cwd === root || cwd.startsWith(`${root}/`))
  })
}

/**
 * Kills claude --print processes whose working directory is inside `base`, and everything they spawned. Not a process-group
 * kill: the agent starts claude in its own group, so signalling the group would also take the agent down.
 */
export function killClaudeUnder(base: string | string[]): number[] {
  const roots = Array.isArray(base) ? base : [base]
  const table = execFileSync('ps', ['-axo', 'pid=,ppid='], { encoding: 'utf8' })
    .split('\n')
    .map((l) => /^\s*(\d+)\s+(\d+)/.exec(l))
    .filter((m): m is RegExpExecArray => m !== null)
    .map((m) => ({ pid: Number(m[1]), ppid: Number(m[2]) }))
  const killed: number[] = []
  for (const p of claudePrintProcesses()) {
    let cwd = ''
    try {
      cwd =
        /\nn(.*)\n?$/m.exec(
          execFileSync('lsof', ['-a', '-p', String(p.pid), '-d', 'cwd', '-Fn'], {
            encoding: 'utf8',
            stdio: ['ignore', 'pipe', 'ignore']
          })
        )?.[1] ?? ''
    } catch {
      continue
    }
    if (roots.some((root) => cwd === root || cwd.startsWith(`${root}/`))) {
      for (const pid of [...descendants(p.pid, table).toReversed(), p.pid]) {
        try {
          process.kill(pid, 'SIGKILL')
          killed.push(pid)
        } catch {
          // already gone
        }
      }
    }
  }
  return killed
}
