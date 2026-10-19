/**
 * What the agent left in the fixture repo after a real execution: the new function and its test in a task worktree, only
 * under src/ and tests/, and the fixture's own tests passing. Read from disk, not from the service.
 */
import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import type { RealFixture } from './real-repo-fixture'

/** Runs a command in `cwd` with a bare environment; stdout and stderr together (unittest reports on stderr). */
function sh(cwd: string, file: string, args: string[], timeout = 60_000): string {
  const r = spawnSync(file, args, {
    cwd,
    encoding: 'utf8',
    timeout,
    env: { PATH: process.env.PATH ?? '', HOME: process.env.HOME ?? '' }
  })
  if (r.error) {
    throw r.error
  }
  return `${r.stdout}${r.stderr}`
}

/** What the agent left in the fixture: the function `fn` and its test in a task worktree, only under src/ and tests/, tests pass. */
export async function expectExpectedChangeInFixture(
  f: LiveFlow,
  fx: RealFixture,
  fn: string
): Promise<void> {
  await f.step('the fixture repo holds the expected change and its tests pass', async () => {
    const trees = fx.worktrees().filter((p) => p !== fx.repoPath)
    const branches = fx.branches().filter((b) => b !== 'main')
    console.log(
      `[fixture] worktrees: ${trees.join(', ') || 'none'}; branches: ${branches.join(', ') || 'none'}`
    )
    // Why: git-gateway makes one worktree per Plan next to the repo; a Plan with a merge step could also land on main.
    const places = trees.length > 0 ? trees : [fx.repoPath]
    expect(
      trees.length,
      'the agent worked in a task worktree, not in the main checkout'
    ).toBeGreaterThan(0)
    expect(
      branches.some((b) => b.startsWith('task/')),
      `a task branch exists (${branches.join(',')})`
    ).toBe(true)
    let verified = ''
    for (const place of places) {
      const calc = readFileSync(join(place, 'src/calc.py'), 'utf8')
      const test = readFileSync(join(place, 'tests/test_calc.py'), 'utf8')
      if (
        new RegExp(`^def\\s+${fn}\\s*\\(`, 'm').test(calc) &&
        new RegExp(`\\b${fn}\\s*\\(`).test(test)
      ) {
        const changed = sh(place, 'git', ['status', '--porcelain']).split('\n').filter(Boolean)
        const committed = sh(place, 'git', ['diff', '--name-only', `${fx.headSha()}..HEAD`])
          .split('\n')
          .filter(Boolean)
        const files = [...changed.map((l) => l.slice(3)), ...committed]
        const outside = files.filter((p) => !/^(src|tests)\//.test(p))
        expect(outside, 'only src/ and tests/ changed').toEqual([])
        const out = sh(place, 'python3', ['-m', 'unittest'])
        expect(out, 'the fixture tests pass in the worktree').toMatch(/^OK\s*$/m)
        verified = `${place} (${files.join(', ') || 'no file list'})`
        break
      }
    }
    expect(verified, `a worktree has ${fn}() and a test for it`).not.toBe('')
    f.lags.push(`verified change in ${verified.split('/').at(-1)}`)
  })
}

export type WorktreeChangeState = { uncommitted: string[]; commitsAhead: number }

/** Whether the agent left its work uncommitted (Source Control "Changes") or committed it (Source Control "Commits"). */
export function worktreeChangeState(fx: RealFixture): WorktreeChangeState {
  const place = fx.worktrees().find((p) => p !== fx.repoPath)
  if (!place) {
    return { uncommitted: [], commitsAhead: 0 }
  }
  const uncommitted = sh(place, 'git', ['status', '--porcelain']).split('\n').filter(Boolean)
  const ahead = sh(place, 'git', ['rev-list', '--count', `${fx.headSha()}..HEAD`]).trim()
  return { uncommitted: uncommitted.map((l) => l.slice(3)), commitsAhead: Number(ahead) || 0 }
}
