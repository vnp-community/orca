import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import * as fs from 'node:fs/promises'
import * as path from 'node:path'
import * as os from 'node:os'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { resolveCompareRange } from './codeintel-merge-base-resolution'
import { CodeIntelError } from './codeintel-errors'

const execFileAsync = promisify(execFile)

async function git(args: string[], cwd: string): Promise<string> {
  const { stdout } = await execFileAsync('git', args, { cwd })
  return stdout.trim()
}

describe('codeintel-merge-base-resolution', () => {
  let tmpDir: string

  beforeEach(async () => {
    tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'codeintel-mb-test-'))
  })

  afterEach(async () => {
    await fs.rm(tmpDir, { recursive: true, force: true })
  })

  it('resolves unborn repo when no commit exists', async () => {
    await git(['init'], tmpDir)
    // unborn repo has no base candidates either -> expect base_required
    await expect(resolveCompareRange({}, tmpDir)).rejects.toMatchObject({
      code: 'CODEINTEL_INVALID_PARAMS',
      data: { reason: 'base_required' }
    })
  })

  it('rejects invalid ref formats with syntax error', async () => {
    await git(['init'], tmpDir)
    await expect(resolveCompareRange({ base: '-x' }, tmpDir)).rejects.toThrow(CodeIntelError)
    await expect(resolveCompareRange({ base: 'a b' }, tmpDir)).rejects.toThrow(CodeIntelError)
    await expect(resolveCompareRange({ head: '-h' }, tmpDir)).rejects.toThrow(CodeIntelError)
  })

  it('resolves merge-base between main and feature branch', async () => {
    await git(['init', '-b', 'main'], tmpDir)
    await git(['config', 'user.name', 'test'], tmpDir)
    await git(['config', 'user.email', 'test@test.com'], tmpDir)

    await fs.writeFile(path.join(tmpDir, 'file.txt'), 'hello')
    await git(['add', 'file.txt'], tmpDir)
    await git(['commit', '-m', 'c1'], tmpDir)
    const c1 = await git(['rev-parse', 'HEAD'], tmpDir)

    // Create branch feature
    await git(['checkout', '-b', 'feature'], tmpDir)
    await fs.writeFile(path.join(tmpDir, 'file2.txt'), 'feature change')
    await git(['add', 'file2.txt'], tmpDir)
    await git(['commit', '-m', 'c2'], tmpDir)
    const c2 = await git(['rev-parse', 'HEAD'], tmpDir)

    // Compare with default base 'main'
    const res = await resolveCompareRange({}, tmpDir)
    expect(res.baseRef).toBe('main')
    expect(res.baseOid).toBe(c1)
    expect(res.headOid).toBe(c2)
    expect(res.mergeBase).toBe(c1)
    expect(res.unborn).toBe(false)
  })

  it('throws unresolved_ref when base does not exist', async () => {
    await git(['init', '-b', 'main'], tmpDir)
    await git(['config', 'user.name', 'test'], tmpDir)
    await git(['config', 'user.email', 'test@test.com'], tmpDir)
    await fs.writeFile(path.join(tmpDir, 'file.txt'), 'hello')
    await git(['add', 'file.txt'], tmpDir)
    await git(['commit', '-m', 'c1'], tmpDir)

    await expect(resolveCompareRange({ base: 'nonexistent' }, tmpDir)).rejects.toMatchObject({
      code: 'CODEINTEL_INVALID_PARAMS',
      data: { reason: 'unresolved_ref', ref: 'nonexistent' }
    })
  })

  it('throws no_merge_base when two orphan branches have no common ancestor', async () => {
    await git(['init', '-b', 'main'], tmpDir)
    await git(['config', 'user.name', 'test'], tmpDir)
    await git(['config', 'user.email', 'test@test.com'], tmpDir)
    await fs.writeFile(path.join(tmpDir, 'file.txt'), 'hello')
    await git(['add', 'file.txt'], tmpDir)
    await git(['commit', '-m', 'c1'], tmpDir)

    // Orphan branch
    await git(['checkout', '--orphan', 'orphan-branch'], tmpDir)
    await fs.writeFile(path.join(tmpDir, 'orphan.txt'), 'orphan')
    await git(['add', 'orphan.txt'], tmpDir)
    await git(['commit', '-m', 'orphan commit'], tmpDir)

    await expect(resolveCompareRange({ base: 'main' }, tmpDir)).rejects.toMatchObject({
      code: 'CODEINTEL_INVALID_PARAMS',
      data: { reason: 'no_merge_base' }
    })
  })

  it('handles unborn HEAD when base is specified', async () => {
    await git(['init', '-b', 'main'], tmpDir)
    await git(['config', 'user.name', 'test'], tmpDir)
    await git(['config', 'user.email', 'test@test.com'], tmpDir)
    await fs.writeFile(path.join(tmpDir, 'file.txt'), 'hello')
    await git(['add', 'file.txt'], tmpDir)
    await git(['commit', '-m', 'c1'], tmpDir)
    const c1 = await git(['rev-parse', 'HEAD'], tmpDir)

    // Create an unborn HEAD via symbolic-ref to an unborn branch
    await git(['symbolic-ref', 'HEAD', 'refs/heads/unborn-branch'], tmpDir)

    const res = await resolveCompareRange({ base: 'main' }, tmpDir)
    expect(res.unborn).toBe(true)
    expect(res.baseRef).toBe('main')
    expect(res.baseOid).toBe(c1)
    expect(res.headOid).toBeNull()
    expect(res.mergeBase).toBeNull()
  })

  it('prefers refs/remotes/origin/HEAD when available', async () => {
    await git(['init', '-b', 'local-branch'], tmpDir)
    await git(['config', 'user.name', 'test'], tmpDir)
    await git(['config', 'user.email', 'test@test.com'], tmpDir)
    await fs.writeFile(path.join(tmpDir, 'file.txt'), 'hello')
    await git(['add', 'file.txt'], tmpDir)
    await git(['commit', '-m', 'c1'], tmpDir)
    const c1 = await git(['rev-parse', 'HEAD'], tmpDir)

    // Fake a remote tracking ref
    await git(['update-ref', 'refs/remotes/origin/custom-main', c1], tmpDir)
    await git(['symbolic-ref', 'refs/remotes/origin/HEAD', 'refs/remotes/origin/custom-main'], tmpDir)

    const res = await resolveCompareRange({}, tmpDir)
    expect(res.baseRef).toBe('origin/custom-main')
    expect(res.baseOid).toBe(c1)
  })
})
