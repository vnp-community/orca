/**
 * The two throwaway repos of the "Add Project -> own project -> Request" spec (TASK-FE-E2E-027): alpha (a decoy that the Request
 * must NOT read) and bravo (the repo the Request is about). Each is a tiny git repo with a recognisable README and two functions;
 * both carry the tool guard of the real-AI fixtures so a stray read outside the repo is refused and every call is logged
 * (the log in a repo proves claude ran THERE). Bravo is exposed as a RealFixture, so the safety net of real-flow-harness applies.
 * Never a repo of the user. The names are the folder names, because Add Project names the project after the folder.
 */
import {
  cpSync,
  existsSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  realpathSync,
  rmSync,
  writeFileSync
} from 'node:fs'
import { join, resolve } from 'node:path'
import { browsableTempBase } from './live-add-project-repro-steps'
import {
  fixtureGit,
  guardLogLines,
  runToolGuardSelfTest,
  type RealFixture
} from './real-repo-fixture'

const GUARD_TEMPLATE = resolve(__dirname, 'fixture-repo/template/.claude')
export const ADDPROJ_PREFIX = 'e2e-ui-addproj-'

/** What each repo is made of: the identifiers the spec looks for in the answer (bravo) and must not find there (alpha). */
export const REPO_FACTS = {
  alpha: {
    functions: ['alpha_mul', 'alpha_div'],
    module: 'alpha_math',
    readme:
      'Alpha math: multiplies and divides integers. Decoy repo of an automated add-project test.'
  },
  bravo: {
    functions: ['bravo_add', 'bravo_sub'],
    module: 'bravo_math',
    readme:
      'Bravo math: adds and subtracts integers. The repo a read-only Request is asked about in an automated add-project test.'
  }
} as const

export type SandboxRepo = {
  /** Folder name = the project name Add Project proposes. */
  name: string
  path: string
  /** Runs the repo's hook as claude would and throws if it would fail open; removes the audit logs it wrote. */
  selfTestGuard: () => void
  repoStatus: () => string
  headSha: () => string
  branches: () => string[]
  worktrees: () => string[]
  guardCalls: () => string[]
  guardDenials: () => string[]
}

function writeSandboxFiles(dir: string, kind: keyof typeof REPO_FACTS): void {
  const facts = REPO_FACTS[kind]
  const [first, second] = facts.functions
  mkdirSync(join(dir, 'src'))
  mkdirSync(join(dir, 'tests'))
  writeFileSync(join(dir, 'README.md'), `# ${kind}-sandbox\n\n${facts.readme}\n`)
  writeFileSync(
    join(dir, 'CLAUDE.md'),
    `# ${kind} sandbox\n\nTiny throwaway repo used by an automated test. Work only inside this directory: do not read or write anything\noutside it, do not install packages, do not use the network, and do not push or change git remotes.\n`
  )
  // Why: the guard's audit logs live in the allowed root (this directory); ignored, so "working tree clean" stays checkable.
  writeFileSync(join(dir, '.gitignore'), 'tool-guard-*.log\n__pycache__/\n')
  const op = (name: string, sign: string): string =>
    `def ${name}(a: int, b: int) -> int:\n    """${kind} ${name.split('_')[1]}: returns a ${sign} b."""\n    return a ${sign} b\n`
  const [signFirst, signSecond] = kind === 'bravo' ? ['+', '-'] : ['*', '//']
  writeFileSync(
    join(dir, 'src', `${facts.module}.py`),
    `${op(first, signFirst)}\n\n${op(second, signSecond)}`
  )
  writeFileSync(
    join(dir, 'tests', `test_${facts.module}.py`),
    `import unittest\n\nfrom src.${facts.module} import ${first}, ${second}\n\n\nclass ${kind[0].toUpperCase()}${kind.slice(1)}MathTest(unittest.TestCase):\n    def test_first(self):\n        self.assertEqual(${first}(6, 3), ${kind === 'bravo' ? 9 : 18})\n\n    def test_second(self):\n        self.assertEqual(${second}(6, 3), ${kind === 'bravo' ? 3 : 2})\n`
  )
  writeFileSync(join(dir, 'src', '__init__.py'), '')
  writeFileSync(join(dir, 'tests', '__init__.py'), '')
}

