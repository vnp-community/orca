/**
 * What the agent left in a task worktree of the user's repo after a real execution, read from disk, not from the service: the
 * Node module and its test under scripts/e2e-sandbox/, nothing else changed, and `node --test` passing there.
 */
import { spawnSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect } from '@playwright/test'
import { SANDBOX_DIR, type DeepseekFixture } from './deepseek-repo-fixture'
import type { LiveFlow } from './live-flow-harness'

function sh(cwd: string, file: string, args: string[], timeout = 90_000): string {
  const r = spawnSync(file, args, {
    cwd,
    encoding: 'utf8',
    timeout,
    // Why: a bare environment, as for the throwaway fixture: the check must not depend on the tester's shell.
    env: { PATH: process.env.PATH ?? '', HOME: process.env.HOME ?? '' }
  })
  if (r.error) {
    throw r.error
  }
  return `${r.stdout}${r.stderr}`
}

export type SandboxChange = { module: string; test: string; fn: string }

/** Files under the sandbox dir only; returns the worktree that holds the change. */
export async function expectSandboxChange(
  f: LiveFlow,
  fx: DeepseekFixture,
  want: SandboxChange
): Promise<string> {
  let place = ''
  await f.step(
    'the task worktree of the repo holds the expected change and its tests pass',
    async () => {
      const trees = fx.taskWorktrees()
      const branches = fx.branches()
      console.log(
        `[deepseek] task worktrees: ${trees.join(', ') || 'none'}; branches now: ${branches.length}`
      )
      expect(
        trees.length,
        'the agent worked in a task worktree, not in the main checkout'
      ).toBeGreaterThan(0)
      expect(
        branches.some((b) => b.startsWith('task/')),
        'a task branch exists'
      ).toBe(true)
      const found = trees.find((t) => existsSync(join(t, SANDBOX_DIR, want.module)))
      expect(found, `a task worktree has ${SANDBOX_DIR}/${want.module}`).toBeTruthy()
      place = found!
      const code = readFileSync(join(place, SANDBOX_DIR, want.module), 'utf8')
      const test = readFileSync(join(place, SANDBOX_DIR, want.test), 'utf8')
      expect(code, `${want.module} exports ${want.fn}()`).toMatch(
        new RegExp(
          `export\\s+(default\\s+)?(async\\s+)?(function|const)\\s+${want.fn}\\b|export\\s*\\{[^}]*\\b${want.fn}\\b`
        )
      )
      expect(test, `${want.test} tests ${want.fn}()`).toMatch(new RegExp(`\\b${want.fn}\\s*\\(`))
      const base = sh(place, 'git', ['merge-base', 'HEAD', fx.headSha()]).trim()
      const changed = [
        ...sh(place, 'git', ['status', '--porcelain', '--untracked-files=all'])
          .split('\n')
          .filter(Boolean)
          .map((l) => l.slice(3)),
        ...sh(place, 'git', ['diff', '--name-only', `${base}..HEAD`])
          .split('\n')
          .filter(Boolean)
      ]
      expect(
        changed.filter((p) => !p.startsWith(`${SANDBOX_DIR}/`)),
        `only ${SANDBOX_DIR}/ changed in the task worktree`
      ).toEqual([])
      // Why: a directory argument is no longer expanded by `node --test` (Node 25); the test file is named.
      const run = spawnSync(process.execPath, ['--test', `${SANDBOX_DIR}/${want.test}`], {
        cwd: place,
        encoding: 'utf8',
        timeout: 90_000,
        env: { PATH: process.env.PATH ?? '', HOME: process.env.HOME ?? '' }
      })
      const out = `${run.stdout}${run.stderr}`
      expect(run.status, `node --test passes in the task worktree:\n${out.slice(-600)}`).toBe(0)
      expect(out, 'node --test ran at least one test').toMatch(/(?:ℹ|#) pass [1-9]\d*/)
      f.lags.push(`verified change in ${place.split('/').at(-1)}`)
    }
  )
  return place
}
