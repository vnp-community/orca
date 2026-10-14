/**
 * isUsableGitRepo decides which project repos the user-data guard fingerprints: a folder git accepts, not a leftover `.git`.
 *   node --test tests/stages/e2e/orca-flows/ui-live/live-usable-git-repo.test.ts
 */
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { mkdirSync, mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { test } from 'node:test'
import { isUsableGitRepo } from './live-usable-git-repo.ts'

test('a real repo is usable; a folder with an empty .git directory and a missing folder are not', () => {
  const root = mkdtempSync(join(tmpdir(), 'orca-usable-'))
  try {
    const real = join(root, 'real')
    mkdirSync(real)
    execFileSync('git', ['-C', real, 'init', '-q'])
    const broken = join(root, 'broken')
    mkdirSync(join(broken, '.git'), { recursive: true })
    assert.equal(isUsableGitRepo(real), true)
    assert.equal(isUsableGitRepo(broken), false)
    assert.equal(isUsableGitRepo(join(root, 'missing')), false)
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})
