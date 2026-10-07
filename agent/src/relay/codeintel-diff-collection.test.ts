import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import * as fs from 'node:fs/promises'
import * as path from 'node:path'
import * as os from 'node:os'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { collectDiff } from './codeintel-diff-collection'
import { resolveCompareRange } from './codeintel-merge-base-resolution'

const execFileAsync = promisify(execFile)

async function git(args: string[], cwd: string): Promise<string> {
  const { stdout } = await execFileAsync('git', args, { cwd })
  return stdout.trim()
}

describe('codeintel-diff-collection', () => {
  let tmpDir: string

  beforeEach(async () => {
    tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'diff-collection-test-'))
  })

  afterEach(async () => {
    await fs.rm(tmpDir, { recursive: true, force: true })
  })

  it('collects modified, added, deleted, renamed, and untracked changes against working tree', async () => {
    await git(['init', '-b', 'main'], tmpDir)
    await git(['config', 'user.name', 'test'], tmpDir)
    await git(['config', 'user.email', 'test@test.com'], tmpDir)

    await fs.writeFile(path.join(tmpDir, 'existing.txt'), 'line 1\nline 2\n')
    await fs.writeFile(path.join(tmpDir, 'to-delete.txt'), 'delete me\n')
    await fs.writeFile(path.join(tmpDir, 'to-rename.txt'), 'rename me\n')
    await git(['add', '.'], tmpDir)
    await git(['commit', '-m', 'initial'], tmpDir)

    // Make changes:
    // 1. Modify existing.txt
    await fs.appendFile(path.join(tmpDir, 'existing.txt'), 'line 3\n')
    // 2. Delete to-delete.txt
    await fs.rm(path.join(tmpDir, 'to-delete.txt'))
    // 3. Rename to-rename.txt to renamed.txt
    await git(['mv', 'to-rename.txt', 'renamed.txt'], tmpDir)
    // 4. Untracked file
    await fs.writeFile(path.join(tmpDir, 'untracked.txt'), 'untracked line\n')

    const range = await resolveCompareRange({}, tmpDir)
    const result = await collectDiff(range, tmpDir)

    expect(result.warnings).toHaveLength(0)
    expect(result.dirtyFiles.size).toBeGreaterThan(0)

    const paths = result.changedFiles.map(f => f.path)
    expect(paths).toContain('existing.txt')
    expect(paths).toContain('to-delete.txt')
    expect(paths).toContain('renamed.txt')
    expect(paths).toContain('untracked.txt')

    const untracked = result.changedFiles.find(f => f.path === 'untracked.txt')
    expect(untracked?.status).toBe('A')
    expect(untracked?.untracked).toBe(true)
    expect(untracked?.additions).toBe(1)
    expect(untracked?.hunks).toHaveLength(1)
    expect(untracked?.hunks[0].startLine).toBe(1)

    const renamed = result.changedFiles.find(f => f.path === 'renamed.txt')
    expect(renamed?.status).toBe('R')
    expect(renamed?.oldPath).toBe('to-rename.txt')
  })

  it('does not include uncommitted/untracked changes when head ref is explicitly specified', async () => {
    await git(['init', '-b', 'main'], tmpDir)
    await git(['config', 'user.name', 'test'], tmpDir)
    await git(['config', 'user.email', 'test@test.com'], tmpDir)

    await fs.writeFile(path.join(tmpDir, 'file.txt'), 'initial\n')
    await git(['add', '.'], tmpDir)
    await git(['commit', '-m', 'c1'], tmpDir)

    await git(['checkout', '-b', 'feat'], tmpDir)
    await fs.writeFile(path.join(tmpDir, 'file.txt'), 'committed change\n')
    await git(['commit', '-am', 'c2'], tmpDir)

    // Uncommitted changes in working tree
    await fs.writeFile(path.join(tmpDir, 'dirty.txt'), 'uncommitted')

    const range = await resolveCompareRange({ base: 'main', head: 'feat' }, tmpDir)
    const result = await collectDiff(range, tmpDir)

    const paths = result.changedFiles.map(f => f.path)
    expect(paths).toContain('file.txt')
    expect(paths).not.toContain('dirty.txt')
    expect(result.dirtyFiles.size).toBe(0)
  })

  it('detects binary files properly in untracked and git diff', async () => {
    await git(['init', '-b', 'main'], tmpDir)
    await git(['config', 'user.name', 'test'], tmpDir)
    await git(['config', 'user.email', 'test@test.com'], tmpDir)

    await fs.writeFile(path.join(tmpDir, 'readme.txt'), 'init\n')
    await git(['add', '.'], tmpDir)
    await git(['commit', '-m', 'c1'], tmpDir)

    // Add binary file with null byte
    const binaryBuf = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x00, 0x0a, 0x1a, 0x0a])
    await fs.writeFile(path.join(tmpDir, 'untracked.bin'), binaryBuf)

    const range = await resolveCompareRange({}, tmpDir)
    const result = await collectDiff(range, tmpDir)

    const binFile = result.changedFiles.find(f => f.path === 'untracked.bin')
    expect(binFile?.binary).toBe(true)
    expect(binFile?.hunks).toHaveLength(0)
  })

  it('falls back to raw/numstat only when maxBuffer is exceeded during -U0', async () => {
    await git(['init', '-b', 'main'], tmpDir)
    await git(['config', 'user.name', 'test'], tmpDir)
    await git(['config', 'user.email', 'test@test.com'], tmpDir)

    await fs.writeFile(path.join(tmpDir, 'file.txt'), 'line 1\nline 2\n')
    await git(['add', '.'], tmpDir)
    await git(['commit', '-m', 'c1'], tmpDir)

    await fs.appendFile(path.join(tmpDir, 'file.txt'), 'long line repeated '.repeat(100) + '\n')

    const range = await resolveCompareRange({}, tmpDir)
    // Pass tiny maxBuffer = 10 to force buffer overflow
    const result = await collectDiff(range, tmpDir, { maxBuffer: 10 })

    expect(result.warnings).toContain('hunks_unavailable_diff_too_large')
    const file = result.changedFiles.find(f => f.path === 'file.txt')
    expect(file).toBeDefined()
    expect(file?.hunks).toHaveLength(0)
    expect(file?.additions).toBeGreaterThan(0)
  })
})
