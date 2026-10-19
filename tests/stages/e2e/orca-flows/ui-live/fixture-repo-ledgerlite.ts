/**
 * A throwaway git repo built from the LedgerLite finance fixture (tests/stages/e2e/fixtures/ledgerlite-finance), the TypeScript twin of
 * ledgerlite_fixture.py. Same steps as the fixture README: copy -> `tdd-agent generate` -> `tdd-agent mcp enable --read-only` -> git init and
 * one commit (so task worktrees get .mcp.json), plus the extras the base-branch and analysis-revision specs need: a `release/1.x` branch
 * whose tip differs from main, a third branch the primary checkout is left on, and the one-line bug of scenario 1, read from
 * docs/e2e-scenarios.md so the patch has one source of truth. No AI and no network. Never a repo of the user.
 */
import { execFileSync, spawnSync } from 'node:child_process'
import {
  cpSync,
  mkdtempSync,
  readFileSync,
  realpathSync,
  rmSync,
  writeFileSync,
  readdirSync
} from 'node:fs'
import { basename, dirname, join, resolve } from 'node:path'
import { browsableTempBase } from './live-add-project-repro-steps'

export const LEDGERLITE_FIXTURE_DIR = resolve(__dirname, '../../fixtures/ledgerlite-finance')
export const LEDGERLITE_RELEASE_BRANCH = 'release/1.x'
const DEFAULT_BRANCH = 'main'
const GIT_ENV = {
  GIT_AUTHOR_NAME: 'ORCA e2e',
  GIT_AUTHOR_EMAIL: 'e2e@orca.invalid',
  GIT_COMMITTER_NAME: 'ORCA e2e',
  GIT_COMMITTER_EMAIL: 'e2e@orca.invalid'
}
const GENERATED = new Set([
  '.git',
  'tdd',
  'runs',
  'logs',
  'staging',
  'last-generate.json',
  '.mcp.json'
])

export type LedgerliteOptions = {
  knowledge?: boolean
  releaseBranch?: boolean
  thirdBranch?: string
  injectBug?: boolean
  verifyBaseline?: boolean
}

export type LedgerliteRepo = {
  path: string
  name: string
  branches: string[]
  knowledge: boolean
  bugCommit: string
  git: (...args: string[]) => string
  revParse: (ref: string) => string
  currentBranch: () => string
  cleanup: () => void
}

function run(cmd: string, args: string[], cwd: string, timeout = 180_000): string {
  return execFileSync(cmd, args, {
    cwd,
    env: { ...process.env, ...GIT_ENV },
    encoding: 'utf8',
    timeout,
    stdio: ['ignore', 'pipe', 'pipe']
  })
}

export function tddAgentAvailable(): boolean {
  return spawnSync('tdd-agent', ['--version'], { encoding: 'utf8' }).status !== null
}

/** The scenario 1 patch, read from the scenarios document so the two cannot drift. */
export function ledgerliteBugPatch(): string {
  const text = readFileSync(join(LEDGERLITE_FIXTURE_DIR, 'docs/e2e-scenarios.md'), 'utf8')
  const m = /git apply <<'PATCH'\n([\s\S]*?)\nPATCH\n/.exec(text)
  if (!m) {
    throw new Error('bug patch block not found in e2e-scenarios.md')
  }
  return `${m[1]}\n`
}

function copyFixture(dest: string): void {
  cpSync(LEDGERLITE_FIXTURE_DIR, dest, {
    recursive: true,
    filter: (src) => {
      const name = basename(src)
      return !GENERATED.has(name) && !/\.db(-shm|-wal)?$/.test(name)
    }
  })
}

export function createLedgerliteRepo(prefix: string, opts: LedgerliteOptions = {}): LedgerliteRepo {
  if (opts.knowledge && !tddAgentAvailable()) {
    throw new Error('tdd-agent is not on PATH; install it or create the repo with knowledge: false')
  }
  const base = mkdtempSync(join(browsableTempBase(), `${prefix}-ledgerlite-`))
  const path = join(realpathSync(base), 'ledgerlite')
  const repo: LedgerliteRepo = {
    path,
    name: 'ledgerlite',
    branches: [],
    knowledge: false,
    bugCommit: '',
    git: (...args) => run('git', args, path),
    revParse: (ref) =>
      run('git', ['rev-parse', '--verify', '--quiet', `${ref}^{commit}`], path).trim(),
    currentBranch: () => run('git', ['rev-parse', '--abbrev-ref', 'HEAD'], path).trim(),
    cleanup: () => {
      for (const sibling of readdirSync(dirname(path))) {
        if (sibling.startsWith('ledgerlite-')) {
          rmSync(join(dirname(path), sibling), { recursive: true, force: true })
        }
      }
      rmSync(base, { recursive: true, force: true })
    }
  }
  try {
    copyFixture(path)
    if (opts.knowledge) {
      run('tdd-agent', ['generate'], path, 300_000)
      run('tdd-agent', ['mcp', 'enable', '--read-only'], path, 120_000)
      repo.knowledge = true
    }
    repo.git('init', '-q', '-b', DEFAULT_BRANCH)
    repo.git('add', '-A')
    repo.git('commit', '-q', '-m', 'LedgerLite baseline')
    repo.branches.push(DEFAULT_BRANCH)
    if (opts.verifyBaseline) {
      execFileSync('go', ['test', './...', '-count=1'], {
        cwd: path,
        env: { ...process.env, GOWORK: 'off' },
        timeout: 300_000
      })
    }
    if (opts.injectBug) {
      const applied = spawnSync('git', ['apply'], {
        cwd: path,
        env: { ...process.env, ...GIT_ENV },
        input: ledgerliteBugPatch(),
        encoding: 'utf8'
      })
      if (applied.status !== 0) {
        throw new Error(`bug patch does not apply: ${applied.stderr.trim().slice(0, 400)}`)
      }
      repo.git('commit', '-q', '-am', 'Use a rolling 24h window for the daily limit')
      repo.bugCommit = repo.revParse('HEAD')
    }
    if (opts.releaseBranch) {
      branchWithCommit(repo, LEDGERLITE_RELEASE_BRANCH, 'RELEASE-1.x.md', 'Release line 1.x\n')
    }
    if (opts.thirdBranch) {
      branchWithCommit(repo, opts.thirdBranch, 'OTHER.md', `Work on ${opts.thirdBranch}\n`)
      repo.git('checkout', '-q', opts.thirdBranch)
    }
    return repo
  } catch (error) {
    rmSync(base, { recursive: true, force: true })
    throw error
  }
}

/** A branch whose tip differs from main, so a worktree's starting commit says which branch it came from. */
function branchWithCommit(
  repo: LedgerliteRepo,
  branch: string,
  filename: string,
  body: string
): void {
  repo.git('checkout', '-q', '-b', branch, DEFAULT_BRANCH)
  writeFileSync(join(repo.path, 'docs', filename), body, 'utf8')
  repo.git('add', '-A')
  repo.git('commit', '-q', '-m', `Commit on ${branch}`)
  repo.branches.push(branch)
  repo.git('checkout', '-q', DEFAULT_BRANCH)
}