function makeSandboxRepo(base: string, stamp: string, kind: keyof typeof REPO_FACTS): SandboxRepo {
  const name = `${ADDPROJ_PREFIX}${stamp}-${kind}`
  const dir = join(base, name)
  mkdirSync(dir)
  const path = realpathSync(dir)
  writeSandboxFiles(path, kind)
  cpSync(GUARD_TEMPLATE, join(path, '.claude'), { recursive: true })
  // Why: the agent hands claude a PATH without /opt/homebrew/bin, so a bare `node` would fail open (hook errors do not block).
  const settings = join(path, '.claude', 'settings.json')
  writeFileSync(settings, readFileSync(settings, 'utf8').replace('__NODE__', process.execPath))
  writeFileSync(
    join(path, '.claude', 'tool-guard.config.json'),
    `${JSON.stringify({ allowedRoot: path })}\n`
  )
  fixtureGit(path, ['init', '-q', '-b', 'main'])
  fixtureGit(path, ['add', '-A'])
  fixtureGit(path, ['commit', '-q', '-m', `init ${kind}-sandbox`])
  return {
    name,
    selfTestGuard: () => runToolGuardSelfTest(path, settings, path),
    path,
    repoStatus: () => fixtureGit(path, ['status', '--porcelain']).trim(),
    headSha: () => fixtureGit(path, ['rev-parse', 'HEAD']).trim(),
    branches: () =>
      fixtureGit(path, ['for-each-ref', '--format=%(refname:short)', 'refs/heads'])
        .split('\n')
        .filter(Boolean),
    worktrees: () =>
      fixtureGit(path, ['worktree', 'list', '--porcelain'])
        .split('\n')
        .filter((l) => l.startsWith('worktree '))
        .map((l) => l.slice('worktree '.length)),
    guardCalls: () => guardLogLines(join(path, 'tool-guard-calls.log')),
    guardDenials: () => guardLogLines(join(path, 'tool-guard-denials.log'))
  }
}

export type AddProjectFixture = RealFixture & {
  alpha: SandboxRepo
  bravo: SandboxRepo
  /** Entries of the base directory that still carry this run's prefix; empty after dispose. */
  leftovers: () => string[]
}

/** `prefix` is the run prefix `e2e-ui-<MMDDhhmmss>`; the repos get `e2e-ui-addproj-<MMDDhhmmss>-alpha|bravo`. */
export function createAddProjectFixture(prefix: string): AddProjectFixture {
  const base = browsableTempBase()
  const stamp = prefix.replace(/^e2e-ui-/, '')
  const mine = `${ADDPROJ_PREFIX}${stamp}-`
  if (existsSync(base) && readdirSync(base).some((n) => n.startsWith(mine))) {
    throw new Error(`${base} already has entries named ${mine}*: pick another ORCA_E2E_RUN_ID`)
  }
  const alpha = makeSandboxRepo(base, stamp, 'alpha')
  let bravo: SandboxRepo
  try {
    bravo = makeSandboxRepo(base, stamp, 'bravo')
  } catch (e) {
    rmSync(alpha.path, { recursive: true, force: true })
    throw e
  }
  const leftovers = (): string[] => readdirSync(base).filter((n) => n.startsWith(mine))
  let disposed = false
  return {
    base: bravo.path,
    repoPath: bravo.path,
    alpha,
    bravo,
    selfTestGuard: () => {
      alpha.selfTestGuard()
      bravo.selfTestGuard()
    },
    guardCalls: bravo.guardCalls,
    guardDenials: () => [...bravo.guardDenials(), ...alpha.guardDenials()],
    repoStatus: bravo.repoStatus,
    headSha: bravo.headSha,
    branches: bravo.branches,
    worktrees: bravo.worktrees,
    leftovers,
    dispose: () => {
      if (disposed) {
        return
      }
      disposed = true
      // Why: also sweeps `<repo>-<branch>` siblings, which exist only if the product created a worktree for these repos.
      for (const name of leftovers()) {
        rmSync(join(base, name), { recursive: true, force: true })
      }
    }
  }
}
